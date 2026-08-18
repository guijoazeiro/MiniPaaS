package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildQueueIsFIFOAndReportsCapacity(t *testing.T) {
	queue := NewBuildQueue(1)
	first, err := queue.Acquire(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	secondReady := make(chan func(), 1)
	go func() {
		release, acquireErr := queue.Acquire(context.Background(), uuid.New())
		if acquireErr != nil {
			return
		}
		secondReady <- release
	}()

	deadline := time.Now().Add(time.Second)
	for queue.Stats().Queued != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if stats := queue.Stats(); stats.Active != 1 || stats.Queued != 1 || stats.Limit != 1 {
		t.Fatalf("queue stats = %#v", stats)
	}
	first()

	select {
	case release := <-secondReady:
		release()
	case <-time.After(time.Second):
		t.Fatal("second build did not receive the released slot")
	}
	if stats := queue.Stats(); stats.Active != 0 || stats.Queued != 0 {
		t.Fatalf("queue did not drain: %#v", stats)
	}
}

func TestBuildQueueCancellationRemovesWaiter(t *testing.T) {
	queue := NewBuildQueue(1)
	release, err := queue.Acquire(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queue.Acquire(ctx, uuid.New()); err == nil {
		t.Fatal("expected cancelled acquire")
	}
	if stats := queue.Stats(); stats.Active != 1 || stats.Queued != 0 {
		t.Fatalf("cancelled waiter remained in queue: %#v", stats)
	}
	release()
}
