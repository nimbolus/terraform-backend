package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

func NewClient() (*sql.DB, error) {
	viper.SetDefault("postgres_connection", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")

	db, err := sql.Open("postgres", viper.GetString("postgres_connection"))
	if err != nil {
		return nil, fmt.Errorf("initializing postgres client: %w", err)
	}

	return db, nil
}
