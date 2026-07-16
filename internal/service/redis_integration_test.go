package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func integrationRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("MALL_REDIS_INTEGRATION_ADDR")
	if addr == "" {
		t.Skip("set MALL_REDIS_INTEGRATION_ADDR to run Redis integration tests")
	}
	db := 0
	if rawDB := os.Getenv("MALL_REDIS_INTEGRATION_DB"); rawDB != "" {
		value, err := strconv.Atoi(rawDB)
		if err != nil {
			t.Fatalf("invalid MALL_REDIS_INTEGRATION_DB: %v", err)
		}
		db = value
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("MALL_REDIS_INTEGRATION_PASSWORD"),
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("connect Redis integration environment: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRedisIntegrationCartTokenAndIdempotencySemantics(t *testing.T) {
	client := integrationRedisClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	suffix := uuid.NewString()
	userID := time.Now().UnixNano()
	cart := cartKey(userID)
	refresh := refreshTokenKey("integration-" + suffix)
	idempotency := orderIdempotencyKey(userID, "integration-"+suffix)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		_ = client.Del(cleanupCtx, cart, refresh, idempotency).Err()
	})

	if err := client.HSet(ctx, cart, "1001", 2).Err(); err != nil {
		t.Fatalf("write cart hash: %v", err)
	}
	if quantity, err := client.HIncrBy(ctx, cart, "1001", 3).Result(); err != nil || quantity != 5 {
		t.Fatalf("increment cart quantity: quantity=%d err=%v", quantity, err)
	}
	if values, err := client.HGetAll(ctx, cart).Result(); err != nil || values["1001"] != "5" {
		t.Fatalf("read cart hash: values=%v err=%v", values, err)
	}

	if err := client.Set(ctx, refresh, userID, time.Minute).Err(); err != nil {
		t.Fatalf("store refresh token: %v", err)
	}
	if err := client.Del(ctx, refresh).Err(); err != nil {
		t.Fatalf("invalidate refresh token: %v", err)
	}
	if err := client.Get(ctx, refresh).Err(); err != redis.Nil {
		t.Fatalf("refresh token still exists: %v", err)
	}

	acquired, err := client.SetNX(ctx, idempotency, "processing", time.Minute).Result()
	if err != nil || !acquired {
		t.Fatalf("acquire idempotency key: acquired=%v err=%v", acquired, err)
	}
	acquiredAgain, err := client.SetNX(ctx, idempotency, fmt.Sprintf("order:%d", userID), time.Minute).Result()
	if err != nil || acquiredAgain {
		t.Fatalf("duplicate idempotency key was accepted: acquired=%v err=%v", acquiredAgain, err)
	}
}
