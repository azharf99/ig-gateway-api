package instagram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	log.Printf("[DEBUG] GetShortLivedToken Facebook Response Code: %d, Body: %s", resp.StatusCode, string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth failed: %s", string(body))
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
	u := fmt.Sprintf("%s/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s",
		c.graphURL, config.AppConfig.IGClientID, config.AppConfig.IGClientSecret, shortLivedToken)

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[DEBUG] GetLongLivedToken Facebook Response Code: %d, Body: %s", resp.StatusCode, string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get long-lived token: %s", string(body))
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
	// 1. Get Facebook Pages managed by user requesting name, id, and instagram_business_account
	u := fmt.Sprintf("%s/me/accounts?fields=id,name,instagram_business_account&access_token=%s", c.graphURL, userAccessToken)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[DEBUG] GetInstagramAccountID Facebook Response Code: %d, Body: %s", resp.StatusCode, string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get FB Pages: %s", string(body))
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
		log.Printf("[DEBUG] Found FB Page: %s (%s), Linked IG Account ID: %s", page.Name, page.ID, page.InstagramBusinessAccount.ID)
		if page.InstagramBusinessAccount.ID != "" {
			return page.InstagramBusinessAccount.ID, nil
		}
	}

	return "", fmt.Errorf("no linked Instagram Business or Creator account found on your Facebook Pages")
}

// PublishPhoto uploads and publishes a single image
func (c *instagramClient) PublishPhoto(igAccountID, accessToken, imageURL, caption string) (string, error) {
	// 1. Create Media Container
	params := url.Values{}
	params.Set("image_url", imageURL)
	params.Set("caption", caption)
	params.Set("access_token", accessToken)

	containerID, err := c.createContainer(igAccountID, params)
	if err != nil {
		return "", err
	}

	// 2. Publish Container
	return c.publishContainer(igAccountID, containerID, accessToken)
}

// PublishVideo uploads and publishes a single video or Reel
func (c *instagramClient) PublishVideo(igAccountID, accessToken, videoURL, caption string, isReels bool) (string, error) {
	// 1. Create Media Container
	params := url.Values{}
	params.Set("media_type", "VIDEO")
	params.Set("video_url", videoURL)
	params.Set("caption", caption)
	params.Set("access_token", accessToken)
	if isReels {
		params.Set("share_to_feed", "true") // Standard for Reels
	}

	containerID, err := c.createContainer(igAccountID, params)
	if err != nil {
		return "", err
	}

	// 2. Videos need to be processed by Instagram before publishing
	if err := c.pollContainerStatus(containerID, accessToken); err != nil {
		return "", err
	}

	// 3. Publish Container
	return c.publishContainer(igAccountID, containerID, accessToken)
}

// PublishCarousel uploads and publishes a carousel (up to 10 images/videos)
func (c *instagramClient) PublishCarousel(igAccountID, accessToken, caption string, items []CarouselItem) (string, error) {
	if len(items) < 2 || len(items) > 10 {
		return "", fmt.Errorf("carousel must have between 2 and 10 items")
	}

	var childIDs []string

	// 1. Create container for each item
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

	// 2. Wait for any video items to process
	for i, item := range items {
		if item.MediaType == "video" {
			if err := c.pollContainerStatus(childIDs[i], accessToken); err != nil {
				return "", fmt.Errorf("video processing failed for carousel item %d: %v", i+1, err)
			}
		}
	}

	// 3. Create parent Carousel container
	parentParams := url.Values{}
	parentParams.Set("media_type", "CAROUSEL")
	parentParams.Set("caption", caption)
	parentParams.Set("children", strings.Join(childIDs, ","))
	parentParams.Set("access_token", accessToken)

	parentContainerID, err := c.createContainer(igAccountID, parentParams)
	if err != nil {
		return "", fmt.Errorf("failed to create parent carousel container: %v", err)
	}

	// 4. Publish parent Carousel container
	return c.publishContainer(igAccountID, parentContainerID, accessToken)
}

// Helpers
func (c *instagramClient) createContainer(igAccountID string, params url.Values) (string, error) {
	apiURL := fmt.Sprintf("%s/%s/media", c.graphURL, igAccountID)

	resp, err := c.httpClient.Post(fmt.Sprintf("%s?%s", apiURL, params.Encode()), "application/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("container creation failed (code %d): %s", resp.StatusCode, string(body))
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

	resp, err := c.httpClient.Post(fmt.Sprintf("%s?%s", apiURL, params.Encode()), "application/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("publishing failed (code %d): %s", resp.StatusCode, string(body))
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
	apiURL := fmt.Sprintf("%s/%s?fields=status_code,status&access_token=%s", c.graphURL, containerID, accessToken)

	// Poll up to 5 minutes (30 attempts, 10s sleep)
	for i := 0; i < 30; i++ {
		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status check failed (code %d): %s", resp.StatusCode, string(body))
		}

		var res struct {
			StatusCode string `json:"status_code"`
			Status     string `json:"status"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return err
		}

		log.Printf("Polling container %s. Status: %s (%s)\n", containerID, res.StatusCode, res.Status)

		if res.StatusCode == "FINISHED" {
			return nil
		}
		if res.StatusCode == "ERROR" {
			return fmt.Errorf("facebook container video processing error: %s", res.Status)
		}

		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("video processing timed out on Instagram servers")
}
