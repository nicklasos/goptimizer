package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"app/config"
	"app/internal/db"
	"app/internal/logger"
	"app/internal/scheduler"
)

func main() {
	var (
		useTestDB = flag.Bool("test-db", false, "Use TEST_DATABASE_PATH instead of DATABASE_PATH")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if *useTestDB {
		if err := cfg.UseTestDatabase(); err != nil {
			log.Fatal(err)
		}
		log.Println("Using TEST_DATABASE_PATH for the database")
	}

	appLogger, err := logger.New(logger.Config{
		Level:     cfg.LogLevel,
		Format:    cfg.LogFormat,
		Output:    cfg.LogOutput,
		AddSource: cfg.Debug,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	appLogger.Info("Starting Gogo Cron Server")

	database, err := db.NewConnection(cfg)
	if err != nil {
		appLogger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	appLogger.Info("Database connection established")

	queries := db.New(database)
	tx := db.NewTxRunner(database)

	deps := &scheduler.Dependencies{
		Config:  cfg,
		DB:      database,
		Queries: queries,
		Tx:      tx,
		Logger:  appLogger,
	}

	cronScheduler := scheduler.NewScheduler(deps)

	if err := cronScheduler.RegisterJobs(); err != nil {
		appLogger.Error("Failed to register cron jobs", "error", err)
		os.Exit(1)
	}

	if err := cronScheduler.Start(); err != nil {
		appLogger.Error("Failed to start the scheduler", "error", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	appLogger.Info("Cron server is running. Press Ctrl+C to exit.")

	<-quit
	appLogger.Info("Shutdown signal received, stopping scheduler...")

	cronScheduler.Stop()
	appLogger.Info("Gogo Cron Server stopped successfully")
}
