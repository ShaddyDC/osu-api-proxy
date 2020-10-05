package main

import (
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"osu-api-proxy/osuapi"
)

func authFunc(db *sql.DB, osuAPI *osuapi.OsuAPI) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query()["code"]
		// code := r.URL.Query()["state"]	// TODO but not high priority as afaik the worst thing that can happen is that an attacker can share their api key?
		if len(code) != 1 || len(code[0]) == 0 {
			fmt.Println("Got no code :(")
			fmt.Fprintf(w, "Invalid code %q", code)
			return
		}

		token, err := osuAPI.GetToken(code[0])
		expiryTime := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second) // TODO Move to getToken

		user, err := osuAPI.GetCurrentUserParsed(token.AccessToken)
		if err != nil {
			fmt.Println(err)
			fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
			return
		}

		if user.ID == 0 {
			fmt.Println("User has ID 0")
			fmt.Fprintf(w, "Error, user ID 0")
			return
		}

		exists, err := userExists(user.ID, db)
		if err != nil {
			fmt.Println(err)
			fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
			return
		}
		fmt.Println("Exists:", exists)

		var key string
		if exists {
			fmt.Println("User exists, updating tokens...")

			err = updateTokens(db, expiryTime, token.AccessToken, token.RefreshToken, user.ID)
			if err != nil {
				fmt.Println(err)
				fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
				return
			}

			key, err = userKey(user.ID, db)
			if err != nil {
				fmt.Println(err)
				fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
				return
			}
		} else {
			fmt.Println("Generating key")
			key, err = uniqueKey(db)
			if err != nil {
				fmt.Println(err)
				fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
				return
			}

			stmt, err := db.Prepare("INSERT INTO test (id,api_key,expiryTime,accessToken,refreshToken) VALUES(?,?,?,?,?)")
			if err != nil {
				fmt.Println(err)
				fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
				return
			}

			_, err = stmt.Exec(user.ID, key, expiryTime, token.AccessToken, token.RefreshToken)
			if err != nil {
				fmt.Println(err)
				fmt.Fprintf(w, "Error %q", html.EscapeString(err.Error()))
				return
			}
		}

		fmt.Fprintf(w, "Hello, %q <br>\n", html.EscapeString(user.Username))
		fmt.Fprintf(w, "Remember your api key %q!", key)
	}
}
func mainPageFunc(osuAPI *osuapi.OsuAPI) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		osuURL, _ := osuAPI.OsuRequestAuthURL()
		fmt.Fprintf(w, "Hello, %q<br> <a href=%q>go here</a>", html.EscapeString(r.URL.String()), osuURL)
	}
}

func authServer(db *sql.DB, osuAPI *osuapi.OsuAPI, cfg config, wg *sync.WaitGroup) {
	mux := http.NewServeMux()

	mux.HandleFunc("/authorize", authFunc(db, osuAPI))
	mux.HandleFunc("/", mainPageFunc(osuAPI))

	server := http.Server{
		Addr:    cfg.Auth.Address,
		Handler: mux,
	}

	server.ListenAndServe()
	wg.Done()
}

func apiServer(db *sql.DB, osuAPI *osuapi.OsuAPI, cfg config, wg *sync.WaitGroup) {
	mux := http.NewServeMux()

	prepareRequest := handleAPIRequest(db, osuAPI)

	for _, endpoint := range cfg.ApiServer.Endpoints {
		apiCall, err := createAPIHandler(db, osuAPI, &endpoint)
		if err != nil {
			panic(err.Error())
		}
		mux.HandleFunc(endpoint.Handler, prepareRequest(apiCall))
		fmt.Printf("Handling api %s with cache policy %s\n", endpoint.Handler, endpoint.CachePolicy)
	}

	server := http.Server{
		Addr:    cfg.ApiServer.Address,
		Handler: mux,
	}

	server.ListenAndServe()
	wg.Done()
}

func main() {
	// sudo docker run -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password -v "/home/space/tmp/osutestdb":/var/lib/mysql -it --rm mysql
	// mysql -h127.0.0.1 -uroot -ppassword

	// CREATE TABLE test (
	//  `id` INT PRIMARY KEY,
	// 	`api_key` CHAR(64) NOT NULL,
	// 	`expiryTime` DATETIME NOT NULL,
	// 	`accessToken` LONGTEXT NOT NULL,
	// 	`refreshToken` LONGTEXT NOT NULL,
	//  UNIQUE Key(`api_key`),
	//  UNIQUE INDEX(`api_key`)
	// );
	// CREATE UNIQUE INDEX `key_index` ON test (`api_key`);

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

	osuAPI := osuapi.NewOsuAPI(cfg.APIConfig)

	// Refresh tokens now and daily
	go refreshTokensRoutine(db, &osuAPI)
	go cleanupVisitors()

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go authServer(db, &osuAPI, cfg, wg)
	go apiServer(db, &osuAPI, cfg, wg)

	wg.Wait()
}
