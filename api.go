package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"osu-api-proxy/osuapi"
)

func handleAPIRequest(db *sql.DB, osuAPI *osuapi.OsuAPI) func(next osuapi.APIFunc) func(w http.ResponseWriter, r *http.Request) {
	return func(next osuapi.APIFunc) func(w http.ResponseWriter, r *http.Request) {
		f := func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			ipLimiter := getVisitor(ipVisitors, ip)
			if !ipLimiter.Allow() {
				http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
				fmt.Println("Ip over rate limit", ip)
				return
			}

			key := r.Header.Get("api-key")
			if key == "" {
				fmt.Fprintf(w, "{error:\"Invalid API key\"}")
				return
			}

			apiLimiter := getVisitor(apiVisitors, key)
			if !apiLimiter.Allow() {
				http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
				fmt.Println("Api key over rate limit", key)
				return
			}

			token, err := keyToToken(key, db)
			if err != nil {
				fmt.Fprintf(w, "{error:\"Couldn't get token\"}")
				return
			}

			body, err := next(osuAPI, r.URL.Path, token)
			if err != nil {
				fmt.Fprintf(w, "{error:\"Error with api call: %v\"}", err)
				return
			}
			fmt.Fprintf(w, body)
		}
		return f
	}
}

func createAPIHandler(db *sql.DB, osuAPI *osuapi.OsuAPI, endpoint *endpointConfig) (osuapi.APIFunc, error) {
	apiCall, exists := osuAPI.Handlers[endpoint.Handler]
	if !exists {
		fmt.Println(osuAPI.Handlers)
		return nil, fmt.Errorf("API handler %v not found or already defined", endpoint.Handler)
	}
	delete(osuAPI.Handlers, endpoint.Handler) // Make sure each endpoint is only used once

	var cacheLoader func(next osuapi.APIFunc) osuapi.APIFunc
	if endpoint.CachePolicy == endpoint.CachePolicy { // TODO
		cacheLoader = func(next osuapi.APIFunc) osuapi.APIFunc {
			return func(osuAPI *osuapi.OsuAPI, path string, token string) (string, error) {
				return next(osuAPI, path, token) // Just pass through for now
			}
		}
	}

	return cacheLoader(apiCall), nil
}
