package repository

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"url-shortener/config"
	"url-shortener/internal/models"
)

type RedisRepository interface {
	GetURL(ctx context.Context, shortCode string) (*models.URL, error)
	SetURL(ctx context.Context, url *models.URL, ttl time.Duration) error
	DeleteURL(ctx context.Context, shortCode string) error
	IncrementClickCount(ctx context.Context, shortCode string) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}

type redisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) RedisRepository {
	return &redisRepository{client: client}
}

func (r *redisRepository) GetURL(ctx context.Context, shortCode string) (*models.URL, error) {
	key := "url:" + shortCode
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrNotFound
	}
	
	url := &models.URL{
		ShortCode:  shortCode,
		LongURL:    data["long_url"],
		ClickCount: 0,
	}
	
	if clickCount, err := r.client.Get(ctx, "clicks:"+shortCode).Int64(); err == nil {
		url.ClickCount = clickCount
	}
	
	return url, nil
}

func (r *redisRepository) SetURL(ctx context.Context, url *models.URL, ttl time.Duration) error {
	key := "url:" + url.ShortCode
	pipe := r.client.Pipeline()
	
	pipe.HSet(ctx, key, map[string]interface{}{
		"long_url": url.LongURL,
	})
	
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisRepository) DeleteURL(ctx context.Context, shortCode string) error {
	key := "url:" + shortCode
	clicksKey := "clicks:" + shortCode
	return r.client.Del(ctx, key, clicksKey).Err()
}

func (r *redisRepository) IncrementClickCount(ctx context.Context, shortCode string) (int64, error) {
	key := "clicks:" + shortCode
	return r.client.Incr(ctx, key).Result()
}

func (r *redisRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *redisRepository) Close() error {
	return r.client.Close()
}

func NewRedisClient(cfg *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         cfg.Address(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
}