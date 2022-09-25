package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pgclient "github.com/nimbolus/terraform-backend/pkg/client/postgres"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

type Lock struct {
	db *pgclient.Client
}

func NewLock() (*Lock, error) {
	db, err := pgclient.NewClient()
	if err != nil {
		return nil, err
	}

	return &Lock{
		db: db,
	}, nil
}

func (l *Lock) GetName() string {
	return "pg"
}

func (l *Lock) Lock(s *terraform.State) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var lockData []byte

	if err := tx.QueryRow(`SELECT lock_data FROM `+l.db.GetLocksTableName()+` WHERE state_id = $1`, s.ID).Scan(&lockData); err != nil {
		if err == sql.ErrNoRows {
			if _, err := tx.Exec(`INSERT INTO locks (state_id, lock_data) VALUES ($1, $2)`, s.ID, s.Lock); err != nil {
				return false, err
			}

			if err := tx.Commit(); err != nil {
				return false, err
			}

			return true, nil
		}

		return false, err
	}

	if string(lockData) == string(s.Lock) {
		// you already have the lock
		return true, nil
	}

	return false, fmt.Errorf("lock already taken for id %s: %s", s.ID, string(lockData))
}

func (l *Lock) Unlock(s *terraform.State) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var lockData []byte

	if err := tx.QueryRow(`SELECT lock_data FROM `+l.db.GetLocksTableName()+` WHERE state_id = $1`, s.ID).Scan(&lockData); err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("no lock for id %s found", s.ID)
		}

		return false, err
	}

	if string(lockData) != string(s.Lock) {
		return false, fmt.Errorf("lock mismatch for id %s", s.ID)
	}

	if _, err := tx.Exec(`DELETE FROM `+l.db.GetLocksTableName()+` WHERE state_id = $1 AND lock_data = $2`, s.ID, s.Lock); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}
