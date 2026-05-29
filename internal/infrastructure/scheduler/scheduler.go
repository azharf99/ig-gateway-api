package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/usecase/post"
	"github.com/go-co-op/gocron"
)

type Scheduler struct {
	s           *gocron.Scheduler
	postUsecase post.Usecase
}

func NewScheduler(postUsecase post.Usecase) *Scheduler {
	s := gocron.NewScheduler(time.Local)
	return &Scheduler{
		s:           s,
		postUsecase: postUsecase,
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

	sc.s.StartAsync()
	log.Println("Scheduler Job: Background scheduler started successfully")
}

func (sc *Scheduler) Stop() {
	sc.s.Stop()
	log.Println("Scheduler Job: Background scheduler stopped")
}
