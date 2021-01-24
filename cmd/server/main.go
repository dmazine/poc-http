/**
 * PoC using HTTP timeouts.
 */
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// TLS certificate
const (
	ServerCertFile       = "cmd/server/server.crt"
	ServerKeyFile        = "cmd/server/server.key"
)

// Server settings
const (
	ServerAddr           = ":8443"
	ServerReadTimeout    = 500 * time.Millisecond
	ServerWriteTimeout   = 1000 * time.Millisecond
	ServerIdleTimeout    = 60000 * time.Millisecond
	ServerMaxHeaderBytes = 1 << 20
)

// Rate limit settings
const (
	RateLimitRate  = 0
	RateLimitBurst = 1
)

// Time limit settings
const (
	TimeLimit = 500 * time.Millisecond
)

// Delay
var (
	Delay = 0 * time.Millisecond
)

// Update delay request
type UpdateDelayRequest struct {
	Delay int64 `json:"delay"`
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.InfoLevel)

	server := newHTTPServer()

	log.Infof("Starting server on %v\n", ServerAddr)

	err := server.ListenAndServeTLS(ServerCertFile, ServerKeyFile)
	if err != nil {
		log.Error("Server startup failed with error: ", err.Error())
	}
}

func newHTTPServer() *http.Server {
	return &http.Server{
		Addr:        ServerAddr,
		Handler:     newHandler(),
		ReadTimeout: ServerReadTimeout,
		// WriteTimeout must me > ReadTimeout + Processing Time
		// See https://blog.cloudflare.com/exposing-go-on-the-internet/
		WriteTimeout:   ServerWriteTimeout,
		IdleTimeout:    ServerIdleTimeout,
		MaxHeaderBytes: ServerMaxHeaderBytes,
	}
}

func newHandler() http.Handler {
	handler := gin.New()
	handler.Use(WithRateLimit(RateLimitRate, RateLimitBurst))
	handler.Use(WithTimeLimit(TimeLimit))
	handler.GET("/ping", handlePing)
	handler.GET("/delay", handleGetDelay)
	handler.PUT("/delay", handleUpdateDelay)
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
		return noTimeLimit
	}

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)

		defer func() {
			cancel()
			if ctx.Err() == context.DeadlineExceeded {
				log.Error("Middleware context: ", ctx.Err())
			}
		}()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

var noTimeLimit = func(c *gin.Context) {
	c.Next()
}

func handlePing(c *gin.Context) {
	time.Sleep(Delay)

	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func handleGetDelay(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"delay": Delay / time.Millisecond})
}

func handleUpdateDelay(c *gin.Context) {
	var request UpdateDelayRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	Delay = time.Duration(request.Delay) * time.Millisecond

	c.Status(http.StatusOK)
}
