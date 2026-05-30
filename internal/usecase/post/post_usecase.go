package post

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/domain/entities"
	"github.com/azharf99/ig-gateway-api/internal/domain/repositories"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/instagram"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/storage"
	"github.com/azharf99/ig-gateway-api/pkg/media"
)

type CreatePostInput struct {
	UserID       uint                 `json:"-"`
	Caption      string               `json:"caption"`
	PostType     entities.PostType    `json:"post_type"` // "photo", "video", "carousel", "reels"
	ScheduledAt  *time.Time           `json:"scheduled_at"`
	MediaFiles   []entities.PostMedia `json:"-"`
	EditMetadata []media.EditMetadata `json:"-"`
	AudioPath    string               `json:"-"`
	LogoPath     string               `json:"-"`
	SubtitlePath string               `json:"-"`
}

type Usecase interface {
	CreatePost(ctx context.Context, input CreatePostInput) (*entities.Post, error)
	GetPosts(ctx context.Context, userID uint) ([]entities.Post, error)
	GetPostByID(ctx context.Context, postID uint, userID uint) (*entities.Post, error)
	DeletePost(ctx context.Context, postID uint, userID uint) error
	PublishPostNow(ctx context.Context, postID uint) error
	ProcessScheduledPosts(ctx context.Context) error
}

type postUsecase struct {
	postRepo    repositories.PostRepository
	userRepo    repositories.UserRepository
	igClient    instagram.Client
	storageServ storage.StorageService
}

func NewPostUsecase(
	postRepo repositories.PostRepository,
	userRepo repositories.UserRepository,
	igClient instagram.Client,
	storageServ storage.StorageService,
) Usecase {
	return &postUsecase{
		postRepo:    postRepo,
		userRepo:    userRepo,
		igClient:    igClient,
		storageServ: storageServ,
	}
}

func (u *postUsecase) CreatePost(ctx context.Context, input CreatePostInput) (*entities.Post, error) {
	if len(input.MediaFiles) == 0 {
		return nil, errors.New("at least one media file is required")
	}

	// Validate user exists and is linked
	user, err := u.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	status := entities.PostStatusDraft
	if input.ScheduledAt != nil {
		if input.ScheduledAt.Before(time.Now()) {
			return nil, errors.New("scheduled time must be in the future")
		}
		status = entities.PostStatusScheduled
	}

	// Clean up uploaded audio/logo/subtitle files at the end of the usecase
	if input.AudioPath != "" {
		defer os.Remove(input.AudioPath)
	}
	if input.LogoPath != "" {
		defer os.Remove(input.LogoPath)
	}
	if input.SubtitlePath != "" {
		defer os.Remove(input.SubtitlePath)
	}

	// Process videos if edit metadata is provided
	for i := range input.MediaFiles {
		if input.MediaFiles[i].MediaType == "video" && i < len(input.EditMetadata) {
			meta := input.EditMetadata[i]
			// Check if we actually need to edit the video
			if meta.Text != "" || meta.HasLogo || meta.HasAudio || meta.MuteAudio || meta.HasSubtitles {
				origRelativePath := input.MediaFiles[i].MediaURL

				// Generate unique output filename
				ext := filepath.Ext(origRelativePath)
				base := strings.TrimSuffix(filepath.Base(origRelativePath), ext)
				procFilename := fmt.Sprintf("%s-proc%s", base, ext)
				processedRelativePath := filepath.Join(filepath.Dir(origRelativePath), procFilename)

				log.Printf("[VideoProcessor] Processing video %s -> %s\n", origRelativePath, processedRelativePath)

				// Run FFmpeg processing
				err := media.ProcessVideo(ctx, origRelativePath, input.AudioPath, input.LogoPath, input.SubtitlePath, meta, processedRelativePath)
				if err != nil {
					log.Printf("[VideoProcessor] Error processing video: %v\n", err)
					return nil, fmt.Errorf("failed to process video: %w", err)
				}

				// Delete original file since we now have the processed one
				_ = os.Remove(origRelativePath)

				// Update media model URL to use the processed path
				input.MediaFiles[i].MediaURL = filepath.ToSlash(processedRelativePath)
			}
		}
	}

	// Generate thumbnails for all video files
	for i := range input.MediaFiles {
		if input.MediaFiles[i].MediaType == "video" {
			videoPath := input.MediaFiles[i].MediaURL
			ext := filepath.Ext(videoPath)
			thumbPath := strings.TrimSuffix(videoPath, ext) + "-thumb.jpg"

			log.Printf("[VideoProcessor] Generating thumbnail for video %s -> %s\n", videoPath, thumbPath)
			err := media.GenerateThumbnail(ctx, videoPath, thumbPath)
			if err != nil {
				log.Printf("[VideoProcessor] Error generating thumbnail: %v\n", err)
			} else {
				input.MediaFiles[i].ThumbnailURL = filepath.ToSlash(thumbPath)
			}
		}
	}

	post := &entities.Post{
		UserID:      input.UserID,
		Caption:     input.Caption,
		PostType:    input.PostType,
		Status:      status,
		ScheduledAt: input.ScheduledAt,
		Media:       input.MediaFiles,
	}

	if err := u.postRepo.Create(ctx, post); err != nil {
		return nil, err
	}

	// If not scheduled, publish immediately in a non-blocking goroutine
	if input.ScheduledAt == nil {
		go func(id uint) {
			ctxBg := context.Background()
			_ = u.PublishPostNow(ctxBg, id)
		}(post.ID)
	}

	return post, nil
}

func (u *postUsecase) GetPosts(ctx context.Context, userID uint) ([]entities.Post, error) {
	return u.postRepo.GetByUserID(ctx, userID)
}

func (u *postUsecase) GetPostByID(ctx context.Context, postID uint, userID uint) (*entities.Post, error) {
	post, err := u.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil || post.UserID != userID {
		return nil, errors.New("post not found")
	}
	return post, nil
}

func (u *postUsecase) DeletePost(ctx context.Context, postID uint, userID uint) error {
	post, err := u.postRepo.GetByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil || post.UserID != userID {
		return errors.New("post not found")
	}

	// Clean up all local media and thumbnail files
	for _, m := range post.Media {
		if m.MediaURL != "" {
			_ = u.storageServ.DeleteFile(m.MediaURL)
		}
		if m.ThumbnailURL != "" {
			_ = u.storageServ.DeleteFile(m.ThumbnailURL)
		}
	}

	return u.postRepo.Delete(ctx, postID)
}

func (u *postUsecase) PublishPostNow(ctx context.Context, postID uint) error {
	post, err := u.postRepo.GetByID(ctx, postID)
	if err != nil {
		return err
	}
	if post == nil {
		return fmt.Errorf("post %d not found", postID)
	}

	// Check if already published or in progress
	if post.Status == entities.PostStatusPublished || post.Status == entities.PostStatusPosting {
		return nil
	}

	// Update status to posting
	post.Status = entities.PostStatusPosting
	_ = u.postRepo.Update(ctx, post)

	// Fetch linked user's credentials
	user, err := u.userRepo.GetByID(ctx, post.UserID)
	if err != nil || user == nil || user.InstagramAccessToken == "" || user.InstagramAccountID == "" {
		post.Status = entities.PostStatusFailed
		post.ErrorMessage = "Instagram account not linked or error retrieving credentials"
		_ = u.postRepo.Update(ctx, post)
		return errors.New("instagram account not linked")
	}

	var publishErr error
	var igMediaID string

	// Core publishing logic based on PostType
	switch post.PostType {
	case entities.PostTypePhoto:
		publicURL := u.storageServ.GetPublicURL(post.Media[0].MediaURL)
		igMediaID, publishErr = u.igClient.PublishPhoto(user.InstagramAccountID, user.InstagramAccessToken, publicURL, post.Caption)

	case entities.PostTypeVideo:
		publicURL := u.storageServ.GetPublicURL(post.Media[0].MediaURL)
		igMediaID, publishErr = u.igClient.PublishVideo(user.InstagramAccountID, user.InstagramAccessToken, publicURL, post.Caption, false)

	case entities.PostTypeReels:
		publicURL := u.storageServ.GetPublicURL(post.Media[0].MediaURL)
		igMediaID, publishErr = u.igClient.PublishVideo(user.InstagramAccountID, user.InstagramAccessToken, publicURL, post.Caption, true)

	case entities.PostTypeCarousel:
		carouselItems := make([]instagram.CarouselItem, len(post.Media))
		for i, m := range post.Media {
			carouselItems[i] = instagram.CarouselItem{
				MediaURL:  u.storageServ.GetPublicURL(m.MediaURL),
				MediaType: m.MediaType,
			}
		}
		igMediaID, publishErr = u.igClient.PublishCarousel(user.InstagramAccountID, user.InstagramAccessToken, post.Caption, carouselItems)

	default:
		publishErr = fmt.Errorf("unsupported post type: %s", post.PostType)
	}

	// Update post status based on result
	if publishErr != nil {
		log.Printf("Failed to publish post %d to IG: %v\n", post.ID, publishErr)
		post.Status = entities.PostStatusFailed
		post.ErrorMessage = publishErr.Error()
	} else {
		log.Printf("Successfully published post %d to IG. Media ID: %s\n", post.ID, igMediaID)
		post.Status = entities.PostStatusPublished
		now := time.Now()
		post.PublishedAt = &now
		post.ErrorMessage = ""

		// Delete local file to save storage since it's published to IG
		for _, m := range post.Media {
			_ = u.storageServ.DeleteFile(m.MediaURL)
		}
	}

	return u.postRepo.Update(ctx, post)
}

func (u *postUsecase) ProcessScheduledPosts(ctx context.Context) error {
	posts, err := u.postRepo.GetScheduledActive(ctx)
	if err != nil {
		return err
	}

	if len(posts) == 0 {
		return nil
	}

	log.Printf("Found %d scheduled posts to process...\n", len(posts))

	for _, p := range posts {
		// Run as concurrently as possible, but to prevent DB lock, run sequential or small worker pool
		// In VPS environment, sequential processing of scheduled posts is usually safe and reliable
		err := u.PublishPostNow(ctx, p.ID)
		if err != nil {
			log.Printf("Scheduler failed processing post %d: %v\n", p.ID, err)
		}
	}

	return nil
}
