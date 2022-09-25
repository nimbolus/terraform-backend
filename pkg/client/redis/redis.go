package redis

import (
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/spf13/viper"
)

func NewPool() *redigo.Pool {
	viper.SetDefault("redis_addr", "localhost:6379")

	return &redigo.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redigo.Conn, error) {
			c, err := redigo.Dial("tcp", viper.GetString("redis_addr"))
			if err != nil {
				if c != nil {
					c.Close()
				}

				return nil, err
			}

			pass := viper.GetString("redis_password")
			if pass != "" {
				if _, err := c.Do("AUTH"); err != nil {
					c.Close()

					return nil, err
				}
			}

			return c, nil
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			_, err := c.Do("PING")

			return err
		},
	}
}
