package postgres

import (
	"context"
	"errors"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/domain/repositories"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/database/models"
	"gorm.io/gorm"
)

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) repositories.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, e *entities.User) error {
	m := &models.User{}
	m.FromEntity(e)
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	e.ID = m.ID
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id uint) (*entities.User, error) {
	m := &models.User{}
	if err := r.db.WithContext(ctx).First(m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	m := &models.User{}
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *userRepo) Update(ctx context.Context, e *entities.User) error {
	m := &models.User{}
	m.FromEntity(e)
	return r.db.WithContext(ctx).Save(m).Error
}
