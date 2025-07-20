package app

import (
	"log"

	"github.com/example/telegrambot/internal/repository"
	"github.com/example/telegrambot/internal/service"
)

// App coordinates the services and repositories.
type App struct {
	settings *service.SettingsService
}

// New creates a new App instance.
func New() *App {
	repo := repository.NewSettingsRepository()
	settings := service.NewSettingsService(repo)
	return &App{settings: settings}
}

// Run starts the bot and triggers the initial setup flow.
func (a *App) Run() {
	log.Println("bot started")
	a.settings.Start()
}

// ShowConfig logs current user settings.
func (a *App) ShowConfig() {
	cfg := a.settings.GetSettings()
	log.Printf("current settings: info_type=%s topics=%v", cfg.InfoType, cfg.Topics)
}

// RemoveTopic removes the specified topic or all topics.
func (a *App) RemoveTopic(topic string) {
	a.settings.DeleteTopic(topic)
}
