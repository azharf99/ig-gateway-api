package entities

import "time"

type User struct {
	ID                   uint
	Username             string
	Email                string
	Password             string // Hashed password
	InstagramAccountID   string // Instagram Professional Account ID
	InstagramAccessToken string // Long-lived access token (encrypted in DB)
	OAuthState           string // State parameter for CSRF verification
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

