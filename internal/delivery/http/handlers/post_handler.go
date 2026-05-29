package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/storage"
	"github.com/azharf99/ig-gateway-api/internal/usecase/post"
	"github.com/gin-gonic/gin"
)

type PostHandler struct {
	postUsecase post.Usecase
	storageServ storage.StorageService
}

func NewPostHandler(postUsecase post.Usecase, storageServ storage.StorageService) *PostHandler {
	return &PostHandler{
		postUsecase: postUsecase,
		storageServ: storageServ,
	}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	// Parse Multipart Form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	caption := c.PostForm("caption")
	postTypeStr := c.PostForm("post_type")
	scheduledAtStr := c.PostForm("scheduled_at")

	// Validate post type
	postType := entities.PostType(postTypeStr)
	if postType != entities.PostTypePhoto &&
		postType != entities.PostTypeVideo &&
		postType != entities.PostTypeCarousel &&
		postType != entities.PostTypeReels {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post type. Must be photo, video, carousel, or reels"})
		return
	}

	// Parse ScheduledAt
	var scheduledAt *time.Time
	if scheduledAtStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, scheduledAtStr)
		if err != nil {
			// Try fallback unix timestamp
			unixTime, unixErr := strconv.ParseInt(scheduledAtStr, 10, 64)
			if unixErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduled_at format. Use ISO8601 (RFC3339) or Unix timestamp"})
				return
			}
			t := time.Unix(unixTime, 0)
			scheduledAt = &t
		} else {
			scheduledAt = &parsedTime
		}
	}

	// Retrieve uploaded files
	files := form.File["media"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No media files uploaded"})
		return
	}

	// Limit carousel files
	if postType == entities.PostTypeCarousel && (len(files) < 2 || len(files) > 10) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Carousel posts must have between 2 and 10 media files"})
		return
	} else if postType != entities.PostTypeCarousel && len(files) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only carousel posts support multiple media files"})
		return
	}

	// Save media files
	var mediaList []entities.PostMedia
	for idx, fileHeader := range files {
		// Detect media type from extension
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		mediaType := "image"
		if ext == ".mp4" || ext == ".mov" || ext == ".avi" || ext == ".mkv" {
			mediaType = "video"
		}

		// Validation: reels must be video
		if postType == entities.PostTypeReels && mediaType != "video" {
			// Cleanup saved files if any
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Reels post type only supports video files"})
			return
		}

		savedPath, err := h.storageServ.SaveFile(fileHeader)
		if err != nil {
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save uploaded file: %v", err)})
			return
		}

		mediaList = append(mediaList, entities.PostMedia{
			MediaURL:  savedPath,
			Order:     idx,
			MediaType: mediaType,
		})
	}

	input := post.CreatePostInput{
		UserID:      userID,
		Caption:     caption,
		PostType:    postType,
		ScheduledAt: scheduledAt,
		MediaFiles:  mediaList,
	}

	createdPost, err := h.postUsecase.CreatePost(c.Request.Context(), input)
	if err != nil {
		h.cleanupMediaList(mediaList)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Post created successfully",
		"post":    createdPost,
	})
}

func (h *PostHandler) GetPosts(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	posts, err := h.postUsecase.GetPosts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Add public URL mapping to media
	for i := range posts {
		for j := range posts[i].Media {
			posts[i].Media[j].MediaURL = h.storageServ.GetPublicURL(posts[i].Media[j].MediaURL)
		}
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func (h *PostHandler) GetPostByID(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	postIDStr := c.Param("id")

	postID, err := strconv.ParseUint(postIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	p, err := h.postUsecase.GetPostByID(c.Request.Context(), uint(postID), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Add public URL mapping to media
	for j := range p.Media {
		p.Media[j].MediaURL = h.storageServ.GetPublicURL(p.Media[j].MediaURL)
	}

	c.JSON(http.StatusOK, gin.H{"post": p})
}

func (h *PostHandler) DeletePost(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	postIDStr := c.Param("id")

	postID, err := strconv.ParseUint(postIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	err = h.postUsecase.DeletePost(c.Request.Context(), uint(postID), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

// Helpers
func (h *PostHandler) cleanupMediaList(mediaList []entities.PostMedia) {
	for _, m := range mediaList {
		_ = h.storageServ.DeleteFile(m.MediaURL)
	}
}
