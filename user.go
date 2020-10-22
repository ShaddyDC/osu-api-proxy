package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"osu-api-proxy/osuapi"
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
	rows, err := db.Query("SELECT accessToken FROM api_tokens WHERE api_key=? LIMIT 1", key)
	defer rows.Close()
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
	err := db.QueryRow("SELECT COUNT(*) FROM api_tokens WHERE api_key = ? LIMIT 1", key).Scan(&keyCount)
	if err != nil {
		return false, fmt.Errorf("Error checking database. %v", err)
	}

	return keyCount == 1, nil
}

func userExists(id int64, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_tokens WHERE id = ? LIMIT 1", id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("Error checking database. %v", err)
	}

	return count == 1, nil
}

func userKey(id int64, db *sql.DB) (string, error) {
	var key string
	err := db.QueryRow("SELECT api_key FROM api_tokens WHERE id = ? LIMIT 1", id).Scan(&key)
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
	stmt, err := db.Prepare("UPDATE api_tokens SET expiryTime=?,accessToken=?,refreshToken=? WHERE id=?")
	defer stmt.Close()
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
	fmt.Println("Refreshing tokens...")
	rows, err := db.Query("SELECT id, refreshToken FROM api_tokens")
	defer rows.Close()
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
	for {
		go refreshTokens(db, osuAPI)
		time.Sleep(time.Hour * 23)
	}
}

func getUserCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_tokens").Scan(&count)
	return count, err
}
