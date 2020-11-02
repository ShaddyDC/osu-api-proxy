package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/time/rate"

	"osu-api-proxy/osuapi"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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
func mainPageFunc(osuAPI *osuapi.OsuAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		osuURL, _ := osuAPI.OsuRequestAuthURL()

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"OsuAuthURL": osuURL,
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

func promServer(db *sql.DB, cfg config, wg *sync.WaitGroup) {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	server := http.Server{
		Addr:    cfg.PromServer.Address,
		Handler: mux,
	}

	server.ListenAndServe()
	wg.Done()
}

func main() {
	// sudo docker run -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password -v "/home/space/tmp/osutestdb":/var/lib/mysql -it --rm mysql
	// mysql -h127.0.0.1 -uroot -ppassword

	// CREATE TABLE api_tokens (
	//  `id` INT PRIMARY KEY,
	// 	`api_key` CHAR(64) NOT NULL,
	// 	`expiryTime` DATETIME NOT NULL,
	// 	`accessToken` LONGTEXT NOT NULL,
	// 	`refreshToken` LONGTEXT NOT NULL,
	//  UNIQUE Key(`api_key`),
	//  UNIQUE INDEX(`api_key`)
	// );
	// CREATE UNIQUE INDEX `key_index` ON api_tokens (`api_key`);

	cfg, err := getConfig()

	if err != nil {
		fmt.Println("Error: ", err)
	}

	db, err := sql.Open("mysql", cfg.Database.Dsn+"?parseTime=true")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS api_tokens (" +
		"`id` INT PRIMARY KEY," +
		"`api_key` CHAR(64) NOT NULL," +
		"`expiryTime` DATETIME NOT NULL," +
		"`accessToken` LONGTEXT NOT NULL," +
		"`refreshToken` LONGTEXT NOT NULL," +
		"UNIQUE Key(`api_key`)," +
		"UNIQUE INDEX(`api_key`)" +
		")")
	if err != nil {
		panic(err)
	}

	osuAPI := osuapi.NewOsuAPI(cfg.APIConfig)

	metricsInit()

	uc, _ := getUserCount(db)
	usersRegistered.Set(float64(uc))

	setupVisitors()

	// Refresh tokens now and daily
	go refreshTokensRoutine(db, &osuAPI)
	go cleanupVisitorsRoutine()

	wg := new(sync.WaitGroup)
	wg.Add(3)

	go authServer(db, &osuAPI, cfg, wg)
	go apiServer(db, &osuAPI, cfg, wg)
	go promServer(db, cfg, wg)

	wg.Wait()
}
