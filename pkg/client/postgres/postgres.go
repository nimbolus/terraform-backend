package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/nimbolus/terraform-backend/internal"
)

func NewClient() (*sql.DB, error) {
	connStr, err := internal.SecretEnvOrFile("postgres_connection", "postgres_connection_file")
	if err != nil {
		return nil, fmt.Errorf("getting postgres connection string: %w", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("initializing postgres client: %w", err)
	}

	return db, nil
}
