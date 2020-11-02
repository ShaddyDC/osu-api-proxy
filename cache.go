package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/peterbourgon/diskv/v3"
)

func paramsToString(params gin.Params) string {
	vals := make([]string, len(params))
	for i, param := range params {
		vals[i] = param.Value
	}

	return strings.Join(vals, "/")
}

func stringToPath(path string) []string {
	return strings.Split(path, "/")
	// return []string{path}
}

func advancedTransformExample(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last],
	}
}

func inverseTransformExample(pathKey *diskv.PathKey) (key string) {
	return strings.Join(pathKey.Path, "/") + "/" + pathKey.FileName
}

type cachingEntry struct {
	res   string
	ready chan struct{}
}

func apiCacheNoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func apiCache(handler rmtHandler) gin.HandlerFunc {
	d := diskv.New(diskv.Options{
		BasePath: "/cache/" + handler.name,
		// Transform:    stringToPath,
		AdvancedTransform: advancedTransformExample,
		InverseTransform:  inverseTransformExample,
		CacheSizeMax:      1024 * 1024,
	})
	m := make(map[string]*cachingEntry)
	mu := &sync.Mutex{}

	return func(c *gin.Context) {
		path := paramsToString(c.Params)

		value, err := d.Read(path)
		if err == nil {
			fmt.Println("Loaded from cache", path)
			apiCallCached.Inc()
			c.String(http.StatusOK, string(value))
			c.Abort()
			return
		}

		mu.Lock()
		exists := d.Has(path)
		if !exists {
			e := m[path]
			if e == nil { // First thread load value
				e = &cachingEntry{ready: make(chan struct{})}
				m[path] = e
				mu.Unlock()

				c.Next()

				valueInterface, exists := c.Get("value")
				if !exists {
					fmt.Println("No value to cache :(")
					close(e.ready)
					delete(m, path)
					c.Abort()
					return
				}

				value, ok := valueInterface.(string)
				if !ok {
					fmt.Println("No value (of correct type) to cache :(")
					close(e.ready)
					delete(m, path)
					c.Abort()
					return
				}

				fmt.Println("Caching", path)
				err = d.Write(path, []byte(value))
				if err != nil {
					fmt.Println("Error caching", err)
				}

				close(e.ready)

				// I don't think this is a race condition, because of reference counting and stuff, right?
				// Either they're waiting at the mutex and then the value will be cached
				// Or they will have a reference to the cachingEntry object
				// In the former case it may happen that a resource is fetched again if the prior attempt failed
				mu.Lock()
				delete(m, path)
				mu.Unlock()

				c.Abort()
				return
			}

			// Wait for first thread to finish
			mu.Unlock()
			<-e.ready

		} else { // Cached since trying to load from cache (or otherwise broken)
			mu.Unlock()
		}

		// Load value from cache
		value, err = d.Read(path)
		if err == nil {
			c.String(http.StatusOK, string(value))
			c.Abort()
			return
		}
		fmt.Println("Couldn't load value", path)
		c.String(http.StatusOK, "Couldn't get value :(")
		c.Abort()
	}
}
