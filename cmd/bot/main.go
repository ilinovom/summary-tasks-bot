package main

import (
	"context"
	"log"

	"github.com/example/summary-tasks-bot/internal/app"
	"github.com/example/summary-tasks-bot/internal/config"
	"github.com/example/summary-tasks-bot/internal/repository"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	repo, err := repository.NewFileUserSettingsRepository(cfg.SettingsPath)
	if err != nil {
		log.Fatal(err)
	}

	application := app.New(cfg, repo)
	if err := application.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
