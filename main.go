package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	_ "github.com/go-sql-driver/mysql"

	"osu-api-backend/osuapi"
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

const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randomString(length int) (string, error) {
	bytes := make([]byte, length-1)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}

	return string(bytes), nil
}

func keyToToken(key string, db *sql.DB) (string, error) {
	rows, err := db.Query("SELECT accessToken FROM test WHERE api_key=? LIMIT 1", key)
	if err != nil {
		return "", fmt.Errorf("Error querying database. %v", err)
	}

	if !rows.Next() {
		return "", fmt.Errorf("No token found")
	}

	var token string
	if err = rows.Scan(&token); err != nil {
		return token, fmt.Errorf("Couldn't scan token %v", err)
	}
	return token, nil
}

func keyExists(key string, db *sql.DB) (bool, error) {
	var keyCount int
	err := db.QueryRow("SELECT COUNT(*) FROM test WHERE api_key = ? LIMIT 1", key).Scan(&keyCount)
	if err != nil {
		return false, fmt.Errorf("Error checking database. %v", err)
	}

	return keyCount == 1, nil
}

func userExists(id int64, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM test WHERE id = ? LIMIT 1", id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("Error checking database. %v", err)
	}

	return count == 1, nil
}

func userKey(id int64, db *sql.DB) (string, error) {
	var key string
	err := db.QueryRow("SELECT api_key FROM test WHERE id = ? LIMIT 1", id).Scan(&key)
	if err != nil {
		return key, fmt.Errorf("Error checking database. %v", err)
	}

	return key, nil
}

func uniqueKey(db *sql.DB) (string, error) {
	for {
		key, err := randomString(64)
		if err != nil {
			return "", fmt.Errorf("Error using RandomString function. %v", err)
		}

		exists, err := keyExists(key, db)
		if err != nil {
			return "", err
		}

		if !exists {
			return key, nil
		}
	}
}

func updateTokens(db *sql.DB, expiryTime time.Time, accessToken string, refreshToken string, id int64) error {
	stmt, err := db.Prepare("UPDATE test SET expiryTime=?,accessToken=?,refreshToken=? WHERE id=?")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(expiryTime, accessToken, refreshToken, id)
	if err != nil {
		return err
	}
	return nil
}

func refreshTokens(db *sql.DB, osuAPI *osuapi.OsuAPI) {
	rows, err := db.Query("SELECT id, refreshToken FROM test")
	if err != nil {
		fmt.Println("Error refreshing tokens", err)
		return
	}

	for rows.Next() {
		var (
			id            int64
			refreshTokens string
		)
		if err = rows.Scan(&id, &refreshTokens); err != nil {
			fmt.Println("Error getting old token to refresh", err)
			continue
		}

		token, err := osuAPI.RefreshToken(refreshTokens)
		if err != nil {
			fmt.Println("Error refreshing token", err)
			continue
		}

		expiryTime := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		updateTokens(db, expiryTime, token.AccessToken, token.RefreshToken, id)
		time.Sleep(time.Second)
	}
}

func refreshTokensRoutine(db *sql.DB, osuAPI *osuapi.OsuAPI) {
	refreshTokens(db, osuAPI)
	for {
		refreshTokens(db, osuAPI)
		time.Sleep(time.Hour * 23)
	}
}

type apiHandler interface {
	ServeAPI(*osuapi.OsuAPI, string, string) (string, error)
}

type apiFunc func(*osuapi.OsuAPI, string, string) (string, error)

func (f apiFunc) ServeAPI(osuAPI *osuapi.OsuAPI, api string, token string) (string, error) {
	return f(osuAPI, api, token)
}

// Stolen from here https://www.alexedwards.net/blog/how-to-rate-limit-http-requests
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var visitors = make(map[string]*visitor)
var mu sync.Mutex

func getVisitor(key string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[key]
	if !exists {
		limiter := rate.NewLimiter(10, 1)
		visitors[key] = &visitor{limiter, time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

func handleAPIRequest(db *sql.DB, osuAPI *osuapi.OsuAPI) func(next apiFunc) http.Handler {
	return func(next apiFunc) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("api-key")
			if key == "" {
				fmt.Fprintf(w, "{error:\"Invalid API key\"}")
				return
			}

			limiter := getVisitor(key)
			if !limiter.Allow() {
				http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
				return
			}

			token, err := keyToToken(key, db)
			if err != nil {
				fmt.Fprintf(w, "{error:\"Couldn't get token\"}")
				return
			}

			body, err := next(osuAPI, r.URL.Path, token)
			if err != nil {
				fmt.Fprintf(w, "{error:\"Error with api call: %v\"}", err)
				return
			}
			fmt.Fprintf(w, body)
		}
		return http.HandlerFunc(f)
	}
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
