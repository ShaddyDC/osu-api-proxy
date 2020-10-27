package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

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

func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	ips := strings.Split(forwarded, ",")
	return ips[len(ips)-1]
}
