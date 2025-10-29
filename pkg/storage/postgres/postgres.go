package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "postgres"

type PostgresStorage struct {
	db    *sql.DB
	table string
}

func NewPostgresStorage(db *sql.DB, table string) (*PostgresStorage, error) {
	p := &PostgresStorage{
		db:    db,
		table: table,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("checking states table: %w", err)
	}

	defer tx.Rollback() // nolint: errcheck

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS ` + p.table + ` (
			state_id CHARACTER VARYING(255) PRIMARY KEY,
			state_data BYTEA
		);`); err != nil {
		return nil, fmt.Errorf("creating states table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing states table: %w", err)
	}

	return p, nil
}

func (p *PostgresStorage) GetName() string {
	return Name
}

func (p *PostgresStorage) SaveState(s *terraform.State) error {
	if _, err := p.db.Exec(`INSERT INTO `+p.table+` (state_id, state_data) VALUES ($1, $2)
		ON CONFLICT (state_id) DO UPDATE SET state_data = EXCLUDED.state_data`, s.ID, s.Data); err != nil {
		return err
	}

	return nil
}

func (p *PostgresStorage) GetState(id string) (*terraform.State, error) {
	s := &terraform.State{}

	err := p.db.QueryRow(`SELECT state_data FROM `+p.table+` WHERE state_id = $1`, id).Scan(&s.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrStateNotFound
	} else if err != nil {
		return nil, err
	}

	return s, nil
}

func (p *PostgresStorage) DeleteState(id string) error {
	return p.db.QueryRow(`DELETE FROM `+p.table+` WHERE state_id = $1`, id).Err()
}
