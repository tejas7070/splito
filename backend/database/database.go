package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"tripledger/config"
)

var (
	DB    *sql.DB
	Redis *redis.Client
)

func InitDB(cfg *config.Config) (*sql.DB, error) {
	var dsn string
	if cfg.DBDSN != "" {
		dsn = cfg.DBDSN
	} else {
		tlsParam := ""
		if cfg.DBTLS {
			tlsParam = "&tls=true"
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local%s",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, tlsParam)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := ensureTripLocationSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ensure trip location schema: %w", err)
	}

	DB = db
	ensureSchemaMigrations(db)
	return db, nil
}

func ensureTripLocationSchema(db *sql.DB) error {
	columns := []struct {
		name       string
		definition string
	}{
		{"start_latitude", "DECIMAL(10,7) NULL"},
		{"start_longitude", "DECIMAL(10,7) NULL"},
		{"destination_latitude", "DECIMAL(10,7) NULL"},
		{"destination_longitude", "DECIMAL(10,7) NULL"},
	}
	for _, column := range columns {
		var exists int
		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'trips' AND COLUMN_NAME = ?
		`, column.name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check trips.%s: %w", column.name, err)
		}
		if exists == 0 {
			if _, alterErr := db.Exec("ALTER TABLE trips ADD COLUMN " + column.name + " " + column.definition); alterErr != nil {
				checkErr := db.QueryRow(`
					SELECT COUNT(*)
					FROM information_schema.COLUMNS
					WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'trips' AND COLUMN_NAME = ?
				`, column.name).Scan(&exists)
				if checkErr != nil {
					return fmt.Errorf("add trips.%s: %w (could not verify concurrent migration: %v)", column.name, alterErr, checkErr)
				}
				if exists == 0 {
					return fmt.Errorf("add trips.%s: %w", column.name, alterErr)
				}
			}
		}
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS trip_stops (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			trip_id BIGINT UNSIGNED NOT NULL,
			position INT NOT NULL,
			name VARCHAR(255) NOT NULL,
			latitude DECIMAL(10,7) NOT NULL,
			longitude DECIMAL(10,7) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_trip_stop_position (trip_id, position),
			CONSTRAINT fk_trip_stops_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)
	if err != nil {
		return fmt.Errorf("create trip_stops table: %w", err)
	}
	return nil
}

func ensureSchemaMigrations(db *sql.DB) {
	// Support shareable invite_code in trips table
	var inviteColExists int
	_ = db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'trips' AND COLUMN_NAME = 'invite_code'`).Scan(&inviteColExists)
	if inviteColExists == 0 {
		_, _ = db.Exec(`ALTER TABLE trips ADD COLUMN invite_code VARCHAR(64) NULL;`)
		_, _ = db.Exec(`CREATE UNIQUE INDEX idx_trips_invite_code ON trips (invite_code);`)
	}
	_, _ = db.Exec(`UPDATE trips SET invite_code = CONCAT('splito_', SUBSTRING(MD5(CONCAT(id, NOW(), RAND())), 1, 12)) WHERE invite_code IS NULL OR invite_code = '';`)

	// Support 'requested' status in trip_members for invite link join requests
	_, _ = db.Exec(`ALTER TABLE trip_members MODIFY COLUMN status ENUM('invited', 'active', 'rejected', 'left', 'removed', 'requested') NOT NULL DEFAULT 'invited';`)
	// Support 'archived' status in trips table
	_, _ = db.Exec(`ALTER TABLE trips MODIFY COLUMN status ENUM('planned', 'active', 'completed', 'cancelled', 'archived') NOT NULL DEFAULT 'planned';`)

	// Add client_uuid column to expenses if not exists
	_, _ = db.Exec(`ALTER TABLE expenses ADD COLUMN client_uuid VARCHAR(64) NULL;`)
	_, _ = db.Exec(`CREATE UNIQUE INDEX idx_expenses_client_uuid ON expenses (client_uuid);`)

	// Add client_uuid column to expense_claims if not exists
	_, _ = db.Exec(`ALTER TABLE expense_claims ADD COLUMN client_uuid VARCHAR(64) NULL;`)
	_, _ = db.Exec(`CREATE UNIQUE INDEX idx_claims_client_uuid ON expense_claims (client_uuid);`)

	// Add client_uuid column to contributions if not exists
	_, _ = db.Exec(`ALTER TABLE contributions ADD COLUMN client_uuid VARCHAR(64) NULL;`)
	_, _ = db.Exec(`CREATE UNIQUE INDEX idx_contributions_client_uuid ON contributions (client_uuid);`)

	// Create custom_parameters table for dynamic security controls & system timers
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS custom_parameters (
			param_key VARCHAR(100) PRIMARY KEY,
			param_value VARCHAR(255) NOT NULL,
			description VARCHAR(255) NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)

	// Seed default security parameters if not present
	defaultParams := map[string]struct{ val, desc string }{
		"otp_cooldown_seconds":    {"30", "Cooldown between consecutive OTP requests (seconds)"},
		"otp_max_attempts":        {"5", "Max OTP requests allowed within the window"},
		"otp_window_minutes":      {"15", "Window size for tracking OTP requests (minutes)"},
		"otp_lockout_minutes":     {"15", "Lockout duration when max OTP requests are exceeded (minutes)"},
		"otp_max_verify_attempts": {"5", "Max incorrect OTP verification attempts before invalidating OTP"},
		"otp_expiry_minutes":      {"10", "OTP lifetime (minutes)"},
		"login_max_failures":      {"5", "Failures per email and IP in the window"},
		"login_lockout_minutes":   {"15", "Window and lockout length for failed logins"},
	}

	for k, v := range defaultParams {
		_, _ = db.Exec(`
			INSERT IGNORE INTO custom_parameters (param_key, param_value, description)
			VALUES (?, ?, ?)
		`, k, v.val, v.desc)
	}
}

func InitRedis(cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	Redis = rdb
	return rdb
}
