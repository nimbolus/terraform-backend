package redisclient

import (
	redis "github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

func NewRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: viper.GetString("redis_addr"),
	})
}
