package docker

import "testing"

func TestCopyLabelsAlwaysMarksManagedContainers(t *testing.T) {
	if got := copyLabels(nil)["com.minipaas.managed"]; got != "true" {
		t.Fatalf("nil labels managed value = %q, want true", got)
	}
	labels := map[string]string{"com.minipaas.managed": "false", "custom": "value"}
	copy := copyLabels(labels)
	if copy["com.minipaas.managed"] != "true" || copy["custom"] != "value" {
		t.Fatalf("copied labels = %#v", copy)
	}
	if labels["com.minipaas.managed"] != "false" {
		t.Fatal("copyLabels mutated the input map")
	}
}
