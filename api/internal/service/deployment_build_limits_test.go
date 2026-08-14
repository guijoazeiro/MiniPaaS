package service

import (
	"context"
	"testing"
	"time"
)

func TestAcquireBuildHonorsConcurrencyLimit(t *testing.T) {
	svc := &DeploymentService{buildSlots: make(chan struct{}, 1)}
	release, err := svc.acquireBuild(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := svc.acquireBuild(ctx); err == nil {
		t.Fatal("second acquire succeeded while the only build slot was occupied")
	}

	release()
	if next, err := svc.acquireBuild(context.Background()); err != nil {
		t.Fatalf("acquire after release: %v", err)
	} else {
		next()
	}
}

func TestAcquireBuildHonorsCancellation(t *testing.T) {
	svc := &DeploymentService{buildSlots: make(chan struct{}, 1)}
	first, err := svc.acquireBuild(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer first()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.acquireBuild(ctx); err == nil {
		t.Fatal("acquire succeeded with a cancelled context")
	}
}
