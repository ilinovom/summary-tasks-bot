package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/summary-tasks-bot/internal/model"
)

func TestFileUserSettingsRepository_CRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	repo, err := NewFileUserSettingsRepository(path)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()
	s := &model.UserSettings{UserID: 1, Categories: []string{"go"}, Active: true}
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserID != s.UserID || got.Categories[0] != "go" || !got.Active {
		t.Fatalf("unexpected data: %#v", got)
	}
	if err := repo.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, 1); !os.IsNotExist(err) {
		t.Fatalf("expected not exist error, got %v", err)
	}
}
