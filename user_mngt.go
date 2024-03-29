package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"
)

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
	rows, err := db.Query("SELECT accessToken FROM api_tokens WHERE api_key=$1 LIMIT 1", key)
	if err != nil {
		return "", fmt.Errorf("error querying database. %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return "", fmt.Errorf("no token found")
	}

	var token string
	if err = rows.Scan(&token); err != nil {
		return token, fmt.Errorf("couldn't scan token %v", err)
	}
	return token, nil
}

func keyExists(key string, db *sql.DB) (bool, error) {
	var keyCount int
	err := db.QueryRow("SELECT COUNT(*) FROM api_tokens WHERE api_key = $1 LIMIT 1", key).Scan(&keyCount)
	if err != nil {
		return false, fmt.Errorf("error checking database. %v", err)
	}

	return keyCount == 1, nil
}

func userExists(id int64, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_tokens WHERE id = $1 LIMIT 1", id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error checking database. %v", err)
	}

	return count == 1, nil
}

func userKey(id int64, db *sql.DB) (string, error) {
	var key string
	err := db.QueryRow("SELECT api_key FROM api_tokens WHERE id = $1 LIMIT 1", id).Scan(&key)
	if err != nil {
		return key, fmt.Errorf("error checking database. %v", err)
	}

	return key, nil
}

func uniqueKey(db *sql.DB) (string, error) {
	for {
		key, err := randomString(64)
		if err != nil {
			return "", fmt.Errorf("error using RandomString function. %v", err)
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
	stmt, err := db.Prepare("UPDATE api_tokens SET expiryTime=$1,accessToken=$2,refreshToken=$3 WHERE id=$4")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(expiryTime, accessToken, refreshToken, id)
	if err != nil {
		return err
	}
	return nil
}

func refreshTokens(db *sql.DB, cfg *osuAPIConfig) {
	fmt.Println("Refreshing tokens...")
	rows, err := db.Query("SELECT id, refreshToken FROM api_tokens")
	if err != nil {
		fmt.Println("Error refreshing tokens", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id            int64
			refreshTokens string
		)
		if err = rows.Scan(&id, &refreshTokens); err != nil {
			fmt.Println("Error getting old token to refresh", err)
			continue
		}

		token, err := refreshToken(cfg, refreshTokens)
		if err != nil {
			fmt.Println("Error refreshing token", err)
			continue
		}

		expiryTime := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		updateTokens(db, expiryTime, token.AccessToken, token.RefreshToken, id)
		time.Sleep(time.Second)
	}
}

func refreshTokensRoutine(db *sql.DB, cfg *osuAPIConfig) {
	for {
		go refreshTokens(db, cfg)
		time.Sleep(time.Hour * 23)
	}
}

func getUserCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_tokens").Scan(&count)
	return count, err
}
