package feedback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benzjeremy/local-ai-worker/storage"
)

func TestFeedbackLearningLoop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feedback_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	v, err := storage.OpenVault(filepath.Join(tmpDir, "vault.enc"), "SecretPass123!")
	if err != nil {
		t.Fatalf("failed to open vault: %v", err)
	}

	engine := NewEngine(v)

	// Record a correction
	err = engine.RecordCorrection(
		"Wie versionieren wir neue Releases?",
		"Wir hängen immer .0 an, also v1.0.0.",
		"Niemals Trailing Zeros! Immer v1.0, v2.1 ohne extra Null.",
		[]string{"releases", "semver"},
	)
	if err != nil {
		t.Fatalf("RecordCorrection failed: %v", err)
	}

	list := engine.ListCorrections()
	if len(list) != 1 {
		t.Fatalf("Expected 1 correction, got %d", len(list))
	}

	// Query matching context
	promptCtx := engine.BuildPromptContext("Kannst du mir die Releases und Versionierung erklären?")
	if promptCtx == "" {
		t.Fatalf("Expected prompt context with learned rules")
	}

	if !strings.Contains(promptCtx, "Niemals Trailing Zeros!") {
		t.Errorf("Prompt context missing corrected rule: %s", promptCtx)
	}

	// Irrelevant query -> empty context
	emptyCtx := engine.BuildPromptContext("Wie kocht man Kaffee?")
	if emptyCtx != "" {
		t.Errorf("Expected empty context for unrelated query, got: %s", emptyCtx)
	}
}
