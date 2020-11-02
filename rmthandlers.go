package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type rmtHandler struct {
	name        string
	lklEndpoint string
	rmtURL      func(c *gin.Context) string
	rmtLimit    *rate.Limit
}

var (
	rmtHandlers = []rmtHandler{
		{
			name:        "osufile",
			lklEndpoint: "/api/v1/osufile/:id",
			rmtURL: func(c *gin.Context) string {
				return "/osu/" + c.Param("id")
			},
		},
		{
			name:        "userinfo",
			lklEndpoint: "/api/v1/users/:user/:mode",
			rmtURL: func(c *gin.Context) string {
				return "/api/v2/users/" + c.Param("user") + "/" + c.Param("mode")
			},
		},
		{
			name:        "scorefile",
			lklEndpoint: "/api/v1/scores/:mode/:score",
			rmtURL: func(c *gin.Context) string {
				return "/api/v2/scores/" + c.Param("mode") + "/" + c.Param("score") + "/download"
			},
			rmtLimit: &[]rate.Limit{rate.Every(6 * time.Second)}[0], // Hack to get a pointer
		},
	}
)

func handlersMap() map[string]rmtHandler {
	m := make(map[string]rmtHandler)

	for _, handler := range rmtHandlers {
		m[handler.name] = handler
	}
	return m
}
