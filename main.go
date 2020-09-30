package main

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"osu-api-proxy/osuapi"
)

func clientSecret() string {
	return os.Getenv("CLIENT_SECRET")
}

func clientID() string {
	return os.Getenv("CLIENT_ID")
}

func redirectURI() string {
	return os.Getenv("REDIRECT_URI")
}

func databaseDSN() string {
	return os.Getenv("DATABASE_DSN")
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
	//  Unique Key(`api_key`)
	// );
	db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/test?parseTime=true")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	id, err := strconv.Atoi(clientID())
	if err != nil {
		panic(err.Error())
	}
	osuAPI := &osuapi.OsuAPI{
		ClientID:     id,
		ClientSecret: clientSecret(),
		RedirectURI:  redirectURI()}

	// Refresh tokens now and daily
	go refreshTokensRoutine(db, osuAPI)
	go cleanupVisitors()

	http.Handle("/authorize", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))

	apiHandler := handleAPIRequest(db, osuAPI)

	http.Handle("/api/user/", apiHandler(apiFunc(func(osuAPI *osuapi.OsuAPI, api string, token string) (string, error) {
		id := strings.TrimPrefix(api, "/api/user/")
		fmt.Println("Requesting user", id)

		user, err := osuAPI.GetUser(token, id)
		if err != nil {
			return "", fmt.Errorf("{Error:\"Error with api call: %v\"}", err)
		}
		return string(user), nil
	})))

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		osuURL, _ := osuAPI.OsuRequestAuthURL()
		fmt.Fprintf(w, "Hello, %q<br> <a href=%q>go here</a>", html.EscapeString(r.URL.String()), osuURL)
	}))

	log.Fatal(http.ListenAndServe(":8125", nil))
}
