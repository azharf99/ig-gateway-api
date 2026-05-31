package postgres

import (
	"context"
	"errors"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/domain/repositories"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/database/models"
	"github.com/azharf99/ig-gateway-api/pkg/crypto"
	"gorm.io/gorm"
)

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) repositories.UserRepository {
	return &userRepo{db: db}
}

func encryptToken(token string) string {
	if token == "" {
		return ""
	}
	enc, err := crypto.Encrypt(token, config.AppConfig.EncryptionKey)
	if err != nil {
		return ""
	}
	return enc
}

func decryptToken(encToken string) string {
	if encToken == "" {
		return ""
	}
	dec, err := crypto.Decrypt(encToken, config.AppConfig.EncryptionKey)
	if err != nil {
		// Fallback for existing plaintext tokens in database
		return encToken
	}
	return dec
}

func (r *userRepo) Create(ctx context.Context, e *entities.User) error {
	m := &models.User{}
	m.FromEntity(e)
	m.InstagramAccessToken = encryptToken(m.InstagramAccessToken)

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
	e := m.ToEntity()
	e.InstagramAccessToken = decryptToken(e.InstagramAccessToken)
	return e, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	m := &models.User{}
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	e := m.ToEntity()
	e.InstagramAccessToken = decryptToken(e.InstagramAccessToken)
	return e, nil
}

func (r *userRepo) Update(ctx context.Context, e *entities.User) error {
	m := &models.User{}
	m.FromEntity(e)
	m.InstagramAccessToken = encryptToken(m.InstagramAccessToken)
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *userRepo) GetByInstagramID(ctx context.Context, instagramID string) (*entities.User, error) {
	m := &models.User{}
	if err := r.db.WithContext(ctx).Where("instagram_account_id = ?", instagramID).First(m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	e := m.ToEntity()
	e.InstagramAccessToken = decryptToken(e.InstagramAccessToken)
	return e, nil
}

func (r *userRepo) DeleteByInstagramID(ctx context.Context, instagramID string) error {
	return r.db.WithContext(ctx).Where("instagram_account_id = ?", instagramID).Delete(&models.User{}).Error
}

func (r *userRepo) GetAllWithInstagramToken(ctx context.Context) ([]*entities.User, error) {
	var ms []models.User
	if err := r.db.WithContext(ctx).Where("instagram_access_token IS NOT NULL AND instagram_access_token != ?", "").Find(&ms).Error; err != nil {
		return nil, err
	}

	res := make([]*entities.User, len(ms))
	for i, m := range ms {
		e := m.ToEntity()
		e.InstagramAccessToken = decryptToken(e.InstagramAccessToken)
		res[i] = e
	}
	return res, nil
}

