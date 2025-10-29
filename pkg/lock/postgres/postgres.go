package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "postgres"

type Lock struct {
	db    *sql.DB
	table string
}

func NewLock(db *sql.DB, table string) (*Lock, error) {
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

	defer tx.Rollback() // nolint: errcheck

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

	defer tx.Rollback() // nolint: errcheck

	var rawLock []byte

	if err := tx.QueryRow(`SELECT lock_data FROM `+l.table+` WHERE state_id = $1`, s.ID).Scan(&rawLock); err != nil {
		if err == sql.ErrNoRows {
			lockBytes, err := json.Marshal(s.Lock)
			if err != nil {
				return false, err
			}

			if _, err := tx.Exec(`INSERT INTO `+l.table+` (state_id, lock_data) VALUES ($1, $2)`, s.ID, lockBytes); err != nil {
				return false, err
			}

			if err := tx.Commit(); err != nil {
				return false, err
			}

			return true, nil
		}

		return false, err
	}

	var lock terraform.LockInfo

	if err := json.Unmarshal(rawLock, &lock); err != nil {
		return false, err
	}

	if lock.Equal(s.Lock) {
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

	defer tx.Rollback() // nolint: errcheck

	var rawLock []byte

	if err := tx.QueryRow(`SELECT lock_data FROM `+l.table+` WHERE state_id = $1`, s.ID).Scan(&rawLock); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	var lock terraform.LockInfo

	if err := json.Unmarshal(rawLock, &lock); err != nil {
		return false, err
	}

	if !lock.Equal(s.Lock) {
		return false, nil
	}

	if _, err := tx.Exec(`DELETE FROM `+l.table+` WHERE state_id = $1 AND lock_data = $2`, s.ID, rawLock); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

func (l *Lock) GetLock(s *terraform.State) (terraform.LockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var rawLock []byte

	if err := l.db.QueryRowContext(ctx, `SELECT lock_data FROM `+l.table+` WHERE state_id = $1`, s.ID).Scan(&rawLock); err != nil {
		return terraform.LockInfo{}, err
	}

	var lock terraform.LockInfo

	if err := json.Unmarshal(rawLock, &lock); err != nil {
		return terraform.LockInfo{}, err
	}

	return lock, nil
}
