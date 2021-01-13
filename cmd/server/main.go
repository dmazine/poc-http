/**
 * PoC using HTTP timeouts.
 */
package main

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type DialContext func(ctx context.Context, network, addr string) (net.Conn, error)

// Server settings
const (
	ServerAddr           = ":8080"
	ServerReadTimeout    = 200 * time.Millisecond
	ServerWriteTimeout   = 200 * time.Millisecond
	ServerIdleTimeout    = 60000 * time.Millisecond
	ServerMaxHeaderBytes = 1 << 20
)

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
	TLSClientPreferServerCipherSuites = false
)

// TLS certificate settings
const (
	CertFile = "server-crt.pem"
	KeyFile  = "server-key.pem"
)

// Rate limit settings
const (
	RateLimitRate  = 0
	RateLimitBurst = 1
)

// Time limit settings
const (
	TimeLimitTimeout = 0 * time.Millisecond
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.InfoLevel)

	server := newHTTPServer()

	//go printMetrics()

	err := server.ListenAndServeTLS(CertFile, KeyFile)
	if err != nil {
		log.Error("Server startup failed with error: ", err.Error())
	}
}

func newHTTPServer() *http.Server {
	return &http.Server{
		Addr:           ServerAddr,
		Handler:        newHandler(),
		ReadTimeout:    ServerReadTimeout,
		WriteTimeout:   ServerWriteTimeout,
		IdleTimeout:    ServerIdleTimeout,
		MaxHeaderBytes: ServerMaxHeaderBytes,
	}
}

func newHandler() http.Handler {
	handler := gin.New()
	handler.Use(WithRateLimit(RateLimitRate, RateLimitBurst))
	handler.Use(WithTimeLimit(TimeLimitTimeout))
	handler.GET("/ping", func(c *gin.Context) {
		handlePing(c, newHTTPClient())
	})
	return handler
}

func WithRateLimit(r float64, b int) gin.HandlerFunc {
	if r == 0 {
		return WithoutRateLimit()
	}

	limiter := rate.NewLimiter(rate.Limit(r), b)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			log.Warn("RateLimit - To too many requests!")
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		c.Next()
	}
}

func WithoutRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func WithTimeLimit(timeout time.Duration) gin.HandlerFunc {
	if timeout == 0 {
		return WithoutTimeLimit()
	}

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func WithoutTimeLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
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
		PreferServerCipherSuites: TLSClientPreferServerCipherSuites,
	}
	return cfg
}

func handlePing(c *gin.Context, client *http.Client) {
	ctx := c.Request.Context()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8989/ping2", nil)
	if err != nil {
		log.Error("handlePing - Error creating the request: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("handlePing - Error executing the request: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Error("Error closing the response body: ", err.Error())
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("handlePing - Error reading response data: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": string(body)})
	log.Debug("handlePing - Response sent")
}

func printMetrics() {
	mem := &runtime.MemStats{}

	for {
		cpu := runtime.NumCPU()
		log.Info("CPU:", cpu)

		rot := runtime.NumGoroutine()
		log.Info("Goroutine:", rot)

		// Byte
		runtime.ReadMemStats(mem)
		log.Info("Memory:", mem.Alloc)

		time.Sleep(2 * time.Second)
		log.Info("-------")
	}
}
