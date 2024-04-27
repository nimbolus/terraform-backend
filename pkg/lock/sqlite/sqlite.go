package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "sqlite"

var _ lock.Locker = (*Lock)(nil)

type Lock struct {
	db *sql.DB
}

const (
	createTableStatement = `CREATE TABLE IF NOT EXISTS locks (
		state_id VARCHAR(255) PRIMARY KEY,
		lock_data BLOB
	);`
	selectLockStatement = `SELECT lock_data FROM locks WHERE state_id = $1`
	insertLockStatement = `INSERT INTO locks (state_id, lock_data) VALUES ($1, $2)`
	deleteLockStatement = `DELETE FROM locks WHERE state_id = $1 AND lock_data = $2`
)

func NewLock(location string) (*Lock, error) {
	db, err := sql.Open("sqlite3", location)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, createTableStatement); err != nil {
		return nil, fmt.Errorf("failed to create DB table: %w", err)
	}

	return &Lock{
		db: db,
	}, nil
}

func (l *Lock) GetName() string {
	return Name
}

func (l *Lock) Lock(s *terraform.State) (ok bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}

	defer tx.Rollback() // nolint: errcheck

	var rawLock []byte

	if err := tx.QueryRow(selectLockStatement, s.ID).Scan(&rawLock); err != nil {
		if err == sql.ErrNoRows {
			lockBytes, err := json.Marshal(s.Lock)
			if err != nil {
				return false, err
			}

			if _, err := tx.Exec(insertLockStatement, s.ID, lockBytes); err != nil {
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

func (l *Lock) Unlock(s *terraform.State) (ok bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}

	defer tx.Rollback() // nolint: errcheck

	var rawLock []byte

	if err := tx.QueryRow(selectLockStatement, s.ID).Scan(&rawLock); err != nil {
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

	if _, err := tx.Exec(deleteLockStatement, s.ID, rawLock); err != nil {
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

	if err := l.db.QueryRowContext(ctx, selectLockStatement, s.ID).Scan(&rawLock); err != nil {
		return terraform.LockInfo{}, err
	}

	var lock terraform.LockInfo

	if err := json.Unmarshal(rawLock, &lock); err != nil {
		return terraform.LockInfo{}, err
	}

	return lock, nil
}
