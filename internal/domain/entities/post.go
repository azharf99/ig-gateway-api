package entities

import "time"

type PostType string

const (
	PostTypePhoto    PostType = "photo"
	PostTypeVideo    PostType = "video"
	PostTypeCarousel PostType = "carousel"
	PostTypeReels    PostType = "reels"
)

type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusScheduled PostStatus = "scheduled"
	PostStatusPosting   PostStatus = "posting"
	PostStatusPublished PostStatus = "published"
	PostStatusFailed    PostStatus = "failed"
)

type Post struct {
	ID           uint
	UserID       uint
	Caption      string
	PostType     PostType
	Status       PostStatus
	ScheduledAt  *time.Time
	PublishedAt  *time.Time
	ErrorMessage string
	Media        []PostMedia
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PostMedia struct {
	ID        uint
	PostID    uint
	MediaURL  string // Local path where file is stored
	Order     int
	MediaType string // "image" or "video"
	CreatedAt time.Time
}
