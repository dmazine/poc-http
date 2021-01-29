/**
 * PoC using HTTP timeouts.
 */
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// TLS certificate
const (
	ServerCertFile = "cmd/server/server.crt"
	ServerKeyFile  = "cmd/server/server.key"
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
	RateLimitRate  float64 = 0
	RateLimitBurst         = 1
)

// Context timeout settings
const (
	Timeout = 0 * time.Millisecond
)

// Delay
var (
	MinimumDelay int64 = 0
	MaximumDelay int64 = 0
)

// Update delay request
type UpdateDelayRequest struct {
	// Minimum delay in milliseconds
	MinimumDelay int64 `json:"minimumDelay"`

	// Maximum delay in milliseconds
	MaximumDelay int64 `json:"maximumDelay"`
}

func (r *UpdateDelayRequest) Validate() error {
	if r.MinimumDelay < 0 {
		return errors.New("MinimumDelay can not be negative")
	}

	if r.MaximumDelay < 0 {
		return errors.New("MaximumDelay can not be negative")
	}

	if r.MinimumDelay > r.MaximumDelay {
		return errors.New("MinimumDelay can not be greater than MaximumDelay")
	}

	return nil
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
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
	handler.Use(WithTimeout(Timeout))
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

func WithTimeout(timeout time.Duration) gin.HandlerFunc {
	if timeout == 0 {
		return noTimeLimit
	}

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)

		defer func() {
			cancel()

			if ctx.Err() == context.DeadlineExceeded {
				fmt.Println("context timeout exceeded")
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
	ctx := c.Request.Context()
	delay := calculateDelay()

	select {
	case <-time.After(delay):
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
		return

	case <-ctx.Done():
		// if the context is done it timed out or was cancelled
		c.JSON(http.StatusInternalServerError, buildError(ctx.Err().Error()))
		return
	}
}

func handleGetDelay(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"MinimumDelay": MinimumDelay,
		"MaximumDelay": MaximumDelay,
	})
}

func handleUpdateDelay(c *gin.Context) {
	var request UpdateDelayRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, buildError(err.Error()))
		return
	}

	if err := request.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, buildError(err.Error()))
		return
	}

	MinimumDelay = request.MinimumDelay
	MaximumDelay = request.MaximumDelay

	c.Status(http.StatusOK)
}

func buildError(message string) *gin.H {
	return &gin.H{"error": message}
}

func calculateDelay() time.Duration {
	if MaximumDelay == MinimumDelay {
		return time.Duration(0) * time.Millisecond
	}

	delay := rand.Int63n(MaximumDelay-MinimumDelay) + MinimumDelay
	return time.Duration(delay) * time.Millisecond
}
