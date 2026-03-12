package storage

import (
	"context"
	"testing"
	"time"
)

func skipIfNoRedis(t *testing.T) RedisClient {
	t.Helper()
	client, err := NewRedisClient("localhost:6379")
	if err != nil {
		t.Skip("requires Redis: " + err.Error())
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestRedis_SetGetDeleteState(t *testing.T) {
	client := skipIfNoRedis(t)
	ctx := context.Background()

	state := "test_state_" + time.Now().Format("20060102150405.000")
	sessionID := "session_abc123"

	// Set
	err := client.SetState(ctx, state, sessionID, 30*time.Second)
	if err != nil {
		t.Fatalf("SetState returned error: %v", err)
	}

	// Get
	got, err := client.GetState(ctx, state)
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if got != sessionID {
		t.Errorf("GetState = %q, want %q", got, sessionID)
	}

	// Delete
	err = client.DeleteState(ctx, state)
	if err != nil {
		t.Fatalf("DeleteState returned error: %v", err)
	}

	// Get after delete should return error (redis.Nil)
	got, err = client.GetState(ctx, state)
	if err == nil {
		t.Errorf("expected error after delete, got value: %q", got)
	}
}

func TestRedis_GetState_NonExistent(t *testing.T) {
	client := skipIfNoRedis(t)
	ctx := context.Background()

	got, err := client.GetState(ctx, "nonexistent_key_"+time.Now().Format("20060102150405.000"))
	if err == nil {
		t.Errorf("expected error for non-existent key, got value: %q", got)
	}
}
