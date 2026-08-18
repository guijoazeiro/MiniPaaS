package service

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type buildWaiter struct {
	id      uuid.UUID
	ready   chan struct{}
	granted bool
}

// BuildQueue is a small FIFO scheduler for Docker builds. It keeps waiting
// deployments visible to capacity readers and lets cancellation remove a
// deployment before it consumes a build slot.
type BuildQueue struct {
	mu      sync.Mutex
	limit   int
	active  int
	waiters []*buildWaiter
}

func NewBuildQueue(limit int) *BuildQueue {
	if limit < 1 {
		limit = 1
	}
	return &BuildQueue{limit: limit}
}

func (q *BuildQueue) Acquire(ctx context.Context, deploymentID uuid.UUID) (func(), error) {
	waiter := &buildWaiter{id: deploymentID, ready: make(chan struct{})}
	q.mu.Lock()
	q.waiters = append(q.waiters, waiter)
	q.dispatchLocked()
	q.mu.Unlock()

	select {
	case <-waiter.ready:
		return q.releaseOnce(), nil
	case <-ctx.Done():
		q.mu.Lock()
		if waiter.granted {
			q.mu.Unlock()
			q.release()
			return nil, ctx.Err()
		}
		q.removeWaiterLocked(waiter)
		q.dispatchLocked()
		q.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (q *BuildQueue) releaseOnce() func() {
	var once sync.Once
	return func() { once.Do(q.release) }
}

func (q *BuildQueue) release() {
	q.mu.Lock()
	if q.active > 0 {
		q.active--
	}
	q.dispatchLocked()
	q.mu.Unlock()
}

func (q *BuildQueue) dispatchLocked() {
	for q.active < q.limit && len(q.waiters) > 0 {
		waiter := q.waiters[0]
		q.waiters = q.waiters[1:]
		waiter.granted = true
		q.active++
		close(waiter.ready)
	}
}

func (q *BuildQueue) removeWaiterLocked(target *buildWaiter) {
	for i, waiter := range q.waiters {
		if waiter == target {
			copy(q.waiters[i:], q.waiters[i+1:])
			q.waiters[len(q.waiters)-1] = nil
			q.waiters = q.waiters[:len(q.waiters)-1]
			return
		}
	}
}

func (q *BuildQueue) Stats() domain.BuildQueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	return domain.BuildQueueStats{Limit: q.limit, Active: q.active, Queued: len(q.waiters)}
}
