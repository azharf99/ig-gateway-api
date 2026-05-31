package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/azharf99/ig-gateway-api/internal/domain/repositories"
	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	userRepo repositories.UserRepository
}

func NewWebhookHandler(userRepo repositories.UserRepository) *WebhookHandler {
	return &WebhookHandler{userRepo: userRepo}
}

// parseSignedRequest decodes and validates Facebook's Signed Request format
func parseSignedRequest(signedRequest string, appSecret string) (map[string]interface{}, error) {
	parts := strings.Split(signedRequest, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid signed request format")
	}

	encodedSig := parts[0]
	encodedPayload := parts[1]

	base64URLDecode := func(s string) ([]byte, error) {
		switch len(s) % 4 {
		case 2:
			s += "=="
		case 3:
			s += "="
		}
		s = strings.ReplaceAll(s, "-", "+")
		s = strings.ReplaceAll(s, "_", "/")
		return base64.StdEncoding.DecodeString(s)
	}

	sig, err := base64URLDecode(encodedSig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature")
	}

	payloadBytes, err := base64URLDecode(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload")
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to parse payload json")
	}

	// Verify HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(encodedPayload))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, fmt.Errorf("invalid signature")
	}

	return data, nil
}

// DataDeletionCallback handles Instagram data deletion callback requests from Meta
func (h *WebhookHandler) DataDeletionCallback(c *gin.Context) {
	signedRequest := c.PostForm("signed_request")
	if signedRequest == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing signed_request parameter"})
		return
	}

	data, err := parseSignedRequest(signedRequest, config.AppConfig.IGClientSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request signature"})
		return
	}

	instagramUserID, ok := data["user_id"].(string)
	if !ok || instagramUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id in payload"})
		return
	}

	err = h.userRepo.DeleteByInstagramID(c.Request.Context(), instagramUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process data deletion request"})
		return
	}

	confirmationCode := fmt.Sprintf("del-%s", instagramUserID)

	c.JSON(http.StatusOK, gin.H{
		"url":               fmt.Sprintf("https://ig.azharfa.cloud/data-deletion-status?id=%s", confirmationCode),
		"confirmation_code": confirmationCode,
	})
}
