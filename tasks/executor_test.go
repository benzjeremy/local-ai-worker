package tasks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benzjeremy/local-ai-worker/client"
	"github.com/benzjeremy/local-ai-worker/feedback"
	"github.com/benzjeremy/local-ai-worker/knowledge"
	"github.com/benzjeremy/local-ai-worker/storage"
)

func TestTaskExecutor(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := client.GenerateResponse{
			Response: "Zusammenfassung: Erfolgreich ausgeführt.",
			Done:     true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	tmpDir, _ := os.MkdirTemp("", "task_test_*")
	defer os.RemoveAll(tmpDir)

	v, _ := storage.OpenVault(filepath.Join(tmpDir, "vault.enc"), "TaskSecret123!")
	fb := feedback.NewEngine(v)
	cli := client.NewOllamaClient(mockServer.URL)
	idx := knowledge.NewIndexer()

	// Seed one document
	doc := knowledge.ParseMarkdown("/tmp/test.md", "test.md", "# 🚀 Test Note\nContent here.\n#level-leaf", time.Now())
	idx.Build([]*knowledge.Document{doc})

	executor := NewExecutor(cli, idx, fb, "llama3.1:8b")

	// 1. Vault Audit
	auditRes, err := executor.Execute(context.Background(), TaskRequest{
		Type: TaskVaultAudit,
	})
	if err != nil {
		t.Fatalf("Audit task failed: %v", err)
	}
	if auditRes.Metadata["total_notes"] != "1" {
		t.Errorf("Expected 1 total note in audit, got %s", auditRes.Metadata["total_notes"])
	}

	// 2. Summarize
	sumRes, err := executor.Execute(context.Background(), TaskRequest{
		Type:        TaskSummarize,
		VaultTarget: "Test Note",
	})
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	if sumRes.Output == "" {
		t.Errorf("Empty summarize output")
	}

	// 3. Unknown task -> error
	_, err = executor.Execute(context.Background(), TaskRequest{
		Type: "unknown_task",
	})
	if err == nil {
		t.Errorf("Expected error on unknown task")
	}
}
