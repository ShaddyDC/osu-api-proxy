package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"osu-api-proxy/osuapi"
)

type apiHandler interface {
	ServeAPI(*osuapi.OsuAPI, string, string) (string, error)
}

type apiFunc func(*osuapi.OsuAPI, string, string) (string, error)

func (f apiFunc) ServeAPI(osuAPI *osuapi.OsuAPI, api string, token string) (string, error) {
	return f(osuAPI, api, token)
}

func handleAPIRequest(db *sql.DB, osuAPI *osuapi.OsuAPI) func(next apiFunc) http.Handler {
	return func(next apiFunc) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("api-key")
			if key == "" {
				fmt.Fprintf(w, "{error:\"Invalid API key\"}")
				return
			}

			limiter := getVisitor(key)
			if !limiter.Allow() {
				http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
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
		return http.HandlerFunc(f)
	}
}
