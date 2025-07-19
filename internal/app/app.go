package app

import "log"

// App coordinates the services and repositories.
type App struct{}

// New creates a new App instance.
func New() *App {
	return &App{}
}

// Run starts the bot (placeholder implementation).
func (a *App) Run() {
	log.Println("bot started")
}
