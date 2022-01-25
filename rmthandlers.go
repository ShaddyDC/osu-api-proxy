package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type rmtHandler struct {
	name        string
	lclEndpoint string
	rmtURL      func(c *gin.Context) string
	rmtLimit    *rate.Limit
}

var (
	rmtHandlers = []rmtHandler{
		{
			name:        "osufile",
			lclEndpoint: "/api/v1/osufile/:id",
			rmtURL: func(c *gin.Context) string {
				return "/osu/" + c.Param("id")
			},
		},
		{
			name:        "userinfo",
			lclEndpoint: "/api/v1/users/:user/:mode",
			rmtURL: func(c *gin.Context) string {
				return "/api/v2/users/" + c.Param("user") + "/" + c.Param("mode")
			},
		},
		{
			name:        "scorefile",
			lclEndpoint: "/api/v1/scorefile/:mode/:score",
			rmtURL: func(c *gin.Context) string {
				return "/api/v2/scores/" + c.Param("mode") + "/" + c.Param("score") + "/download"
			},
			rmtLimit: &[]rate.Limit{rate.Every(6 * time.Second)}[0], // Hack to get a pointer
		},
		{
			name:        "beatmaps_lookup",
			lclEndpoint: "/api/v1/beatmaps/lookup",
			rmtURL: func(c *gin.Context) string {
				var params strings.Builder
				for _, param_name := range []string{"checksum", "filename", "id"} {
					param := c.Query(param_name)
					fmt.Printf("Param '%s' found '%s'\n", param_name, param)
					if param == "" {
						continue
					}
					params.WriteRune('?')
					params.WriteString(param_name)
					params.WriteRune('=')
					params.WriteString(param)
					break
				}

				return fmt.Sprint("/api/v2/beatmaps/lookup?" + params.String())
			},
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
