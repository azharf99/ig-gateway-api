package models

import (
	"time"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
)

type Post struct {
	ID           uint        `gorm:"primaryKey"`
	UserID       uint        `gorm:"not null;index"`
	Caption      string      `gorm:"type:text"`
	PostType     string      `gorm:"size:50;not null"`
	Status       string      `gorm:"size:50;not null"`
	ScheduledAt  *time.Time  `gorm:"index"`
	PublishedAt  *time.Time
	ErrorMessage string      `gorm:"type:text"`
	Media        []PostMedia `gorm:"foreignKey:PostID;constraint:OnDelete:CASCADE;"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PostMedia struct {
	ID           uint      `gorm:"primaryKey"`
	PostID       uint      `gorm:"not null;index"`
	MediaURL     string    `gorm:"type:text;not null"`
	ThumbnailURL string    `gorm:"type:text"`
	Order        int       `gorm:"not null"`
	MediaType    string    `gorm:"size:50;not null"` // "image" or "video"
	CreatedAt    time.Time
}

// ToEntity converts GORM Post to Domain Post Entity
func (p *Post) ToEntity() *entities.Post {
	mediaEntities := make([]entities.PostMedia, len(p.Media))
	for i, m := range p.Media {
		mediaEntities[i] = entities.PostMedia{
			ID:           m.ID,
			PostID:       m.PostID,
			MediaURL:     m.MediaURL,
			ThumbnailURL: m.ThumbnailURL,
			Order:        m.Order,
			MediaType:    m.MediaType,
			CreatedAt:    m.CreatedAt,
		}
	}

	return &entities.Post{
		ID:           p.ID,
		UserID:       p.UserID,
		Caption:      p.Caption,
		PostType:     entities.PostType(p.PostType),
		Status:       entities.PostStatus(p.Status),
		ScheduledAt:  p.ScheduledAt,
		PublishedAt:  p.PublishedAt,
		ErrorMessage: p.ErrorMessage,
		Media:        mediaEntities,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

// FromEntity populates GORM Post from Domain Post Entity
func (p *Post) FromEntity(e *entities.Post) {
	p.ID = e.ID
	p.UserID = e.UserID
	p.Caption = e.Caption
	p.PostType = string(e.PostType)
	p.Status = string(e.Status)
	p.ScheduledAt = e.ScheduledAt
	p.PublishedAt = e.PublishedAt
	p.ErrorMessage = e.ErrorMessage
	p.CreatedAt = e.CreatedAt
	p.UpdatedAt = e.UpdatedAt

	p.Media = make([]PostMedia, len(e.Media))
	for i, m := range e.Media {
		p.Media[i] = PostMedia{
			ID:           m.ID,
			PostID:       m.PostID,
			MediaURL:     m.MediaURL,
			ThumbnailURL: m.ThumbnailURL,
			Order:        m.Order,
			MediaType:    m.MediaType,
			CreatedAt:    m.CreatedAt,
		}
	}
}
