package redis

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis"
	rsredigo "github.com/go-redsync/redsync/v4/redis/redigo"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	redisclient "github.com/nimbolus/terraform-backend/pkg/client/redis"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const (
	Name    = "redis"
	lockKey = "terraform-backend-state-lock"
)

type Lock struct {
	pool   *redigo.Pool
	rsPool redis.Pool
	client *redsync.Redsync
}

func NewLock() *Lock {
	pool := redisclient.NewPool()
	rsPool := rsredigo.NewPool(pool)

	return &Lock{
		pool:   pool,
		rsPool: rsPool,
		client: redsync.New(rsPool),
	}
}

func (r *Lock) GetName() string {
	return Name
}

func (r *Lock) Lock(s *terraform.State) (locked bool, err error) {
	mutex := r.client.NewMutex(lockKey, redsync.WithExpiry(12*time.Hour), redsync.WithTries(1), redsync.WithGenValueFunc(func() (string, error) {
		return uuid.New().String(), nil
	}))

	// lock the global redis mutex
	if err := mutex.Lock(); err != nil {
		log.Errorf("failed to lock redsync mutex: %v", err)

		return false, err
	}

	defer func() {
		// unlock the global redis mutex
		if _, mutErr := mutex.Unlock(); mutErr != nil {
			log.Errorf("failed to unlock redsync mutex: %v", mutErr)

			if err != nil {
				err = multierr.Append(err, mutErr)
			}
		}
	}()

	// check if the state is already locked
	lock, err := r.getLock(s)
	if err != nil {
		if !errors.Is(err, redigo.ErrNil) {
			return false, err
		}

		// state is not locked
		// set the lock for the state
		if err := r.setLock(s); err != nil {
			return false, err
		}

		// you have the lock now
		return true, nil
	}

	// state is locked
	if string(lock) == string(s.Lock) {
		return true, nil
	}

	s.Lock = lock

	return false, nil
}

func (r *Lock) Unlock(s *terraform.State) (unlocked bool, err error) {
	mutex := r.client.NewMutex(lockKey, redsync.WithExpiry(12*time.Hour), redsync.WithTries(1), redsync.WithGenValueFunc(func() (string, error) {
		return uuid.New().String(), nil
	}))

	// lock the global redis mutex
	if err := mutex.Lock(); err != nil {
		log.Errorf("failed to lock redsync mutex: %v", err)

		return false, err
	}

	defer func() {
		// unlock the global redis mutex
		if _, mutErr := mutex.Unlock(); mutErr != nil {
			log.Errorf("failed to unlock redsync mutex: %v", mutErr)

			if err != nil {
				err = multierr.Append(err, mutErr)
			}
		}
	}()

	lock, err := r.getLock(s)
	if err != nil {
		return false, nil
	}

	if string(lock) != string(s.Lock) {
		return false, nil
	}

	if err := r.deleteLock(s); err != nil {
		return false, err
	}

	return true, nil
}

func (r *Lock) GetLock(s *terraform.State) (lock []byte, err error) {
	mutex := r.client.NewMutex(lockKey, redsync.WithExpiry(12*time.Hour), redsync.WithTries(1), redsync.WithGenValueFunc(func() (string, error) {
		return uuid.New().String(), nil
	}))

	// lock the global redis mutex
	if err := mutex.Lock(); err != nil {
		log.Errorf("failed to lock redsync mutex: %v", err)

		return nil, err
	}

	defer func() {
		// unlock the global redis mutex
		if _, mutErr := mutex.Unlock(); mutErr != nil {
			log.Errorf("failed to unlock redsync mutex: %v", mutErr)

			if err != nil {
				err = multierr.Append(err, mutErr)
			}
		}
	}()

	return r.getLock(s)
}

func (r *Lock) setLock(s *terraform.State) error {
	ctx := context.Background()

	conn, err := r.pool.GetContext(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	reply, err := redigo.String(conn.Do("SET", s.ID, base64.StdEncoding.EncodeToString(s.Lock), "NX", "PX", int(12*time.Hour/time.Millisecond)))
	if err != nil {
		return err
	}

	if reply != "OK" {
		return fmt.Errorf("could not set lock for id %s", s.ID)
	}

	return nil
}

func (r *Lock) getLock(s *terraform.State) ([]byte, error) {
	ctx := context.Background()

	conn, err := r.pool.GetContext(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	value, err := redigo.String(conn.Do("GET", s.ID))
	if err != nil {
		return nil, err
	}

	lock, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}

	return lock, nil
}

func (r *Lock) deleteLock(s *terraform.State) error {
	ctx := context.Background()

	conn, err := r.pool.GetContext(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	count, err := redigo.Int(conn.Do("DEL", s.ID))
	if err != nil {
		return err
	}

	if count != 1 {
		return fmt.Errorf("deleted %d redis keys while unlocking id %s", count, s.ID)
	}

	return nil
}
