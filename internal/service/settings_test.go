package service

import (
	"testing"

	"github.com/example/telegrambot/internal/repository"
)

func TestSettingsService_AddAndDeleteTopics(t *testing.T) {
	repo := repository.NewSettingsRepository()
	s := NewSettingsService(repo)

	s.AddTopic("health")
	s.AddTopic("sport")
	cfg := s.GetSettings()
	if len(cfg.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(cfg.Topics))
	}

	s.DeleteTopic("health")
	cfg = s.GetSettings()
	if len(cfg.Topics) != 1 || cfg.Topics[0] != "sport" {
		t.Fatalf("topic removal failed: %+v", cfg.Topics)
	}

	s.DeleteTopic("all")
	cfg = s.GetSettings()
	if len(cfg.Topics) != 0 {
		t.Fatalf("expected all topics removed")
	}
}
