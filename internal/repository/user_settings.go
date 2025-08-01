package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/ilinovom/summary-tasks-bot/internal/model"
)

// UserSettingsRepository abstracts persistence of user settings.
type UserSettingsRepository interface {
	Get(ctx context.Context, userID int64) (*model.UserSettings, error)
	Save(ctx context.Context, settings *model.UserSettings) error
	Delete(ctx context.Context, userID int64) error
	List(ctx context.Context) ([]*model.UserSettings, error)
}

// FileUserSettingsRepository stores settings in a JSON file.
type FileUserSettingsRepository struct {
	path string
	mu   sync.Mutex
	data map[int64]*model.UserSettings
}

// NewFileUserSettingsRepository loads settings from the given JSON file or creates it if missing.
func NewFileUserSettingsRepository(path string) (*FileUserSettingsRepository, error) {
	r := &FileUserSettingsRepository{path: path, data: map[int64]*model.UserSettings{}}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

// load reads the JSON file into memory.
func (r *FileUserSettingsRepository) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	file, err := os.Open(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			r.data = map[int64]*model.UserSettings{}
			return nil
		}
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&r.data)
}

// saveLocked writes the in-memory data back to disk.
func (r *FileUserSettingsRepository) saveLocked() error {
	file, err := os.Create(r.path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(r.data)
}

// Get retrieves settings for the user or returns os.ErrNotExist.
func (r *FileUserSettingsRepository) Get(ctx context.Context, userID int64) (*model.UserSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.data[userID]; ok {
		copy := *s
		return &copy, nil
	}
	return nil, os.ErrNotExist
}

// Save persists new settings for a user.
func (r *FileUserSettingsRepository) Save(ctx context.Context, settings *model.UserSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *settings
	r.data[settings.UserID] = &copy
	return r.saveLocked()
}

// Delete removes settings for a user.
func (r *FileUserSettingsRepository) Delete(ctx context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, userID)
	return r.saveLocked()
}

// List returns all stored user settings.
func (r *FileUserSettingsRepository) List(ctx context.Context) ([]*model.UserSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]*model.UserSettings, 0, len(r.data))
	for _, s := range r.data {
		copy := *s
		res = append(res, &copy)
	}
	return res, nil
}
