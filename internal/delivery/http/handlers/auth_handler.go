package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/azharf99/ig-gateway-api/internal/usecase/auth"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authUsecase auth.Usecase
}

func NewAuthHandler(authUsecase auth.Usecase) *AuthHandler {
	return &AuthHandler{authUsecase: authUsecase}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input auth.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authUsecase.Register(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input auth.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, user, err := h.authUsecase.Login(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":                     user.ID,
			"username":               user.Username,
			"email":                  user.Email,
			"instagram_connected":    user.InstagramAccountID != "",
			"instagram_account_id":   user.InstagramAccountID,
		},
	})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	user, err := h.authUsecase.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                   user.ID,
			"username":             user.Username,
			"email":                user.Email,
			"instagram_connected":  user.InstagramAccountID != "",
			"instagram_account_id": user.InstagramAccountID,
		},
	})
}

func (h *AuthHandler) GetInstagramOAuthURL(c *gin.Context) {
	clientID := config.AppConfig.IGClientID
	redirectURI := config.AppConfig.IGRedirectURI
	scopes := []string{
		"instagram_basic",
		"instagram_content_publish",
		"pages_read_engagement",
		"pages_show_list",
		"business_management",
	}

	extras := `{"setup":{"channel":"IG_API_ONBOARDING"}}`

	oauthURL := fmt.Sprintf(
		"https://www.facebook.com/v25.0/dialog/oauth?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&display=page&extras=%s",
		clientID,
		url.QueryEscape(redirectURI),
		url.QueryEscape(stringsJoin(scopes, ",")),
		url.QueryEscape(extras),
	)

	c.JSON(http.StatusOK, gin.H{"url": oauthURL})
}

func (h *AuthHandler) LinkInstagram(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.authUsecase.LinkInstagram(c.Request.Context(), userID, req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Instagram account linked successfully"})
}

// Helpers
func stringsJoin(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	if len(elems) == 1 {
		return elems[0]
	}
	n := len(sep) * (len(elems) - 1)
	for i := 0; i < len(elems); i++ {
		n += len(elems[i])
	}

	var b []byte
	b = append(b, elems[0]...)
	for _, s := range elems[1:] {
		b = append(b, sep...)
		b = append(b, s...)
	}
	return string(b)
}
