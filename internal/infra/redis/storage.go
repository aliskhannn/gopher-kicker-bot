package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aliskhannn/gopher-kicker-bot/internal/config"
	"github.com/aliskhannn/gopher-kicker-bot/internal/models"
)

type Storage struct {
	client *redis.Client
}

func New(ctx context.Context, cfg *config.Config) (*Storage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Username:     cfg.Redis.User,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		MaxRetries:   cfg.Redis.MaxRetries,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.Timeout,
		WriteTimeout: cfg.Redis.Timeout,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Storage{client: client}, nil
}

func (s *Storage) SetPending(ctx context.Context, state *models.UserState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("pending:%d", state.UserID)

	ttl := int(state.TimeoutAt.Sub(time.Now()).Seconds())

	return s.client.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
}

func (s *Storage) GetPending(ctx context.Context, userID int64) (*models.UserState, bool, error) {
	key := fmt.Sprintf("pending:%d", userID)
	date, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var state models.UserState
	if err := json.Unmarshal([]byte(date), &state); err != nil {
		return nil, false, err
	}

	return &state, true, nil
}

func (s *Storage) GetAllPending(ctx context.Context) ([]*models.UserState, error) {
	var states []*models.UserState
	var cursor uint64

	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, "pending:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}

		for _, key := range keys {
			data, err := s.client.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}

			var state models.UserState
			if err := json.Unmarshal(data, &state); err != nil {
				continue
			}
			states = append(states, &state)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return states, nil
}

func (s *Storage) DeletePending(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("pending:%d", userID)
	return s.client.Del(ctx, key).Err()
}

func (s *Storage) GetTimedOut(ctx context.Context) ([]*models.UserState, error) {
	var timedOut []*models.UserState
	keys, err := s.client.Keys(ctx, "pending:*").Result()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for _, key := range keys {
		data, err := s.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var state models.UserState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		if now.After(state.TimeoutAt) {
			timedOut = append(timedOut, &state)
		}
	}

	return timedOut, nil
}

func (s *Storage) Close() error {
	return s.client.Close()
}
