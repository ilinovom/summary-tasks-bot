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

// init creates necessary tables if they do not yet exist.
func (r *PostgresUserSettingsRepository) init() error {
	if _, err := r.db.Exec(`
        CREATE TABLE IF NOT EXISTS user_settings (
            user_id BIGINT PRIMARY KEY,
            username TEXT,
            active BOOLEAN,
            info_types JSONB,
            tariff TEXT
        )`); err != nil {
		return err
	}
	if _, err := r.db.Exec(`
        CREATE TABLE IF NOT EXISTS message_schedule (
            user_id BIGINT PRIMARY KEY REFERENCES user_settings(user_id),
            frequency INTEGER,
            last_scheduled_sent BIGINT,
            next_category_index INTEGER
        )`); err != nil {
		return err
	}
	_, err := r.db.Exec(`
        CREATE TABLE IF NOT EXISTS user_commands (
            user_id BIGINT PRIMARY KEY REFERENCES user_settings(user_id),
            last_get_news_now BIGINT,
            get_news_now_count INTEGER,
            last_get_last_24h BIGINT,
            get_last_24h_count INTEGER
        )`)
	return err
}

// Get retrieves a user's settings by ID.
func (r *PostgresUserSettingsRepository) Get(ctx context.Context, userID int64) (*model.UserSettings, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT u.user_id, u.username, u.active, u.info_types, u.categories,
               s.frequency, u.tariff, s.last_scheduled_sent,
               c.last_get_news_now, c.get_news_now_count,
               c.last_get_last_24h, c.get_last_24h_count,
               s.next_category_index
        FROM user_settings u
        LEFT JOIN message_schedule s ON u.user_id = s.user_id
        LEFT JOIN user_commands c ON u.user_id = c.user_id
        WHERE u.user_id=$1`, userID)
	var s model.UserSettings
	var topics []byte
	if err := row.Scan(&s.UserID, &s.UserName, &s.Active, &topics, &s.Frequency, &s.Tariff, &s.LastScheduledSent, &s.LastGetNewsNow, &s.GetNewsNowCount, &s.LastGetLast24h, &s.GetLast24hCount, &s.NextCategoryIndex); err != nil {
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

	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `
        INSERT INTO user_settings (user_id, username, active, info_types, tariff)
        VALUES ($1,$2,$3,$4,$5,$6)
        ON CONFLICT (user_id) DO UPDATE SET
            username=EXCLUDED.username,
            active=EXCLUDED.active,
            info_types=EXCLUDED.info_types,
            tariff=EXCLUDED.tariff
        `, settings.UserID, settings.UserName, settings.Active, string(topics), settings.Tariff); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `
        INSERT INTO message_schedule (user_id, frequency, last_scheduled_sent, next_category_index)
        VALUES ($1,$2,$3,$4)
        ON CONFLICT (user_id) DO UPDATE SET
            frequency=EXCLUDED.frequency,
            last_scheduled_sent=EXCLUDED.last_scheduled_sent,
            next_category_index=EXCLUDED.next_category_index
        `, settings.UserID, settings.Frequency, settings.LastScheduledSent, settings.NextCategoryIndex); err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
        INSERT INTO user_commands (user_id, last_get_news_now, get_news_now_count, last_get_last_24h, get_last_24h_count)
        VALUES ($1,$2,$3,$4,$5)
        ON CONFLICT (user_id) DO UPDATE SET
            last_get_news_now=EXCLUDED.last_get_news_now,
            get_news_now_count=EXCLUDED.get_news_now_count,
            last_get_last_24h=EXCLUDED.last_get_last_24h,
            get_last_24h_count=EXCLUDED.get_last_24h_count
        `, settings.UserID, settings.LastGetNewsNow, settings.GetNewsNowCount, settings.LastGetLast24h, settings.GetLast24hCount)
	return err
}

// Delete removes settings for a user.
func (r *PostgresUserSettingsRepository) Delete(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM message_schedule WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM user_commands WHERE user_id=$1`, userID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_settings WHERE user_id=$1`, userID)
	return err
}

// List returns settings for all users.
func (r *PostgresUserSettingsRepository) List(ctx context.Context) ([]*model.UserSettings, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT u.user_id, u.username, u.active, u.info_types, u.categories,
               s.frequency, u.tariff, s.last_scheduled_sent,
               c.last_get_news_now, c.get_news_now_count,
               c.last_get_last_24h, c.get_last_24h_count,
               s.next_category_index
        FROM user_settings u
        LEFT JOIN message_schedule s ON u.user_id = s.user_id
        LEFT JOIN user_commands c ON u.user_id = c.user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*model.UserSettings
	for rows.Next() {
		var s model.UserSettings
		var topics []byte
		if err := rows.Scan(&s.UserID, &s.UserName, &s.Active, &topics, &s.Frequency, &s.Tariff, &s.LastScheduledSent, &s.LastGetNewsNow, &s.GetNewsNowCount, &s.LastGetLast24h, &s.GetLast24hCount, &s.NextCategoryIndex); err != nil {
			return nil, err
		}
		json.Unmarshal(topics, &s.Topics)
		result = append(result, &s)
	}
	return result, rows.Err()
}
