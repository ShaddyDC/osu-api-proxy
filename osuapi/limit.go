package osuapi

import (
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(10, 1)

func notRateLimited() bool {
	return limiter.Allow()
}
