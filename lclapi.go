package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func apiAuth(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("api-key")
		if apiKey == "" {
			c.String(http.StatusUnauthorized, "api key required")
			apiRequestsBadAuth.Inc()
			c.Abort()
			return
		}

		token, err := keyToToken(apiKey, db)
		if err != nil {
			c.String(http.StatusUnauthorized, "Couldn't get token")
			apiRequestsBadAuth.Inc()
			c.Abort()
			return
		}

		c.Set("token", token)

		c.Next()
	}
}

func apiHandler(handler rmtHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenInterface, exists := c.Get("token")
		if !exists {
			c.String(http.StatusOK, "Internal error retrieving token")
			return
		}

		token, ok := tokenInterface.(string)
		if !ok {
			c.String(http.StatusOK, "Internal error with token type????")
			return
		}

		url := "https://osu.ppy.sh" + handler.rmtURL(c)

		val, err := rmtAPIRequest(url, token)
		if err == nil { // TODO: It may make sense to use this to cache in some error cases as well
			c.Set("value", val)
			c.String(http.StatusOK, val)
			apiCallSuccess.Inc()
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		apiCallFailed.Inc()
	}
}

func apiServer(db *sql.DB, cache *clientv3.Client, cfg config, wg *sync.WaitGroup) {
	router := gin.Default()

	// authentication and local api-wide rate limits
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.APIServer.AllowedOrigins,
		AllowHeaders: []string{"api-key"},
	}))

	router.Use(apiLimitIP())

	// Remote api total aggregate rate limits
	globalRmtLimitHandler := apiRmtLimit(10)

	handlers := handlersMap()
	for _, handlerCFG := range cfg.APIServer.Endpoints {
		handler, exists := handlers[handlerCFG.Handler]
		if !exists {
			panic(fmt.Sprint("Endpoint does not exist", handlerCFG.Handler))
		}
		// Remove used handlers so we have a list of unused ones
		delete(handlers, handlerCFG.Handler)

		// Cache stuff maybe
		var cacheHandler gin.HandlerFunc
		if handlerCFG.CachePolicy == "always" {
			cacheHandler = apiCache(cache, handler)
		} else {
			cacheHandler = apiCacheNoCache()
		}

		// Local endpoint specific rate limits
		var lclLimitHandler gin.HandlerFunc = apiNoLimit()

		// Remote endpoint specific rate limits
		var rmtLimitHandler gin.HandlerFunc
		if handler.rmtLimit != nil {
			rmtLimitHandler = apiRmtLimit(*handler.rmtLimit)
		} else {
			rmtLimitHandler = apiNoLimit()
		}

		fmt.Println("Using endpoint", handlerCFG.Handler, handler.lclEndpoint)
		if cfg.APIServer.PublicCache {
			router.GET(handler.lclEndpoint, lclLimitHandler, cacheHandler, apiLimitKey(), apiAuth(db), rmtLimitHandler, globalRmtLimitHandler, apiHandler(handler))
		} else {
			router.GET(handler.lclEndpoint, lclLimitHandler, apiLimitKey(), apiAuth(db), cacheHandler, rmtLimitHandler, globalRmtLimitHandler, apiHandler(handler))
		}
		// TODO: Synchronisation to prevent duplicate work
	}

	for _, handler := range handlers {
		fmt.Println("Disabling endpoint", handler)
		router.GET(handler.lclEndpoint, func(c *gin.Context) {
			c.File("html/disabled.json")
		})
	}

	router.Run(cfg.APIServer.Address)

	wg.Done()
}
