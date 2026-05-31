package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/config"
)

type Client interface {
	GetShortLivedToken(code string) (string, error)
	GetLongLivedToken(shortLivedToken string) (string, error)
	GetInstagramAccountID(pageAccessToken string) (string, error)
	PublishPhoto(igAccountID, accessToken, imageURL, caption string) (string, error)
	PublishVideo(igAccountID, accessToken, videoURL, caption string, isReels bool) (string, error)
	PublishCarousel(igAccountID, accessToken, caption string, items []CarouselItem) (string, error)
	RefreshLongLivedToken(currentToken string) (string, error)
}

type CarouselItem struct {
	MediaURL  string
	MediaType string // "image" or "video"
}

type instagramClient struct {
	httpClient *http.Client
	graphURL   string
}

func NewInstagramClient() Client {
	return &instagramClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		graphURL:   "https://graph.facebook.com/v25.0",
	}
}

// generateAppSecretProof calculates the HMAC-SHA256 signature for Meta's appsecret_proof requirement
func generateAppSecretProof(accessToken string) string {
	h := hmac.New(sha256.New, []byte(config.AppConfig.IGClientSecret))
	h.Write([]byte(accessToken))
	return hex.EncodeToString(h.Sum(nil))
}

// GetShortLivedToken exchanges the OAuth code for a short-lived user access token
func (c *instagramClient) GetShortLivedToken(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", config.AppConfig.IGClientID)
	data.Set("client_secret", config.AppConfig.IGClientSecret)
	data.Set("redirect_uri", config.AppConfig.IGRedirectURI)
	data.Set("code", code)

	resp, err := c.httpClient.PostForm("https://graph.facebook.com/v25.0/oauth/access_token", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth failed with status %d", resp.StatusCode)
	}

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}

	return res.AccessToken, nil
}

// GetLongLivedToken exchanges a short-lived access token for a long-lived page/user token (60 days)
func (c *instagramClient) GetLongLivedToken(shortLivedToken string) (string, error) {
	proof := generateAppSecretProof(shortLivedToken)
	u := fmt.Sprintf("%s/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s&appsecret_proof=%s",
		c.graphURL, config.AppConfig.IGClientID, config.AppConfig.IGClientSecret, shortLivedToken, proof)

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get long-lived token: status code %d", resp.StatusCode)
	}

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}

	return res.AccessToken, nil
}

// RefreshLongLivedToken refreshes a long-lived access token (generating a new one valid for 60 days)
func (c *instagramClient) RefreshLongLivedToken(currentToken string) (string, error) {
	proof := generateAppSecretProof(currentToken)
	u := fmt.Sprintf("%s/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s&appsecret_proof=%s",
		c.graphURL, config.AppConfig.IGClientID, config.AppConfig.IGClientSecret, currentToken, proof)

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to refresh token: status code %d", resp.StatusCode)
	}

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}

	return res.AccessToken, nil
}

// GetInstagramAccountID fetches the Instagram Business/Creator account ID from the linked Facebook Pages
func (c *instagramClient) GetInstagramAccountID(userAccessToken string) (string, error) {
	proof := generateAppSecretProof(userAccessToken)
	u := fmt.Sprintf("%s/me/accounts?fields=id,name,instagram_business_account&access_token=%s&appsecret_proof=%s", c.graphURL, userAccessToken, proof)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get FB Pages: status code %d", resp.StatusCode)
	}

	var pageResponse struct {
		Data []struct {
			ID                       string `json:"id"`
			Name                     string `json:"name"`
			InstagramBusinessAccount struct {
				ID string `json:"id"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &pageResponse); err != nil {
		return "", err
	}

	// Look for a page that has an Instagram Business/Creator Account linked
	for _, page := range pageResponse.Data {
		if page.InstagramBusinessAccount.ID != "" {
			return page.InstagramBusinessAccount.ID, nil
		}
	}

	return "", fmt.Errorf("no linked Instagram Business or Creator account found on your Facebook Pages")
}

// PublishPhoto uploads and publishes a single image
func (c *instagramClient) PublishPhoto(igAccountID, accessToken, imageURL, caption string) (string, error) {
	params := url.Values{}
	params.Set("image_url", imageURL)
	params.Set("caption", caption)
	params.Set("access_token", accessToken)

	containerID, err := c.createContainer(igAccountID, params)
	if err != nil {
		return "", err
	}

	return c.publishContainer(igAccountID, containerID, accessToken)
}

// PublishVideo uploads and publishes a single video or Reel
func (c *instagramClient) PublishVideo(igAccountID, accessToken, videoURL, caption string, isReels bool) (string, error) {
	params := url.Values{}
	params.Set("media_type", "REELS")
	params.Set("video_url", videoURL)
	params.Set("caption", caption)
	params.Set("access_token", accessToken)
	if isReels {
		params.Set("share_to_feed", "true")
	}

	containerID, err := c.createContainer(igAccountID, params)
	if err != nil {
		return "", err
	}

	if err := c.pollContainerStatus(containerID, accessToken); err != nil {
		return "", err
	}

	return c.publishContainer(igAccountID, containerID, accessToken)
}

// PublishCarousel uploads and publishes a carousel (up to 10 images/videos)
func (c *instagramClient) PublishCarousel(igAccountID, accessToken, caption string, items []CarouselItem) (string, error) {
	if len(items) < 2 || len(items) > 10 {
		return "", fmt.Errorf("carousel must have between 2 and 10 items")
	}

	var childIDs []string

	for _, item := range items {
		params := url.Values{}
		params.Set("is_carousel_item", "true")
		params.Set("access_token", accessToken)

		if item.MediaType == "video" {
			params.Set("media_type", "VIDEO")
			params.Set("video_url", item.MediaURL)
		} else {
			params.Set("image_url", item.MediaURL)
		}

		childID, err := c.createContainer(igAccountID, params)
		if err != nil {
			return "", fmt.Errorf("failed to create container for item: %v", err)
		}
		childIDs = append(childIDs, childID)
	}

	for i, item := range items {
		if item.MediaType == "video" {
			if err := c.pollContainerStatus(childIDs[i], accessToken); err != nil {
				return "", fmt.Errorf("video processing failed for carousel item %d: %v", i+1, err)
			}
		}
	}

	parentParams := url.Values{}
	parentParams.Set("media_type", "CAROUSEL")
	parentParams.Set("caption", caption)
	parentParams.Set("children", strings.Join(childIDs, ","))
	parentParams.Set("access_token", accessToken)

	parentContainerID, err := c.createContainer(igAccountID, parentParams)
	if err != nil {
		return "", fmt.Errorf("failed to create parent carousel container: %v", err)
	}

	return c.publishContainer(igAccountID, parentContainerID, accessToken)
}

// Helpers
func (c *instagramClient) createContainer(igAccountID string, params url.Values) (string, error) {
	apiURL := fmt.Sprintf("%s/%s/media", c.graphURL, igAccountID)

	accessToken := params.Get("access_token")
	if accessToken != "" {
		params.Set("appsecret_proof", generateAppSecretProof(accessToken))
	}

	// Post params as application/x-www-form-urlencoded inside the request body
	resp, err := c.httpClient.PostForm(apiURL, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("container creation failed: status %d", resp.StatusCode)
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}

	return res.ID, nil
}

func (c *instagramClient) publishContainer(igAccountID, containerID, accessToken string) (string, error) {
	apiURL := fmt.Sprintf("%s/%s/media_publish", c.graphURL, igAccountID)
	params := url.Values{}
	params.Set("creation_id", containerID)
	params.Set("access_token", accessToken)
	params.Set("appsecret_proof", generateAppSecretProof(accessToken))

	resp, err := c.httpClient.PostForm(apiURL, params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("publishing failed: status %d", resp.StatusCode)
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}

	return res.ID, nil
}

func (c *instagramClient) pollContainerStatus(containerID, accessToken string) error {
	proof := generateAppSecretProof(accessToken)
	apiURL := fmt.Sprintf("%s/%s?fields=status_code,status&access_token=%s&appsecret_proof=%s", c.graphURL, containerID, accessToken, proof)

	for i := 0; i < 30; i++ {
		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status check failed: status %d", resp.StatusCode)
		}

		var res struct {
			StatusCode string `json:"status_code"`
			Status     string `json:"status"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return err
		}

		if res.StatusCode == "FINISHED" {
			return nil
		}
		if res.StatusCode == "ERROR" {
			return fmt.Errorf("facebook container video processing error")
		}

		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("video processing timed out on Instagram servers")
}

