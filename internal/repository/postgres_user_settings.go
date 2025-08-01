package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/ilinovom/summary-tasks-bot/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresUserSettingsRepository stores settings in a Postgres database.
type PostgresUserSettingsRepository struct {
	db *sql.DB
}

// NewPostgresUserSettingsRepository connects to Postgres and ensures the table exists.
func NewPostgresUserSettingsRepository(connStr string) (*PostgresUserSettingsRepository, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}
	r := &PostgresUserSettingsRepository{db: db}
	if err := r.init(); err != nil {
		db.Close()
		return nil, err
	}
	return r, nil
}

// init creates the user_settings table if it does not yet exist.
func (r *PostgresUserSettingsRepository) init() error {
	_, err := r.db.Exec(`
        CREATE TABLE IF NOT EXISTS user_settings (
            user_id BIGINT PRIMARY KEY,
            username TEXT,
            active BOOLEAN,
            info_types JSONB,
            categories JSONB,
            frequency INTEGER,
            tariff TEXT,
           last_scheduled_sent BIGINT,
           last_get_news_now BIGINT,
            get_news_now_count INTEGER,
            last_get_last_24h BIGINT,
            get_last_24h_count INTEGER
        )`)
	if err != nil {
		return err
	}
	if _, err = r.db.Exec(`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS username TEXT`); err != nil {
		return err
	}
	if _, err = r.db.Exec(`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS last_get_last_24h BIGINT`); err != nil {
		return err
	}
	_, err = r.db.Exec(`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS get_last_24h_count INTEGER`)
	return err
}

// Get retrieves a user's settings by ID.
func (r *PostgresUserSettingsRepository) Get(ctx context.Context, userID int64) (*model.UserSettings, error) {
	row := r.db.QueryRowContext(ctx, `SELECT user_id, username, active, info_types, categories, frequency, tariff, last_scheduled_sent, last_get_news_now, get_news_now_count, last_get_last_24h, get_last_24h_count FROM user_settings WHERE user_id=$1`, userID)
	var s model.UserSettings
	var topics, categories []byte
	if err := row.Scan(&s.UserID, &s.UserName, &s.Active, &topics, &categories, &s.Frequency, &s.Tariff, &s.LastScheduledSent, &s.LastGetNewsNow, &s.GetNewsNowCount, &s.LastGetLast24h, &s.GetLast24hCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	json.Unmarshal(topics, &s.Topics)
	return &s, nil
}

// Save inserts or updates a user's settings.
func (r *PostgresUserSettingsRepository) Save(ctx context.Context, settings *model.UserSettings) error {
	topics, err := json.Marshal(settings.Topics)
	if err != nil {
		return err
	}
	cats := []string{}
	for c := range settings.Topics {
		cats = append(cats, c)
	}
	categories, err := json.Marshal(cats)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
        INSERT INTO user_settings (user_id, username, active, info_types, categories, frequency, tariff, last_scheduled_sent, last_get_news_now, get_news_now_count, last_get_last_24h, get_last_24h_count)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
        ON CONFLICT (user_id) DO UPDATE SET
            username=EXCLUDED.username,
            active=EXCLUDED.active,
            info_types=EXCLUDED.info_types,
            categories=EXCLUDED.categories,
            frequency=EXCLUDED.frequency,
            tariff=EXCLUDED.tariff,
            last_scheduled_sent=EXCLUDED.last_scheduled_sent,
            last_get_news_now=EXCLUDED.last_get_news_now,
            get_news_now_count=EXCLUDED.get_news_now_count,
            last_get_last_24h=EXCLUDED.last_get_last_24h,
            get_last_24h_count=EXCLUDED.get_last_24h_count
   `, settings.UserID, settings.UserName, settings.Active, string(topics), string(categories), settings.Frequency, settings.Tariff, settings.LastScheduledSent, settings.LastGetNewsNow, settings.GetNewsNowCount, settings.LastGetLast24h, settings.GetLast24hCount)
	return err
}

// Delete removes settings for a user.
func (r *PostgresUserSettingsRepository) Delete(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_settings WHERE user_id=$1`, userID)
	return err
}

// List returns settings for all users.
func (r *PostgresUserSettingsRepository) List(ctx context.Context) ([]*model.UserSettings, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT user_id, username, active, info_types, categories, frequency, tariff, last_scheduled_sent, last_get_news_now, get_news_now_count, last_get_last_24h, get_last_24h_count FROM user_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*model.UserSettings
	for rows.Next() {
		var s model.UserSettings
		var topics, categories []byte
		if err := rows.Scan(&s.UserID, &s.UserName, &s.Active, &topics, &categories, &s.Frequency, &s.Tariff, &s.LastScheduledSent, &s.LastGetNewsNow, &s.GetNewsNowCount, &s.LastGetLast24h, &s.GetLast24hCount); err != nil {
			return nil, err
		}
		json.Unmarshal(topics, &s.Topics)
		result = append(result, &s)
	}
	return result, rows.Err()
}
