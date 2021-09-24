package main

import "github.com/prometheus/client_golang/prometheus"

var (
	apiRequestsBadAuth = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_requests_bad_auth",
			Help: "Number of api requests that failed on auth.",
		},
	)
	apiRateLimitedIP = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_rate_limited_ip",
			Help: "Number of api requests that were rate limited by ip.",
		},
	)
	apiRateLimitedKey = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_rate_limited_key",
			Help: "Number of api requests that were rate limited by api key.",
		},
	)
	apiCallFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_call_failed",
			Help: "Number of otherwise failed api requests.",
		},
	)
	apiCallSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_call_success",
			Help: "Number of successful api requests.",
		},
	)
	apiCallCached = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_call_cached",
			Help: "Number of cached api requests.",
		},
	)
	usersRegistered = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "users_registered",
			Help: "Number registered users.",
		},
	)
	tokensRefreshed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tokens_refreshed",
			Help: "Number of times users manually refreshed tokens.",
		},
	)
)

func metricsInit() {
	prometheus.MustRegister(apiRequestsBadAuth)
	prometheus.MustRegister(apiRateLimitedIP)
	prometheus.MustRegister(apiRateLimitedKey)
	prometheus.MustRegister(apiCallFailed)
	prometheus.MustRegister(apiCallSuccess)
	prometheus.MustRegister(apiCallCached)
	prometheus.MustRegister(usersRegistered)
	prometheus.MustRegister(tokensRefreshed)
}
