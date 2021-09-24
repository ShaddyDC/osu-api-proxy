package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
	cfg, err := getConfig()
	if err != nil {
		fmt.Println("Error: ", err)
	}

	db, err := sql.Open("postgres", cfg.Database.Dsn)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS api_tokens (" +
		"id INT PRIMARY KEY," +
		"api_key CHAR(64) NOT NULL," +
		"expiryTime timestamp NOT NULL," +
		"accessToken TEXT NOT NULL," +
		"refreshToken TEXT NOT NULL," +
		"UNIQUE(api_key)" +
		")")
	if err != nil {
		panic(err)
	}

	cache := setupCache(&cfg.RedisConfig)

	metricsInit()

	uc, _ := getUserCount(db)
	usersRegistered.Set(float64(uc))

	setupVisitors()

	// Refresh tokens now and daily
	go refreshTokensRoutine(db, &cfg.APIConfig)
	go cleanupVisitorsRoutine()

	wg := new(sync.WaitGroup)
	wg.Add(3)

	go authServer(db, cfg, wg)
	go apiServer(db, cache, cfg, wg)
	go promServer(db, cfg, wg)

	wg.Wait()
}
