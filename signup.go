package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

func disabledSignupsHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"OsuAuthURL": "",
		"EnableAuth": false,
	})
}

func authFunc(db *sql.DB, cfg config) gin.HandlerFunc {
	if !cfg.Auth.EnableAuth {
		return disabledSignupsHandler
	}

	handleError := func(c *gin.Context, err string) {
		fmt.Println(err)
		c.HTML(200, "error.tmpl", gin.H{
			"Error": err,
		})
		c.Abort()
	}

	return func(c *gin.Context) {
		code := c.Query("code")
		// code := r.URL.Query()["state"]	// TODO but not high priority as afaik the worst thing that can happen is that an attacker can share their api key?
		if len(code) == 0 {
			handleError(c, "got no code for signup")
			return
		}

		token, err := getNewToken(&cfg.APIConfig, code)
		if err != nil {
			handleError(c, fmt.Sprintf("failed to get new token with error: %v", err))
			return
		}

		user, err := getCurrentUser(token.AccessToken)
		if err != nil {
			handleError(c, fmt.Sprintf("failed to fetch user with error: %v", err))
			return
		}

		if user.ID == 0 {
			handleError(c, "received invalid user id 0")
			return
		}

		exists, err := userExists(user.ID, db)
		if err != nil {
			fmt.Println(err)
			handleError(c, fmt.Sprintf("error checking user existence: %v", err))
			return
		}
		fmt.Printf("user %s (%d) - exists: %t\n", user.Username, user.ID, exists)

		var key string
		if exists {
			fmt.Printf("user %s (%d) - Updating tokens...\n", user.Username, user.ID)
			err = updateTokens(db, token.ExpiryTime, token.AccessToken, token.RefreshToken, user.ID)
			if err != nil {
				handleError(c, fmt.Sprintf("user %s (%d) - failed to update token: %v", user.Username, user.ID, err))
				return
			}

			key, err = userKey(user.ID, db)
			if err != nil {
				handleError(c, fmt.Sprintf("user %s (%d) - failed to retrieve api key: %v", user.Username, user.ID, err))
				return
			}
			tokensRefreshed.Inc()
		} else {
			fmt.Printf("user %s (%d) - Generating key...\n", user.Username, user.ID)
			key, err = uniqueKey(db)
			if err != nil {
				handleError(c, fmt.Sprintf("user %s (%d) - failed to generate api key: %v", user.Username, user.ID, err))
				return
			}

			stmt, err := db.Prepare("INSERT INTO api_tokens (id,api_key,expiryTime,accessToken,refreshToken) VALUES($1,$2,$3,$4,$5)")
			if err != nil {
				handleError(c, fmt.Sprintf("user %s (%d) - failed to prepare database save statement: %v", user.Username, user.ID, err))
				return
			}
			defer stmt.Close()

			_, err = stmt.Exec(user.ID, key, token.ExpiryTime, token.AccessToken, token.RefreshToken)
			if err != nil {
				handleError(c, fmt.Sprintf("user %s (%d) - failed to execute database save: %v", user.Username, user.ID, err))
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
	router.GET("/authorize", apiLimitAuth(), authFunc(db, cfg))
	router.GET("/", mainPageFunc(&cfg))

	router.Run(cfg.Auth.Address)
	wg.Done()
}
