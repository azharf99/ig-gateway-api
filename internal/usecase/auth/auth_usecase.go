package auth

import (
	"context"
	"errors"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/domain/repositories"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/instagram"
	"github.com/azharf99/ig-gateway-api/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type Usecase interface {
	Register(ctx context.Context, input RegisterInput) (*entities.User, error)
	Login(ctx context.Context, input LoginInput) (string, *entities.User, error)
	GetProfile(ctx context.Context, userID uint) (*entities.User, error)
	LinkInstagram(ctx context.Context, userID uint, code string, state string) error
	SaveOAuthState(ctx context.Context, userID uint, state string) error
	RefreshExpiredTokens(ctx context.Context) error
}

type authUsecase struct {
	userRepo repositories.UserRepository
	igClient instagram.Client
}

func NewAuthUsecase(userRepo repositories.UserRepository, igClient instagram.Client) Usecase {
	return &authUsecase{
		userRepo: userRepo,
		igClient: igClient,
	}
}

func (u *authUsecase) Register(ctx context.Context, input RegisterInput) (*entities.User, error) {
	existing, err := u.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("email is already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entities.User{
		Username: input.Username,
		Email:    input.Email,
		Password: string(hashedPassword),
	}

	if err := u.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (u *authUsecase) Login(ctx context.Context, input LoginInput) (string, *entities.User, error) {
	user, err := u.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return "", nil, errors.New("invalid email or password")
	}

	token, err := jwt.GenerateToken(user.ID)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func (u *authUsecase) GetProfile(ctx context.Context, userID uint) (*entities.User, error) {
	return u.userRepo.GetByID(ctx, userID)
}

func (u *authUsecase) SaveOAuthState(ctx context.Context, userID uint, state string) error {
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	user.OAuthState = state
	return u.userRepo.Update(ctx, user)
}

func (u *authUsecase) LinkInstagram(ctx context.Context, userID uint, code string, state string) error {
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	// CSRF validation
	if state == "" || user.OAuthState == "" || user.OAuthState != state {
		return errors.New("invalid oauth state (possible CSRF attack)")
	}

	// Clear the state after use
	user.OAuthState = ""

	// 1. Exchange OAuth code for a short-lived user access token
	shortToken, err := u.igClient.GetShortLivedToken(code)
	if err != nil {
		// Reset state so it cannot be reused
		_ = u.userRepo.Update(ctx, user)
		return err
	}

	// 2. Exchange short-lived token for a long-lived user/page access token
	longToken, err := u.igClient.GetLongLivedToken(shortToken)
	if err != nil {
		_ = u.userRepo.Update(ctx, user)
		return err
	}

	// 3. Find connected Instagram business account ID using longToken
	igAccountID, err := u.igClient.GetInstagramAccountID(longToken)
	if err != nil {
		_ = u.userRepo.Update(ctx, user)
		return err
	}

	// 4. Update user entity in the DB
	user.InstagramAccessToken = longToken
	user.InstagramAccountID = igAccountID

	return u.userRepo.Update(ctx, user)
}

func (u *authUsecase) RefreshExpiredTokens(ctx context.Context) error {
	users, err := u.userRepo.GetAllWithInstagramToken(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		newToken, err := u.igClient.RefreshLongLivedToken(user.InstagramAccessToken)
		if err != nil {
			// Log error and continue to avoid failing the whole batch
			continue
		}

		user.InstagramAccessToken = newToken
		_ = u.userRepo.Update(ctx, user)
	}

	return nil
}

