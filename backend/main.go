package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"tripledger/config"
	"tripledger/database"
	"tripledger/handlers"
	"tripledger/loaders"
	"tripledger/routes"
	"tripledger/services"
	"tripledger/writers"
)

func main() {
	cfg := config.LoadConfig()

	// 1. Initialize MySQL connection with splito_app user
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()
	log.Printf("Successfully connected to MySQL database: %s", cfg.DBName)

	if cfg.SMTPHost != "" && cfg.SMTPUser != "" {
		log.Printf("Email service configured: SMTP Host: %s, User: %s", cfg.SMTPHost, cfg.SMTPUser)
	} else {
		log.Printf("Email service in local fallback mode (SMTP not fully configured)")
	}

	// Local development mode
	gin.SetMode(gin.DebugMode)
	router := gin.Default()

	// Initialize Loaders & Writers
	userLoader := loaders.NewUserLoader(db)
	userWriter := writers.NewUserWriter(db)
	tripLoader := loaders.NewTripLoader(db)
	tripWriter := writers.NewTripWriter(db)
	expenseLoader := loaders.NewExpenseLoader(db)
	expenseWriter := writers.NewExpenseWriter(db)

	// Initialize Services
	authService := services.NewAuthService(db, cfg, userLoader, userWriter)
	tripService := services.NewTripService(db, cfg, tripLoader, tripWriter, userWriter, expenseLoader)
	ledgerService := services.NewLedgerService(db, cfg, tripLoader, expenseLoader, expenseWriter)

	// Initialize Handlers
	authHandler := handlers.NewAuthHandler(authService)
	tripHandler := handlers.NewTripHandler(tripService, userLoader)
	ledgerHandler := handlers.NewLedgerHandler(ledgerService)
	locationHandler := handlers.NewLocationHandler(services.NewPlaceService())

	// Register API Routes
	routes.RegisterRoutes(router, cfg, authHandler, tripHandler, ledgerHandler, locationHandler)

	// Health check & root endpoints for Render deployment verification (supports GET & HEAD on / and /health)
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"app":      "Splito API",
			"database": cfg.DBName,
		})
	}
	router.GET("/health", healthHandler)
	router.HEAD("/health", healthHandler)
	router.GET("/", healthHandler)
	router.HEAD("/", healthHandler)

	// Serve Frontend Static Build if present
	frontendDist := filepath.Join("..", "frontend", "dist")
	if _, err := os.Stat(frontendDist); err == nil {
		router.Static("/assets", filepath.Join(frontendDist, "assets"))
		router.NoRoute(func(c *gin.Context) {
			// If request is for an existing static asset (e.g. manifest.webmanifest, sw.js, pwa-*.png, favicon.svg)
			cleanPath := filepath.Clean(c.Request.URL.Path)
			targetFile := filepath.Join(frontendDist, cleanPath)
			if stat, err := os.Stat(targetFile); err == nil && !stat.IsDir() {
				c.File(targetFile)
				return
			}
			c.File(filepath.Join(frontendDist, "index.html"))
		})
		log.Printf("Serving frontend from: %s", frontendDist)
	}

	log.Printf("Splito dev server running on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
