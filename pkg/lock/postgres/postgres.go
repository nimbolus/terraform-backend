package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pgclient "github.com/nimbolus/terraform-backend/pkg/client/postgres"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "postgres"

type Lock struct {
	db    *sql.DB
	table string
}

func NewLock(table string) (*Lock, error) {
	db, err := pgclient.NewClient()
	if err != nil {
		return nil, err
	}

	l := &Lock{
		db:    db,
		table: table,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("checking locks table: %w", err)
	}

	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS ` + l.table + ` (
			state_id CHARACTER VARYING(255) PRIMARY KEY,
			lock_data BYTEA
		);`); err != nil {
		return nil, fmt.Errorf("creating locks table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing locks table: %w", err)
	}

	return l, nil
}

func (l *Lock) GetName() string {
	return Name
}

func (l *Lock) Lock(s *terraform.State) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var lock []byte

	if err := tx.QueryRow(`SELECT lock_data FROM `+l.table+` WHERE state_id = $1`, s.ID).Scan(&lock); err != nil {
		if err == sql.ErrNoRows {
			if _, err := tx.Exec(`INSERT INTO `+l.table+` (state_id, lock_data) VALUES ($1, $2)`, s.ID, s.Lock); err != nil {
				return false, err
			}

			if err := tx.Commit(); err != nil {
				return false, err
			}

			return true, nil
		}

		return false, err
	}

	if string(lock) == string(s.Lock) {
		// you already have the lock
		return true, nil
	}

	s.Lock = lock

	return false, nil
}

func (l *Lock) Unlock(s *terraform.State) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var lock []byte

	if err := tx.QueryRow(`SELECT lock_data FROM `+l.table+` WHERE state_id = $1`, s.ID).Scan(&lock); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	if string(lock) != string(s.Lock) {
		return false, nil
	}

	if _, err := tx.Exec(`DELETE FROM `+l.table+` WHERE state_id = $1 AND lock_data = $2`, s.ID, s.Lock); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

func (l *Lock) GetLock(s *terraform.State) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var lock []byte

	if err := l.db.QueryRowContext(ctx, `SELECT lock_data FROM `+l.table+` WHERE state_id = $1`, s.ID).Scan(&lock); err != nil {
		return nil, err
	}

	return lock, nil
}
