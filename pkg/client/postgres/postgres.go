package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

type Client struct {
	*sql.DB
	locksTableName string
}

func NewClient() (*Client, error) {
	viper.SetDefault("postgres_connection", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	viper.SetDefault("postgres_locks_table", "locks")

	client := &Client{
		locksTableName: viper.GetString("postgres_locks_table"),
	}

	db, err := sql.Open("postgres", viper.GetString("postgres_connection"))
	if err != nil {
		return nil, fmt.Errorf("initializing postgres client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("checking locks table: %w", err)
	}

	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS ` + client.locksTableName + ` (
			state_id CHARACTER VARYING(255) PRIMARY KEY,
			lock_data BYTEA
		);`); err != nil {
		return nil, fmt.Errorf("creating locks table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing locks table: %w", err)
	}

	client.DB = db

	return client, nil
}

func (c Client) GetLocksTableName() string {
	return c.locksTableName
}
