package redislock

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/nimbolus/terraform-backend/redisclient"
	"github.com/nimbolus/terraform-backend/terraform"
)

type RedisLock struct {
	pool   redis.Pool
	client *redsync.Redsync
}

func NewRedisLock() *RedisLock {
	pool := goredis.NewPool(redisclient.NewRedisClient())

	return &RedisLock{
		pool:   pool,
		client: redsync.New(pool),
	}
}

func (r *RedisLock) GetName() string {
	return "redis"
}

func (r *RedisLock) Lock(s *terraform.State) (bool, error) {
	mutex := r.client.NewMutex(s.ID, redsync.WithExpiry(12*time.Hour), redsync.WithTries(1), redsync.WithGenValueFunc(genValueFunc(s)))
	err := mutex.Lock()
	if err == redsync.ErrFailed {
		if _, err := r.getLock(s); err != nil {
			return false, err
		}
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (r *RedisLock) Unlock(s *terraform.State) (bool, error) {
	if ok, err := r.getLock(s); err != nil {
		return false, err
	} else if !ok {
		return false, fmt.Errorf("no lock for id %s found", s.ID)
	}
	value := base64.StdEncoding.EncodeToString(s.Lock)
	mutex := r.client.NewMutex(s.ID, redsync.WithValue(value))
	return mutex.Unlock()
}

func genValueFunc(s *terraform.State) func() (string, error) {
	return func() (string, error) {
		return base64.StdEncoding.EncodeToString(s.Lock), nil
	}
}

func (r *RedisLock) getLock(s *terraform.State) (bool, error) {
	ctx := context.Background()
	conn, err := r.pool.Get(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	value, err := conn.Get(s.ID)
	if err != nil {
		return false, err
	}
	s.Lock, err = base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
