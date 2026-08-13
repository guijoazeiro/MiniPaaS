package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type fakeManagedCleaner struct {
	items   []docker.ManagedContainer
	removed []string
	err     error
}

func (f *fakeManagedCleaner) ListManagedContainers(context.Context) ([]docker.ManagedContainer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func (f *fakeManagedCleaner) RemoveContainer(_ context.Context, id string) error {
	if id == "remove-error" {
		return errors.New("remove failed")
	}
	f.removed = append(f.removed, id)
	return nil
}

func TestReconcileManagedContainerIDsKeepsReferencedAndRemovesOrphans(t *testing.T) {
	cleaner := &fakeManagedCleaner{items: []docker.ManagedContainer{
		{ID: "active"}, {ID: "candidate"}, {ID: "orphan"}, {ID: "remove-error"},
	}}
	removed, err := reconcileManagedContainerIDs(context.Background(), cleaner, map[string]struct{}{
		"active": {}, "candidate": {},
	})
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("cleanup error = %v, want remove failed", err)
	}
	if len(cleaner.removed) != 1 || cleaner.removed[0] != "orphan" {
		t.Fatalf("removed IDs = %v, want [orphan]", cleaner.removed)
	}
}

func TestReferencedContainerIDsIncludesCandidates(t *testing.T) {
	ids := referencedContainerIDs([]domain.Deployment{{ContainerID: "active", CandidateContainerID: "candidate"}})
	if len(ids) != 2 {
		t.Fatalf("referenced IDs = %#v, want active and candidate", ids)
	}
}
