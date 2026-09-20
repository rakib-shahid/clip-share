package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestJobStatusesPreserveRequestOrderAndOwnership(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "statuses.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	alice, err := database.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := database.CreateUser(ctx, "Bob", "bob", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	first := createJobForControlTest(t, database, alice, "First", "statuses-first")
	second := createJobForControlTest(t, database, alice, "Second", "statuses-second")
	other := createJobForControlTest(t, database, bob, "Other", "statuses-other")
	statuses, err := database.JobStatuses(ctx, []int64{second.ID, other.ID, first.ID, 999999}, &alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0].JobID != second.ID || statuses[1].JobID != first.ID {
		t.Fatalf("statuses=%+v", statuses)
	}
	if statuses[0].ClipID != second.ClipID || statuses[0].Progress == nil || *statuses[0].Progress != 0 || statuses[0].SizeBytes != nil {
		t.Fatalf("status projection=%+v", statuses[0])
	}
}
