package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/coreos/etcd/client"
)

func setupCache(cfg *etcdConfig) client.Client {
	etcdCfg := client.Config{
		Endpoints: cfg.Endpoints,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	cache, err := client.New(etcdCfg)
	if err != nil {
		panic(fmt.Sprintf("Couldn't connect to etcd: %s", string(err.Error())))
	}

	return cache
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

func apiCache(cache *client.Client, handler rmtHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := handler.name + "-" + paramsToString(c.Params)

		kapi := client.NewKeysAPI(*cache)
		resp, err := kapi.Get(context.Background(), key, nil)

		if err == nil {
			fmt.Println("Loaded from cache", key)
			apiCallCached.Inc()
			apiCallSuccess.Inc()
			c.Header("Cache-Control", "public, max-age=604800")
			c.String(http.StatusOK, resp.Node.Value)
			c.Abort()
			return
		}

		c.Next()

		valueInterface, exists := c.Get("value")
		if !exists {
			fmt.Println("No value to cache :(")
			c.Header("Cache-Control", "max-age=0")
			return
		}

		value, ok := valueInterface.(string)
		if !ok {
			fmt.Println("No value (of correct type) to cache :(")
			c.Header("Cache-Control", "max-age=0")
			return
		}

		// Note that redis is single-threaded, so this isn't a race condition probably
		// It isn't an issue if we overwrite data as it should be identical
		// The only issue is potentially duplicate work
		fmt.Println("Caching", key)
		resp, err = kapi.Set(context.Background(), key, value, nil)
		if err != nil {
			fmt.Println("Failed to cache", key, err)
			c.Header("Cache-Control", "max-age=0")
		} else {
			fmt.Println("Got database response", resp)
			c.Header("Cache-Control", "public, max-age=604800")
		}
	}
}
