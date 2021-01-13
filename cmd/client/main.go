package main

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type DialContext func(ctx context.Context, network, addr string) (net.Conn, error)

// HTTP client settings
const (
	HTTPClientTimeout = 0 * time.Millisecond
)

// HTTP transport settings
const (
	HTTPTransportTLSHandshakeTimeout    = 0 * time.Millisecond
	HTTPTransportDisableKeepAlives      = false
	HTTPTransportMaxIdleConns           = 0
	HTTPTransportMaxIdleConnsPerHost    = 0
	HTTPTransportMaxConnsPerHost        = 0
	HTTPTransportIdleConnTimeout        = 0 * time.Second
	HTTPTransportResponseHeaderTimeout  = 0 * time.Millisecond
	HTTPTransportExpectContinueTimeout  = 0 * time.Millisecond
	HTTPTransportMaxResponseHeaderBytes = 0
	HTTPTransportWriteBufferSize        = 0
	HTTPTransportReadBufferSize         = 0
)

// Dialer settings
const (
	DialerTimeout   = 0 * time.Millisecond
	DialerKeepAlive = 0 * time.Millisecond
)

// TLS client settings
const (
	TLSClientInsecureSkipVerify = true
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	client := newHTTPClient()

	var waitGroup sync.WaitGroup

	for requesterId := 0; requesterId < 2; requesterId++ {
		waitGroup.Add(1)

		contextLogger := log.WithFields(log.Fields{
			"RequesterId": requesterId,
		})

		go func(logger *log.Entry) {
			defer waitGroup.Done()

			for j := 0; j < 100; j++ {
				startTime := time.Now()

				err := ping(client)

				stopTime := time.Now()
				elapsedTime := stopTime.Sub(startTime)

				logger.WithFields(log.Fields{
					"Start":   startTime,
					"Stop":    stopTime,
					"Elapsed": elapsedTime,
				}).Printf("Request finished with err [%v]\n", err)

				time.Sleep(1000 * time.Millisecond)
			}
		}(contextLogger)
	}

	waitGroup.Wait()
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: newHTTPTransport(),
		Timeout:   HTTPClientTimeout,
	}
}

func newHTTPTransport() http.RoundTripper {
	httpTransport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            newDialContext(),
		TLSClientConfig:        newTLSClientConfig(),
		TLSHandshakeTimeout:    HTTPTransportTLSHandshakeTimeout,
		DisableKeepAlives:      HTTPTransportDisableKeepAlives,
		MaxIdleConns:           HTTPTransportMaxIdleConns,
		MaxIdleConnsPerHost:    HTTPTransportMaxIdleConnsPerHost,
		MaxConnsPerHost:        HTTPTransportMaxConnsPerHost,
		IdleConnTimeout:        HTTPTransportIdleConnTimeout,
		ResponseHeaderTimeout:  HTTPTransportResponseHeaderTimeout,
		ExpectContinueTimeout:  HTTPTransportExpectContinueTimeout,
		MaxResponseHeaderBytes: HTTPTransportMaxResponseHeaderBytes,
		WriteBufferSize:        HTTPTransportWriteBufferSize,
		ReadBufferSize:         HTTPTransportReadBufferSize,
	}

	//err := http2.ConfigureTransport(httpTransport)
	//if err != nil {
	//	panic(err)
	//}

	return httpTransport
}

func newDialContext() DialContext {
	return (&net.Dialer{
		Timeout:   DialerTimeout,
		KeepAlive: DialerKeepAlive,
	}).DialContext
}

func newTLSClientConfig() *tls.Config {
	cfg := &tls.Config{
		InsecureSkipVerify: TLSClientInsecureSkipVerify,
	}
	return cfg
}

func ping(client *http.Client) error {
	resp, err := client.Get("http://localhost:8080/ping")

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	return nil
}
