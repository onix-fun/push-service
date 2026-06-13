package main

import (
	"path/filepath"
	"testing"
)

func TestIdempotencyStoreSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delivered.log")
	store, err := openIdempotencyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDelivered("event-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.file.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := openIdempotencyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.file.Close()
	if !reopened.Contains("event-1") {
		t.Fatal("expected event to remain delivered after reopening the store")
	}
}
