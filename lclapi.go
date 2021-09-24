package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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

		apiLimiter := getVisitor(apiVisitors, apiKey)
		if !apiLimiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(429))
			fmt.Println("Api key over rate limit", apiKey)
			apiRateLimitedKey.Inc()
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
		if err == nil {
			c.Set("value", val)
			c.String(http.StatusOK, val)
			apiCallSuccess.Inc()
			return
		}
		c.String(http.StatusOK, err.Error())
		apiCallFailed.Inc()
	}
}

func apiServer(db *sql.DB, cache *redis.Client, cfg config, wg *sync.WaitGroup) {
	router := gin.Default()

	// authentication and local api-wide rate limits
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.APIServer.AllowedOrigins,
		AllowHeaders: []string{"api-key"},
	}))

	router.Use(apiLimitIP())
	router.Use(apiAuth(db))

	// Remote api total aggregate rate limits
	globalRmtLimitHandler := apiRmtLimit(10)

	handlers := handlersMap()
	for _, handlerCFG := range cfg.APIServer.Endpoints {
		handler, exists := handlers[handlerCFG.Handler]
		if !exists {
			panic(fmt.Sprint("Endpoint does not exist", handlerCFG.Handler))
		}

		// Cache stuff maybe
		var cacheHandler gin.HandlerFunc
		if handlerCFG.CachePolicy == "always" {
			cacheHandler = apiCache(cache, handler)
		} else {
			cacheHandler = apiCacheNoCache()
		}

		// Local endpoint specific rate limits
		var lclLimitHandler gin.HandlerFunc
		lclLimitHandler = apiNoLimit()

		// Remote endpoint specific rate limits
		var rmtLimitHandler gin.HandlerFunc
		if handler.rmtLimit != nil {
			rmtLimitHandler = apiRmtLimit(*handler.rmtLimit)
		} else {
			rmtLimitHandler = apiNoLimit()
		}

		fmt.Println("Using endpoint", handlerCFG.Handler, handler.lclEndpoint)
		router.GET(handler.lclEndpoint, lclLimitHandler, cacheHandler, rmtLimitHandler, globalRmtLimitHandler, apiHandler(handler))
	}

	router.Run(cfg.APIServer.Address)

	wg.Done()
}
