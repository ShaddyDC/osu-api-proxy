package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func authFunc(db *sql.DB, cfg config) gin.HandlerFunc {
	errPage := func(c *gin.Context, err string) {
		c.HTML(200, "error.tmpl", gin.H{
			"Error": err,
		})
	}

	return func(c *gin.Context) {
		if !cfg.Auth.EnableAuth {
			c.HTML(http.StatusOK, "index.tmpl", gin.H{
				"OsuAuthURL": "",
				"EnableAuth": cfg.Auth.EnableAuth,
			})
			return
		}

		interval := rate.Every(30 * time.Second)
		limiter := rate.NewLimiter(interval, 1)

		ip := getIP(c)
		ipLimiter := getVisitorWithLimiter(authVisitors, ip, limiter)
		if !ipLimiter.Allow() {
			c.String(http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
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

		token, err := getNewToken(&cfg.APIConfig, code)
		if err != nil {
			fmt.Println("Failed to get new token :(")
			errPage(c, "Failed to get new token")
			return
		}
		expiryTime := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second) // TODO Move to getToken

		user, err := getCurrentUser(token.AccessToken)
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

			stmt, err := db.Prepare("INSERT INTO api_tokens (id,api_key,expiryTime,accessToken,refreshToken) VALUES($1,$2,$3,$4,$5)")
			if err != nil {
				fmt.Println(err)
				errPage(c, fmt.Sprintf("Error: %v", err))
				return
			}
			defer stmt.Close()

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

func mainPageFunc(cfg *config) gin.HandlerFunc {
	url, err := osuRequestAuthURL(&cfg.APIConfig)
	if err != nil {
		panic("Couldn't create auth url")
	}

	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"OsuAuthURL": url,
			"EnableAuth": cfg.Auth.EnableAuth,
		})
	}
}

func authServer(db *sql.DB, cfg config, wg *sync.WaitGroup) {
	router := gin.Default()
	router.LoadHTMLGlob("html/templates/*")

	router.Static("/css/", "html/css")
	router.GET("/authorize", authFunc(db, cfg))
	router.GET("/", mainPageFunc(&cfg))

	router.Run(cfg.Auth.Address)
	wg.Done()
}
