package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

//go:generate mockgen -destination=mock/cache_mock.go -package=mock . Cache

// ErrNotFound notifies about non existing key in cache
var ErrNotFound = errors.New("key is not found")

// Config cache configuration
type Config struct {
	Addr        string `yaml:"address"`
	ExpDuration string `yaml:"expiration_duration"`
}

// Cache defines an interface for interaction with cache
type Cache interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
}

// Redis implements Cache
type Redis struct {
	Client *redis.Client
	ctx    context.Context
	exp    time.Duration
}

// NewRedis constructor
func NewRedis(ctx context.Context, cfg *Config, lg *zap.Logger) (*Redis, error) {
	exp, err := time.ParseDuration(cfg.ExpDuration)
	if err != nil {
		return nil, fmt.Errorf("parse expiration duration: %w", err)
	}

	r := redis.NewClient(&redis.Options{
		Addr: cfg.Addr,
	})

	err = r.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	go func() {
		<-ctx.Done()

		err := r.FlushAll(context.Background()).Err()
		if err != nil {
			lg.Error("redis flush all", zap.Error(err))
		}

		err = r.Close()
		if err != nil {
			lg.Error("redis close conn", zap.Error(err))
		}

		lg.Info("redis flushed and closed")
	}()

	return &Redis{
		Client: r,
		ctx:    ctx,
		exp:    exp,
	}, nil
}

// Set sets value in cache
func (r *Redis) Set(key string, value []byte) error {
	return r.Client.Set(r.ctx, key, value, r.exp).Err()
}

// Get retrieves value from cache
func (r *Redis) Get(key string) ([]byte, error) {
	b, err := r.Client.Get(r.ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}

	return b, err
}
