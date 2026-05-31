package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/storage"
	"github.com/azharf99/ig-gateway-api/internal/usecase/post"
	"github.com/azharf99/ig-gateway-api/pkg/media"
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

// validateUploadedFile checks the file size and reads the first 512 bytes to determine the actual MIME type.
func validateUploadedFile(fileHeader *multipart.FileHeader, maxAllowedSize int64, allowedMimeTypes []string) (string, error) {
	if fileHeader.Size > maxAllowedSize {
		return "", fmt.Errorf("file size %d exceeds the maximum limit of %d bytes", fileHeader.Size, maxAllowedSize)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file for validation")
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file content header")
	}

	contentType := http.DetectContentType(buffer[:n])
	if contentType == "application/octet-stream" {
		contentType = fileHeader.Header.Get("Content-Type")
	}

	allowed := false
	for _, t := range allowedMimeTypes {
		if strings.HasPrefix(contentType, t) || contentType == t {
			allowed = true
			break
		}
	}

	if !allowed {
		return "", fmt.Errorf("file type '%s' is not allowed", contentType)
	}

	return contentType, nil
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	caption := c.PostForm("caption")
	postTypeStr := c.PostForm("post_type")
	scheduledAtStr := c.PostForm("scheduled_at")

	postType := entities.PostType(postTypeStr)
	if postType != entities.PostTypePhoto &&
		postType != entities.PostTypeVideo &&
		postType != entities.PostTypeCarousel &&
		postType != entities.PostTypeReels {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post type. Must be photo, video, carousel, or reels"})
		return
	}

	var scheduledAt *time.Time
	if scheduledAtStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, scheduledAtStr)
		if err != nil {
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

	files := form.File["media"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No media files uploaded"})
		return
	}

	if postType == entities.PostTypeCarousel && (len(files) < 2 || len(files) > 10) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Carousel posts must have between 2 and 10 media files"})
		return
	} else if postType != entities.PostTypeCarousel && len(files) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only carousel posts support multiple media files"})
		return
	}

	var mediaList []entities.PostMedia
	for idx, fileHeader := range files {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		mediaType := "image"
		if ext == ".mp4" || ext == ".mov" || ext == ".avi" || ext == ".mkv" {
			mediaType = "video"
		}

		if postType == entities.PostTypeReels && mediaType != "video" {
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Reels post type only supports video files"})
			return
		}

		// Security: Validate file content type and size limits (10MB for image, 50MB for video)
		var maxAllowedSize int64 = 10 << 20
		allowedMimes := []string{"image/jpeg", "image/png", "image/webp"}
		if mediaType == "video" {
			maxAllowedSize = 50 << 20
			allowedMimes = []string{"video/mp4", "video/quicktime", "video/x-msvideo", "video/x-matroska"}
		}

		_, err = validateUploadedFile(fileHeader, maxAllowedSize, allowedMimes)
		if err != nil {
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid media file: %v", err)})
			return
		}

		savedPath, err := h.storageServ.SaveFile(fileHeader)
		if err != nil {
			log.Printf("[ERROR] Failed to save media file: %v", err)
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file"})
			return
		}

		mediaList = append(mediaList, entities.PostMedia{
			MediaURL:  savedPath,
			Order:     idx,
			MediaType: mediaType,
		})
	}

	editMetadataStr := c.PostForm("edit_metadata")
	var editMetadata []media.EditMetadata
	if editMetadataStr != "" {
		if err := json.Unmarshal([]byte(editMetadataStr), &editMetadata); err != nil {
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid edit_metadata JSON format"})
			return
		}
	}

	var audioPath string
	var logoPath string
	var subtitlePath string

	audioHeader, err := c.FormFile("audio_file")
	if err == nil && audioHeader != nil {
		// Limit to 10MB, allow safe audio types
		_, err = validateUploadedFile(audioHeader, 10<<20, []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/aac", "audio/mp3", "application/octet-stream"})
		if err != nil {
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid audio file: %v", err)})
			return
		}

		savedAudio, err := h.storageServ.SaveFile(audioHeader)
		if err != nil {
			log.Printf("[ERROR] Failed to save audio file: %v", err)
			h.cleanupMediaList(mediaList)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio file"})
			return
		}
		audioPath = savedAudio
	}

	logoHeader, err := c.FormFile("logo_file")
	if err == nil && logoHeader != nil {
		// Limit to 2MB, allow safe image types
		_, err = validateUploadedFile(logoHeader, 2<<20, []string{"image/jpeg", "image/png", "image/webp"})
		if err != nil {
			h.cleanupMediaList(mediaList)
			if audioPath != "" {
				_ = h.storageServ.DeleteFile(audioPath)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid logo file: %v", err)})
			return
		}

		savedLogo, err := h.storageServ.SaveFile(logoHeader)
		if err != nil {
			log.Printf("[ERROR] Failed to save logo file: %v", err)
			h.cleanupMediaList(mediaList)
			if audioPath != "" {
				_ = h.storageServ.DeleteFile(audioPath)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save logo file"})
			return
		}
		logoPath = savedLogo
	}

	subtitleHeader, err := c.FormFile("subtitle_file")
	if err == nil && subtitleHeader != nil {
		// Limit to 1MB, allow subtitle plain texts/binaries
		_, err = validateUploadedFile(subtitleHeader, 1<<20, []string{"text/plain", "application/octet-stream", "text/vtt"})
		if err != nil {
			h.cleanupMediaList(mediaList)
			if audioPath != "" {
				_ = h.storageServ.DeleteFile(audioPath)
			}
			if logoPath != "" {
				_ = h.storageServ.DeleteFile(logoPath)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid subtitle file: %v", err)})
			return
		}

		savedSubtitle, err := h.storageServ.SaveFile(subtitleHeader)
		if err != nil {
			log.Printf("[ERROR] Failed to save subtitle file: %v", err)
			h.cleanupMediaList(mediaList)
			if audioPath != "" {
				_ = h.storageServ.DeleteFile(audioPath)
			}
			if logoPath != "" {
				_ = h.storageServ.DeleteFile(logoPath)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save subtitle file"})
			return
		}
		subtitlePath = savedSubtitle
	}

	input := post.CreatePostInput{
		UserID:       userID,
		Caption:      caption,
		PostType:     postType,
		ScheduledAt:  scheduledAt,
		MediaFiles:   mediaList,
		EditMetadata: editMetadata,
		AudioPath:    audioPath,
		LogoPath:     logoPath,
		SubtitlePath: subtitlePath,
	}

	createdPost, err := h.postUsecase.CreatePost(c.Request.Context(), input)
	if err != nil {
		h.cleanupMediaList(mediaList)
		if audioPath != "" {
			_ = h.storageServ.DeleteFile(audioPath)
		}
		if logoPath != "" {
			_ = h.storageServ.DeleteFile(logoPath)
		}
		if subtitlePath != "" {
			_ = h.storageServ.DeleteFile(subtitlePath)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create post. Invalid post inputs or media processing failed."})
		return
	}

	message := "Post created successfully"
	if createdPost.Status == "processing" {
		message = "Post created. Video is being processed in the background."
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": message,
		"post":    createdPost,
	})
}

func (h *PostHandler) GetPosts(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	posts, err := h.postUsecase.GetPosts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve posts"})
		return
	}

	for i := range posts {
		for j := range posts[i].Media {
			posts[i].Media[j].MediaURL = h.storageServ.GetPublicURL(posts[i].Media[j].MediaURL)
			if posts[i].Media[j].ThumbnailURL != "" {
				posts[i].Media[j].ThumbnailURL = h.storageServ.GetPublicURL(posts[i].Media[j].ThumbnailURL)
			}
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	for j := range p.Media {
		p.Media[j].MediaURL = h.storageServ.GetPublicURL(p.Media[j].MediaURL)
		if p.Media[j].ThumbnailURL != "" {
			p.Media[j].ThumbnailURL = h.storageServ.GetPublicURL(p.Media[j].ThumbnailURL)
		}
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to delete post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

func (h *PostHandler) cleanupMediaList(mediaList []entities.PostMedia) {
	for _, m := range mediaList {
		_ = h.storageServ.DeleteFile(m.MediaURL)
	}
}

