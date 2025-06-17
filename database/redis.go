package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig конфиг для redis
type RedisConfig struct {
	Address  string `yaml:"address"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// GetRedisClient получаем новый клиент для redis
func GetRedisClient(cfg RedisConfig) (db *redis.Client, err error) {
	ctx := context.Background()

	db = redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		Username: cfg.User,
	})

	if err = db.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return db, nil
}

// FindInCache ищем в кеше
func FindInCache(db *redis.Client, limit, offset int) (payload json.RawMessage, err error) {
	ctx := context.Background()
	res, err := db.Get(ctx, fmt.Sprintf("%d-%d", limit, offset)).Bytes()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// PutInCache записываем в кеш
func PutInCache(db *redis.Client, payload json.RawMessage, limit, offset int) (err error) {
	ctx := context.Background()
	return db.Set(ctx, fmt.Sprintf("%d-%d", limit, offset), string(payload), time.Minute).Err()
}

// InvalidateCache ивалидируем кеш
func InvalidateCache(db *redis.Client) error {
	ctx := context.Background()
	return db.FlushDB(ctx).Err()
}
