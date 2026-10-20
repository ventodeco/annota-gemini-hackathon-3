package storage

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
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

func TestRedisClientPingReportsAvailability(t *testing.T) {
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

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() with Redis up error = %v", err)
	}

	mr.Close()
	if err := client.Ping(context.Background()); err == nil {
		t.Fatal("Ping() with Redis stopped should fail")
	}
}

// stallableRedisProxy forwards to miniredis until stall is set, then withholds
// every server response.
func stallableRedisProxy(t *testing.T) (string, *atomic.Bool) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	var stall atomic.Bool
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			server, err := net.Dial("tcp", mr.Addr())
			if err != nil {
				client.Close()
				return
			}
			t.Cleanup(func() { client.Close(); server.Close() })
			go io.Copy(server, client)
			go func() {
				buf := make([]byte, 4096)
				for {
					n, err := server.Read(buf)
					if err != nil || stall.Load() {
						return
					}
					client.Write(buf[:n])
				}
			}()
		}
	}()
	return listener.Addr().String(), &stall
}

func TestRedisClientPingHonorsContextDeadline(t *testing.T) {
	addr, stall := stallableRedisProxy(t)
	client, err := NewRedisClient(addr)
	if err != nil {
		t.Fatalf("NewRedisClient() error = %v", err)
	}
	defer client.Close()
	stall.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := client.Ping(ctx); err == nil {
		t.Fatal("Ping() against a stalled server should fail")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Ping() took %s, want it bounded by the context deadline", elapsed)
	}
}
