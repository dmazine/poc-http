package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
)

type DialContext func(ctx context.Context, network, addr string) (net.Conn, error)

// Server settings
const (
	ServerBaseURL = "https://localhost:8443"
)

// HTTP client settings
const (
	HTTPClientTimeout = 500 * time.Millisecond
)

// HTTP transport settings
const (
	HTTPTransportTLSHandshakeTimeout    = 0 * time.Millisecond
	HTTPTransportDisableKeepAlives      = false
	HTTPTransportMaxIdleConns           = 0
	HTTPTransportMaxIdleConnsPerHost    = 1000
	HTTPTransportMaxConnsPerHost        = 0
	HTTPTransportIdleConnTimeout        = 60 * time.Second
	HTTPTransportResponseHeaderTimeout  = 0 * time.Millisecond
	HTTPTransportExpectContinueTimeout  = 0 * time.Millisecond
	HTTPTransportMaxResponseHeaderBytes = 0
	HTTPTransportWriteBufferSize        = 0
	HTTPTransportReadBufferSize         = 0
)

// HTTP2 transport settings
const (
	AllowHTTP                  = true
	StrictMaxConcurrentStreams = false
	ReadIdleTimeout            = 0 * time.Millisecond
	PingTimeout                = 0 * time.Millisecond
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

	for requesterId := 0; requesterId < 5; requesterId++ {
		waitGroup.Add(1)

		contextLogger := log.WithFields(log.Fields{
			"RequesterId": requesterId,
		})

		go func(logger *log.Entry) {
			defer waitGroup.Done()

			for requestCount := 0; requestCount < 10; requestCount++ {
				startTime := time.Now()

				//statusCode, body, err := ping(client)
				_, _, err := ping(client)

				stopTime := time.Now()
				elapsedTime := stopTime.Sub(startTime)

				if err != nil {
					logger.WithFields(log.Fields{
						"Start":   startTime,
						"Stop":    stopTime,
						"Elapsed": elapsedTime,
					}).Printf("Request failed with error [%v]\n", err)

					continue
				}

				//logger.WithFields(log.Fields{
				//	"Start":   startTime,
				//	"Stop":    stopTime,
				//	"Elapsed": elapsedTime,
				//}).Printf("Request finished with statusCode [%v] and body [%v]\n", statusCode, *body)
			}

			logger.Print("All requests executed")
		}(contextLogger)
	}

	waitGroup.Wait()
}

func newHTTPClient() *http.Client {
	return &http.Client{
		//Transport: newHTTPTransport(),
		Transport: newHTTP2Transport(),
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

func newHTTP2Transport() *http2.Transport {
	return &http2.Transport{
		TLSClientConfig:            newTLSClientConfig(),
		AllowHTTP:                  AllowHTTP,
		StrictMaxConcurrentStreams: StrictMaxConcurrentStreams,
		ReadIdleTimeout:            ReadIdleTimeout,
		PingTimeout:                PingTimeout,
	}
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

func ping(client *http.Client) (int, *string, error) {
	url := fmt.Sprintf("%s/ping", ServerBaseURL)

	resp, err := client.Get(url)
	if err != nil {
		return 0, nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	data := string(body)

	return resp.StatusCode, &data, nil
}
