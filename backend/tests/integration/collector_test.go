package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/collector"
)

func TestCollectorClientOnline(t *testing.T) {
	dir := t.TempDir()
	client, err := collector.NewLocalCollector(dir)
	if err != nil {
		t.Fatalf("new local collector: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if !client.Online(ctx) {
		t.Fatal("expected collector to be online")
	}
}

func TestCollectorClientAcceptedFileTypes(t *testing.T) {
	dir := t.TempDir()
	client, err := collector.NewLocalCollector(dir)
	if err != nil {
		t.Fatalf("new local collector: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	types, err := client.AcceptedFileTypes(ctx)
	if err != nil {
		t.Fatalf("accepted file types: %v", err)
	}
	if len(types) == 0 {
		t.Fatal("expected at least one accepted file type")
	}
}

func TestCollectorClientProcessDocument(t *testing.T) {
	dir := t.TempDir()
	client, err := collector.NewLocalCollector(dir)
	if err != nil {
		t.Fatalf("new local collector: %v", err)
	}

	// Create a real text file to process
	txtPath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello, this is a test document."), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.ProcessDocument(ctx, txtPath, nil)
	if err != nil {
		t.Fatalf("process document: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Reason)
	}
	if len(result.Documents) == 0 {
		t.Fatal("expected at least one document")
	}
	if result.Documents[0].WordCount == 0 {
		t.Fatal("expected non-zero word count")
	}
}
