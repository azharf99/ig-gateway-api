package repositories

import (
	"context"
	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
)

type PostRepository interface {
	Create(ctx context.Context, post *entities.Post) error
	GetByID(ctx context.Context, id uint) (*entities.Post, error)
	GetByUserID(ctx context.Context, userID uint) ([]entities.Post, error)
	GetScheduledActive(ctx context.Context) ([]entities.Post, error)
	Update(ctx context.Context, post *entities.Post) error
	Delete(ctx context.Context, id uint) error
}
