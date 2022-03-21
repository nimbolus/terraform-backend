package redis

import (
	redis "github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

func NewRedisClient() *redis.Client {
	viper.SetDefault("redis_addr", "localhost:6379")
	return redis.NewClient(&redis.Options{
		Addr:     viper.GetString("redis_addr"),
		Password: viper.GetString("redis_password"),
	})
}
