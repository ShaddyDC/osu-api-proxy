package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"osu-api-proxy/osuapi"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/diskv/v3"
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
	if endpoint.CachePolicy == "always" {
		isolateID := func(s string) string {
			tokens := strings.Split(s, "/")
			return tokens[len(tokens)-1]
		}
		onlyFileTransform := func(s string) *diskv.PathKey { return &diskv.PathKey{FileName: isolateID(s), Path: []string{"."}} }
		identityTransform := func(pathKey *diskv.PathKey) string { return pathKey.FileName }
		d := diskv.New(diskv.Options{
			BasePath:          "/cache" + endpoint.Handler,
			AdvancedTransform: onlyFileTransform,
			InverseTransform:  identityTransform,
			CacheSizeMax:      1024 * 1024,
		})

		mut := &sync.Mutex{}
		m := make(map[string]*sync.WaitGroup)

		cacheLoader = func(next osuapi.APIFunc) osuapi.APIFunc {
			return func(osuAPI *osuapi.OsuAPI, path string, token string) (string, error) {
				id := isolateID(path)
				value, err := d.Read(id)
				if err == nil {
					fmt.Println("Loaded from cache", path)
					return string(value), nil
				}

				var wg *sync.WaitGroup
				{
					mut.Lock()
					wg, exists = m[id]
					if !exists {
						m[id] = &sync.WaitGroup{}
						wg = m[id]
						wg.Add(1)
						go func() {
							s, err := next(osuAPI, path, token) // Fetch from remote
							time.Sleep(5 * time.Second)
							if err == nil {
								fmt.Println("Writing to cache", path) //TODO don't cache when json contains error
								d.Write(id, []byte(s))
							} else {
								fmt.Println("Not caching due to error", path, err, s)
							}
							wg.Done()

							mut.Lock()
							defer mut.Unlock()
							delete(m, id)
						}()
					}
					mut.Unlock()
				}
				wg.Wait()

				value, err = d.Read(id)
				if err == nil {
					fmt.Println("Loaded from cache", path)
					return string(value), nil
				}
				fmt.Println("Couldn't load data", path)
				return "", fmt.Errorf("Couldn't load data for %v", path)
			}
		}
	} else {
		cacheLoader = func(next osuapi.APIFunc) osuapi.APIFunc {
			return func(osuAPI *osuapi.OsuAPI, path string, token string) (string, error) {
				return next(osuAPI, path, token) // Just pass through
			}
		}
	}

	return cacheLoader(apiCall), nil
}
