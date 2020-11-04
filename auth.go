package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"osu-api-proxy/osuapi"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func authFunc(db *sql.DB, osuAPI *osuapi.OsuAPI, cfg config) gin.HandlerFunc {
	errPage := func(c *gin.Context, err string) {
		c.HTML(200, "error.tmpl", gin.H{
			"Error": err,
		})
	}

	return func(c *gin.Context) {
		interval := rate.Every(10 * time.Minute)
		limiter := rate.NewLimiter(interval, 1)

		ip := getIP(c)
		ipLimiter := getVisitorWithLimiter(authVisitors, ip, limiter)
		if !ipLimiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(429))
			fmt.Println("Ip over rate limit", ip)
			apiRateLimitedIP.Inc()
			return
		}

		code := c.Query("code")
		// code := r.URL.Query()["state"]	// TODO but not high priority as afaik the worst thing that can happen is that an attacker can share their api key?
		if len(code) == 0 {
			fmt.Println("Got no code :(")
			errPage(c, fmt.Sprintf("Invalid code: %v", code))
			return
		}

		token, err := osuAPI.GetToken(code)
		expiryTime := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second) // TODO Move to getToken

		user, err := osuAPI.GetCurrentUserParsed(token.AccessToken)
		if err != nil {
			fmt.Println(err)
			errPage(c, fmt.Sprintf("Error: %v", err))
			return
		}

		if user.ID == 0 {
			fmt.Println("User has ID 0")
			errPage(c, "Error, user ID 0")
			return
		}

		exists, err := userExists(user.ID, db)
		if err != nil {
			fmt.Println(err)
			errPage(c, fmt.Sprintf("Error: %v", err))
			return
		}
		fmt.Println("Exists:", exists)

		var key string
		if exists {
			fmt.Println("User exists, updating tokens...")

			err = updateTokens(db, expiryTime, token.AccessToken, token.RefreshToken, user.ID)
			if err != nil {
				fmt.Println(err)
				errPage(c, fmt.Sprintf("Error: %v", err))
				return
			}

			key, err = userKey(user.ID, db)
			if err != nil {
				fmt.Println(err)
				errPage(c, fmt.Sprintf("Error: %v", err))
				return
			}
			tokensRefreshed.Inc()
		} else {
			fmt.Println("Generating key")
			key, err = uniqueKey(db)
			if err != nil {
				fmt.Println(err)
				errPage(c, fmt.Sprintf("Error: %v", err))
				return
			}

			stmt, err := db.Prepare("INSERT INTO api_tokens (id,api_key,expiryTime,accessToken,refreshToken) VALUES(?,?,?,?,?)")
			defer stmt.Close()
			if err != nil {
				fmt.Println(err)
				errPage(c, fmt.Sprintf("Error: %v", err))
				return
			}

			_, err = stmt.Exec(user.ID, key, expiryTime, token.AccessToken, token.RefreshToken)
			if err != nil {
				fmt.Println(err)
				errPage(c, fmt.Sprintf("Error: %v", err))
				return
			}
			usersRegistered.Inc()
		}

		c.HTML(http.StatusOK, "authorize.tmpl", gin.H{
			"Username":  user.Username,
			"Key":       key,
			"AppKeyURL": cfg.App.AppKeyURL,
		})
	}
}

func authServer(db *sql.DB, osuAPI *osuapi.OsuAPI, cfg config, wg *sync.WaitGroup) {
	router := gin.Default()
	router.LoadHTMLGlob("html/templates/*")

	router.Static("/css/", "html/css")
	router.GET("/authorize", authFunc(db, osuAPI, cfg))
	router.GET("/", mainPageFunc(osuAPI))

	router.Run(cfg.Auth.Address)
	wg.Done()
}
