package repository

import "github.com/example/telegrambot/internal/model"

// SettingsRepository stores user settings in memory.
type SettingsRepository struct {
	settings model.Settings
}

func NewSettingsRepository() *SettingsRepository {
	return &SettingsRepository{}
}

func (r *SettingsRepository) Save(s model.Settings) {
	r.settings = s
}

func (r *SettingsRepository) Load() model.Settings {
	return r.settings
}
