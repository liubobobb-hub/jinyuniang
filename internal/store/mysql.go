package store

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"whale-live/internal/textutil"
)

// MySQL is optional. The server keeps running on the file log when it is off
// or when the database cannot be reached.
type MySQL struct {
	db *sql.DB
}

const mysqlSchema = `CREATE TABLE IF NOT EXISTS whale_interactions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_name VARCHAR(64) NOT NULL DEFAULT '',
  source VARCHAR(32) NOT NULL DEFAULT '',
  danmaku TEXT NOT NULL,
  reply TEXT NOT NULL,
  mock TINYINT(1) NOT NULL DEFAULT 0,
  degraded TINYINT(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_whale_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

func OpenMySQL(dsn string) (*MySQL, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(2 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, mysqlSchema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &MySQL{db: db}, nil
}

func (m *MySQL) Save(ctx context.Context, r Record) error {
	_, err := m.db.ExecContext(ctx, `INSERT INTO whale_interactions
		(created_at, user_name, source, danmaku, reply, mock, degraded)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.Time,
		textutil.Clip(r.User, 64),
		textutil.Clip(r.Source, 32),
		r.Danmaku,
		r.Reply,
		boolTiny(r.Mock),
		boolTiny(r.Degraded),
	)
	return err
}

func (m *MySQL) Close() error { return m.db.Close() }

func boolTiny(v bool) int {
	if v {
		return 1
	}
	return 0
}
