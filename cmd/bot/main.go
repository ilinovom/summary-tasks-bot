package main

import (
	"context"
	"log"

	"github.com/ilinovom/summary-tasks-bot/internal/app"
	"github.com/ilinovom/summary-tasks-bot/internal/config"
	"github.com/ilinovom/summary-tasks-bot/internal/repository"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	repo, err := repository.NewPostgresUserSettingsRepository(cfg.DBConnString)
	if err != nil {
		log.Fatal(err)
	}

	application := app.New(cfg, repo)
	log.Println("bot running")
	if err := application.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
