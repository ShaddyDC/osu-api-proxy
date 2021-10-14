package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func apiLimitIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getIP(c)
		ipLimiter := getVisitor(ipVisitors, ip)

		if !ipLimiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
			fmt.Println("Ip over rate limit", ip)
			apiRateLimitedIP.Inc()
			c.Abort()
			return
		}

		c.Next()
	}
}

func apiLimitAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		interval := rate.Every(30 * time.Second)
		defaultLimiter := rate.NewLimiter(interval, 1)

		ip := getIP(c)
		limiter := getVisitorWithLimiter(authVisitors, ip, defaultLimiter)

		if !limiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
			fmt.Println("Ip over rate limit", ip)
			apiRateLimitedIP.Inc()
			c.Abort()
			return
		}

		c.Next()
	}
}

func apiLimitKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("api-key")
		apiLimiter := getVisitor(apiVisitors, key)

		if !apiLimiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
			fmt.Println("Api key over rate limit", key)
			apiRateLimitedKey.Inc()
			c.Abort()
			return
		}

		c.Next()
	}
}

func apiLclLimit(limit rate.Limit) gin.HandlerFunc {
	limiter := rate.NewLimiter(limit, 1)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
			c.Abort()
			return
		}
		c.Next()
	}
}

func apiRmtLimit(limit rate.Limit) gin.HandlerFunc {
	limiter := rate.NewLimiter(limit, 1)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.String(http.StatusOK, "Server rate limited :(")
			c.Abort()
			return
		}
		c.Next()
	}
}

func apiNoLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Stolen from here https://www.alexedwards.net/blog/how-to-rate-limit-http-requests
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type visitors struct {
	visitors map[string]*visitor
	mu       sync.Mutex
}

var apiVisitors = &visitors{}
var ipVisitors = &visitors{}
var authVisitors = &visitors{}

func getVisitor(vs *visitors, key string) *rate.Limiter {
	return getVisitorWithLimiter(vs, key, rate.NewLimiter(2, 1))
}

func getVisitorWithLimiter(vs *visitors, key string, limiter *rate.Limiter) *rate.Limiter {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	v, exists := vs.visitors[key]
	if !exists {
		vs.visitors[key] = &visitor{limiter, time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func setupVisitors() {
	apiVisitors.visitors = make(map[string]*visitor)
	ipVisitors.visitors = make(map[string]*visitor)
	authVisitors.visitors = make(map[string]*visitor)
}

func cleanupVisitors(vs *visitors) {
	vs.mu.Lock()
	for key, v := range vs.visitors {
		if time.Since(v.lastSeen) > 3*time.Minute {
			delete(vs.visitors, key)
		}
	}
	vs.mu.Unlock()
}

func cleanupVisitorsRoutine() {
	for {
		time.Sleep(time.Minute)

		cleanupVisitors(apiVisitors)
		cleanupVisitors(ipVisitors)
		cleanupVisitors(authVisitors)
	}
}

func getIP(c *gin.Context) string {
	forwarded := c.GetHeader("X-Forwarded-For")
	ips := strings.Split(forwarded, ",")
	return ips[len(ips)-1]
}
