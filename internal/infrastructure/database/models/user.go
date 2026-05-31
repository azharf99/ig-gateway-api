package models

import (
	"time"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
)

type User struct {
	ID                   uint   `gorm:"primaryKey"`
	Username             string `gorm:"size:255;not null"`
	Email                string `gorm:"size:255;uniqueIndex;not null"`
	Password             string `gorm:"size:255;not null"`
	InstagramAccountID   string `gorm:"size:255"`
	InstagramAccessToken string `gorm:"type:text"`
	OAuthState           string `gorm:"size:255"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ToEntity converts GORM User to Domain User Entity
func (u *User) ToEntity() *entities.User {
	return &entities.User{
		ID:                   u.ID,
		Username:             u.Username,
		Email:                u.Email,
		Password:             u.Password,
		InstagramAccountID:   u.InstagramAccountID,
		InstagramAccessToken: u.InstagramAccessToken,
		OAuthState:           u.OAuthState,
		CreatedAt:            u.CreatedAt,
		UpdatedAt:            u.UpdatedAt,
	}
}

// FromEntity populates GORM User from Domain User Entity
func (u *User) FromEntity(e *entities.User) {
	u.ID = e.ID
	u.Username = e.Username
	u.Email = e.Email
	u.Password = e.Password
	u.InstagramAccountID = e.InstagramAccountID
	u.InstagramAccessToken = e.InstagramAccessToken
	u.OAuthState = e.OAuthState
	u.CreatedAt = e.CreatedAt
	u.UpdatedAt = e.UpdatedAt
}

