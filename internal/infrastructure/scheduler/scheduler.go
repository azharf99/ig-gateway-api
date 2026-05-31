package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/usecase/auth"
	"github.com/azharf99/ig-gateway-api/internal/usecase/post"
	"github.com/go-co-op/gocron"
)

type Scheduler struct {
	s           *gocron.Scheduler
	postUsecase post.Usecase
	authUsecase auth.Usecase
}

func NewScheduler(postUsecase post.Usecase, authUsecase auth.Usecase) *Scheduler {
	s := gocron.NewScheduler(time.Local)
	return &Scheduler{
		s:           s,
		postUsecase: postUsecase,
		authUsecase: authUsecase,
	}
}

func (sc *Scheduler) Start() {
	// Runs every minute
	_, err := sc.s.Every(1).Minute().Do(func() {
		ctx := context.Background()
		if err := sc.postUsecase.ProcessScheduledPosts(ctx); err != nil {
			log.Printf("Scheduler Job: Error processing scheduled posts: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Scheduler Job: Failed to schedule post processor: %v", err)
	}

	// Runs daily at 2:00 AM to refresh Instagram long-lived tokens
	_, err = sc.s.Every(1).Day().At("02:00").Do(func() {
		ctx := context.Background()
		log.Println("Scheduler Job: Starting daily Instagram access token refresh...")
		if err := sc.authUsecase.RefreshExpiredTokens(ctx); err != nil {
			log.Printf("Scheduler Job: Error refreshing tokens: %v", err)
		} else {
			log.Println("Scheduler Job: Instagram access token refresh completed successfully")
		}
	})
	if err != nil {
		log.Printf("Scheduler Job: Failed to schedule token refresher: %v", err)
	}

	sc.s.StartAsync()
	log.Println("Scheduler Job: Background scheduler started successfully")
}

func (sc *Scheduler) Stop() {
	sc.s.Stop()
	log.Println("Scheduler Job: Background scheduler stopped")
}

