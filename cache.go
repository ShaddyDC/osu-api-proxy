package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func setupCache(cfg *etcdConfig) *clientv3.Client {
	etcdCfg := clientv3.Config{
		Endpoints: cfg.Endpoints,
		// set timeout per request to fail fast when the target endpoint is unavailable
		DialTimeout: time.Second,
	}

	cache, err := clientv3.New(etcdCfg)
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

func apiCache(cache *clientv3.Client, handler rmtHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := handler.name + "-" + paramsToString(c.Params)

		resp, err := cache.Get(context.Background(), key)
		if err == nil && len(resp.Kvs) != 0 {
			fmt.Println("Loaded from cache", key)
			apiCallCached.Inc()
			apiCallSuccess.Inc()
			c.Header("Cache-Control", "public, max-age=604800")
			c.String(http.StatusOK, string(resp.Kvs[0].Value))
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
		_, err = cache.Put(context.Background(), key, value)
		if err != nil {
			fmt.Println("Failed to cache", key, err)
			c.Header("Cache-Control", "max-age=0")
		} else {
			fmt.Println("Cached", key)
			c.Header("Cache-Control", "public, max-age=604800")
		}
	}
}
