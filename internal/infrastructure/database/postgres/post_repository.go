package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/domain/repositories"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/database/models"
	"gorm.io/gorm"
)

type postRepo struct {
	db *gorm.DB
}

func NewPostRepository(db *gorm.DB) repositories.PostRepository {
	return &postRepo{db: db}
}

func (r *postRepo) Create(ctx context.Context, e *entities.Post) error {
	m := &models.Post{}
	m.FromEntity(e)
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	// Re-map generated IDs back to entity
	createdPost := m.ToEntity()
	*e = *createdPost
	return nil
}

func (r *postRepo) GetByID(ctx context.Context, id uint) (*entities.Post, error) {
	m := &models.Post{}
	if err := r.db.WithContext(ctx).Preload("Media").First(m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *postRepo) GetByUserID(ctx context.Context, userID uint) ([]entities.Post, error) {
	var list []models.Post
	if err := r.db.WithContext(ctx).Preload("Media").Where("user_id = ?", userID).Order("created_at desc").Find(&list).Error; err != nil {
		return nil, err
	}
	res := make([]entities.Post, len(list))
	for i, m := range list {
		res[i] = *m.ToEntity()
	}
	return res, nil
}

func (r *postRepo) GetScheduledActive(ctx context.Context) ([]entities.Post, error) {
	var list []models.Post
	now := time.Now()
	if err := r.db.WithContext(ctx).Preload("Media").
		Where("status = ? AND scheduled_at <= ?", string(entities.PostStatusScheduled), now).
		Find(&list).Error; err != nil {
		return nil, err
	}
	res := make([]entities.Post, len(list))
	for i, m := range list {
		res[i] = *m.ToEntity()
	}
	return res, nil
}

func (r *postRepo) Update(ctx context.Context, e *entities.Post) error {
	m := &models.Post{}
	m.FromEntity(e)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(m).Error; err != nil {
			return err
		}

		if len(m.Media) > 0 {
			if err := tx.Where("post_id = ?", m.ID).Delete(&models.PostMedia{}).Error; err != nil {
				return err
			}
			for i := range m.Media {
				m.Media[i].PostID = m.ID
			}
			if err := tx.Create(&m.Media).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("post_id = ?", m.ID).Delete(&models.PostMedia{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Post{}, id).Error
}
