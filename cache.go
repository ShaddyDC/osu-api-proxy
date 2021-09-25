package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/go-redis/redis/v8"
)

func setupCache(cfg *redisConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       0,
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		panic(fmt.Sprintf("Cannot ping redis %s", string(err.Error())))
	}

	return client
}

func paramsToString(params gin.Params) string {
	vals := make([]string, len(params))
	for i, param := range params {
		vals[i] = param.Value
	}

	return strings.Join(vals, "/")
}

func apiCacheNoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func apiCache(redis *redis.Client, handler rmtHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := handler.name + "-" + paramsToString(c.Params)

		value, err := redis.Get(c, key).Result()
		if err == nil {
			fmt.Println("Loaded from cache", key)
			apiCallCached.Inc()
			apiCallSuccess.Inc()
			c.String(http.StatusOK, string(value))
			c.Abort()
			return
		}

		c.Next()

		valueInterface, exists := c.Get("value")
		if !exists {
			fmt.Println("No value to cache :(")
			return
		}

		value, ok := valueInterface.(string)
		if !ok {
			fmt.Println("No value (of correct type) to cache :(")
			return
		}

		// Note that redis is single-threaded, so this isn't a race condition probably
		// It isn't an issue if we overwrite data as it should be identical
		// The only issue is potentially duplicate work
		fmt.Println("Caching", key)
		err = redis.Set(c, key, value, 0).Err()
		if err != nil {
			fmt.Println("Failed to cache", key, err)
		}
	}
}
