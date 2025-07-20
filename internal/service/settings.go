package service

import (
	"log"

	"github.com/example/telegrambot/internal/model"
	"github.com/example/telegrambot/internal/repository"
)

// SettingsService manages user preferences.
type SettingsService struct {
	repo *repository.SettingsRepository
}

func NewSettingsService(repo *repository.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

// Start initiates configuration flow.
func (s *SettingsService) Start() {
	log.Println("select info type: insight, news, idea, fact")
}

// SetInfoType stores desired info type.
func (s *SettingsService) SetInfoType(t string) {
	cfg := s.repo.Load()
	cfg.InfoType = t
	s.repo.Save(cfg)
	log.Printf("info type set to %s", t)
}

// AddTopic appends a topic without removing existing ones.
func (s *SettingsService) AddTopic(topic string) {
	cfg := s.repo.Load()
	for _, t := range cfg.Topics {
		if t == topic {
			s.repo.Save(cfg)
			return
		}
	}
	cfg.Topics = append(cfg.Topics, topic)
	s.repo.Save(cfg)
	log.Printf("topic %s added", topic)
}

// GetSettings returns current settings.
func (s *SettingsService) GetSettings() model.Settings {
	return s.repo.Load()
}

// DeleteTopic removes a single topic or all topics when topic is "all".
func (s *SettingsService) DeleteTopic(topic string) {
	cfg := s.repo.Load()
	if topic == "all" || topic == "" {
		cfg.Topics = nil
		s.repo.Save(cfg)
		log.Println("all topics deleted")
		return
	}
	topics := []string{}
	for _, t := range cfg.Topics {
		if t != topic {
			topics = append(topics, t)
		}
	}
	cfg.Topics = topics
	s.repo.Save(cfg)
	log.Printf("topic %s deleted", topic)
}
