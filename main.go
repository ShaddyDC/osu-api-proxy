package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"osu-api-proxy/osuapi"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func mainPageFunc(osuAPI *osuapi.OsuAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		osuURL, _ := osuAPI.OsuRequestAuthURL()

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"OsuAuthURL": osuURL,
		})
	}
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
