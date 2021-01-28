/**
 * PoC using HTTP timeouts.
 */
package main

import (
	"context"
	"fmt"
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
var (
	RateLimitRate  float64 = 0
	RateLimitBurst         = 1
)

// Time limit settings
var (
	TimeLimit = 500 * time.Millisecond
)

// Delay
var (
	Delay = 0 * time.Millisecond
)

// Update rate limit request
type UpdateRateLimitRequest struct {
	Rate  float64 `json:"rate"`
	Burst int     `json:"burst"`
}

// Update time limit request
type UpdateTimeLimitRequest struct {
	// Timeout in milliseconds
	TimeLimit int64 `json:"timeLimit"`
}

// Update delay request
type UpdateDelayRequest struct {
	// Delay in milliseconds
	Delay int64 `json:"delay"`
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
	handler.Use(WithTimeLimit(TimeLimit))
	handler.GET("/ping", handlePing)

	handler.GET("/rate-limit", handleGetRateLimit)
	handler.PUT("/rate-limit", handleUpdateRateLimit)

	handler.GET("/time-limit", handleGetTimeLimit)
	handler.PUT("/time-limit", handleUpdateTimeLimit)

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

	select {
	case <-time.After(Delay):
		c.JSON(http.StatusOK, gin.H{"message": "pong"})

	case <-ctx.Done():
		// if the context is done it timed out or was cancelled
		c.JSON(http.StatusInternalServerError, buildError(ctx.Err().Error()))
		return
	}
}

func handleGetRateLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"rate":  RateLimitRate,
		"burst": RateLimitBurst,
	})
}

func handleUpdateRateLimit(c *gin.Context) {
	var request UpdateRateLimitRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, buildError(err.Error()))
		return
	}

	RateLimitRate = request.Rate
	RateLimitBurst = request.Burst

	c.Status(http.StatusOK)
}

func handleGetTimeLimit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"timeout": TimeLimit / time.Millisecond,
	})
}

func handleUpdateTimeLimit(c *gin.Context) {
	var request UpdateTimeLimitRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, buildError(err.Error()))
		return
	}

	TimeLimit = time.Duration(request.TimeLimit) * time.Millisecond

	c.Status(http.StatusOK)
}

func handleGetDelay(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"delay": Delay / time.Millisecond})
}

func handleUpdateDelay(c *gin.Context) {
	var request UpdateDelayRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, buildError(err.Error()))
		return
	}

	Delay = time.Duration(request.Delay) * time.Millisecond

	c.Status(http.StatusOK)
}

func buildError(message string) *gin.H {
	return &gin.H{"error": message}
}
