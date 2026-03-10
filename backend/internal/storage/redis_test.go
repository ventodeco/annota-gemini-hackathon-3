package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisClientStateLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer mr.Close()

	client, err := NewRedisClient(mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.SetState(ctx, "state-1", "session-1", time.Minute); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}

	got, err := client.GetState(ctx, "state-1")
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if got != "session-1" {
		t.Fatalf("GetState() = %q, want session-1", got)
	}

	if err := client.DeleteState(ctx, "state-1"); err != nil {
		t.Fatalf("DeleteState() error = %v", err)
	}

	_, err = client.GetState(ctx, "state-1")
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("GetState() after delete error = %v, want redis.Nil", err)
	}
}

func TestRedisClientUsesExpectedKeyPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer mr.Close()

	client, err := NewRedisClient(mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisClient() error = %v", err)
	}
	defer client.Close()

	if err := client.SetState(context.Background(), "abc", "sid", time.Minute); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}

	got, err := mr.Get("oauth:state:abc")
	if err != nil {
		t.Fatalf("miniredis.Get() error = %v", err)
	}
	if got != "sid" {
		t.Fatalf("stored value = %q, want sid", got)
	}
}
