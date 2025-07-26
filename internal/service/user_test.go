package service

import (
	"context"
	"os"
	"testing"

	"github.com/ilinovom/summary-tasks-bot/internal/config"
	"github.com/ilinovom/summary-tasks-bot/internal/model"
	"github.com/ilinovom/summary-tasks-bot/internal/repository"
)

type memRepo struct {
	data map[int64]*model.UserSettings
}

var _ repository.UserSettingsRepository = (*memRepo)(nil)

func newMemRepo() *memRepo {
	return &memRepo{data: map[int64]*model.UserSettings{}}
}

func (m *memRepo) Get(ctx context.Context, userID int64) (*model.UserSettings, error) {
	if s, ok := m.data[userID]; ok {
		copy := *s
		return &copy, nil
	}
	return nil, os.ErrNotExist
}

func (m *memRepo) Save(ctx context.Context, settings *model.UserSettings) error {
	c := *settings
	m.data[settings.UserID] = &c
	return nil
}

func (m *memRepo) Delete(ctx context.Context, userID int64) error {
	delete(m.data, userID)
	return nil
}

func (m *memRepo) List(ctx context.Context) ([]*model.UserSettings, error) {
	out := []*model.UserSettings{}
	for _, s := range m.data {
		c := *s
		out = append(out, &c)
	}
	return out, nil
}

func TestUserService_StartStop(t *testing.T) {
	repo := newMemRepo()
	svc := NewUserService(repo, nil, nil)
	ctx := context.Background()

	if err := svc.Start(ctx, 1, "testuser"); err != nil {
		t.Fatalf("start: %v", err)
	}
	u, _ := repo.Get(ctx, 1)
	if !u.Active {
		t.Fatalf("expected active")
	}
	if err := svc.Stop(ctx, 1); err != nil {
		t.Fatalf("stop: %v", err)
	}
	u, _ = repo.Get(ctx, 1)
	if u.Active {
		t.Fatalf("expected inactive")
	}
}

func TestUserService_GetByUsername_SetTariff(t *testing.T) {
	repo := newMemRepo()
	svc := NewUserService(repo, nil, map[string]config.Tariff{"base": {}, "plus": {}})
	ctx := context.Background()

	repo.Save(ctx, &model.UserSettings{UserID: 1, UserName: "user1"})
	u, err := svc.GetByUsername(ctx, "user1")
	if err != nil || u.UserID != 1 {
		t.Fatalf("get by username failed: %v", err)
	}
	if err := svc.SetTariff(ctx, 1, "plus"); err != nil {
		t.Fatalf("set tariff: %v", err)
	}
	u2, _ := repo.Get(ctx, 1)
	if u2.Tariff != "plus" {
		t.Fatalf("tariff not updated")
	}
}
