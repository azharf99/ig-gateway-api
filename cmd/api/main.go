package main

import (
	"log"

	"github.com/azharf99/ig-gateway-api/internal/config"
	"github.com/azharf99/ig-gateway-api/internal/delivery"
	"github.com/azharf99/ig-gateway-api/internal/delivery/http/handlers"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/database/models"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/database/postgres"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/instagram"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/scheduler"
	"github.com/azharf99/ig-gateway-api/internal/infrastructure/storage"
	"github.com/azharf99/ig-gateway-api/internal/usecase/auth"
	"github.com/azharf99/ig-gateway-api/internal/usecase/post"
)

func main() {
	log.Println("Starting Instagram Gateway API...")

	// 1. Load Configurations
	config.LoadConfig()

	// 2. Initialize Database & Redis
	config.InitDatabase()
	config.InitRedis()

	// 3. Run GORM Auto Migrations
	log.Println("Running database migrations...")
	err := config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.PostMedia{})
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Database migration completed successfully")

	// 4. Initialize Services (Infrastructure Layer)
	igClient := instagram.NewInstagramClient()
	storageService := storage.NewLocalStorage("./uploads")

	// 5. Initialize Repositories (Data Layer)
	userRepo := postgres.NewUserRepository(config.DB)
	postRepo := postgres.NewPostRepository(config.DB)

	// 6. Initialize Usecases (Usecase Layer)
	authUC := auth.NewAuthUsecase(userRepo, igClient)
	postUC := post.NewPostUsecase(postRepo, userRepo, igClient, storageService)

	// 7. Initialize Cron Scheduler
	cronScheduler := scheduler.NewScheduler(postUC)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	// 8. Initialize HTTP Handlers (Delivery Layer)
	authHandler := handlers.NewAuthHandler(authUC)
	postHandler := handlers.NewPostHandler(postUC, storageService)

	// 9. Setup Router and Start Server
	router := delivery.SetupRouter(authHandler, postHandler)
	server := delivery.NewServer(router)

	server.Run()
}
