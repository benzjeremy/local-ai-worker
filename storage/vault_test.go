package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultEncryptionCycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test_vault.enc")
	passphrase := "HyperSecureZeroDummyStandardPassword2026!"

	// Open initial
	v, err := OpenVault(vaultPath, passphrase)
	if err != nil {
		t.Fatalf("OpenVault failed: %v", err)
	}

	// Add feedback
	rec := FeedbackRecord{
		ID:                "fb-1",
		Query:             "Was ist der Standard für Releases?",
		OriginalResponse:  "Nutze SemVer mit Trailing Zeros",
		CorrectedResponse: "Niemals Trailing Zeros nutzen: v1.0, v2.1 statt v1.0.0",
		ContextTags:       []string{"standards", "releases"},
		Timestamp:         "2026-09-11T06:00:00Z",
	}

	if err := v.AddFeedback(rec); err != nil {
		t.Fatalf("AddFeedback failed: %v", err)
	}

	if err := v.SetSetting("default_model", "llama3.1:8b"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Reopen with correct password
	v2, err := OpenVault(vaultPath, passphrase)
	if err != nil {
		t.Fatalf("Re-opening vault failed: %v", err)
	}

	records := v2.GetFeedback()
	if len(records) != 1 {
		t.Fatalf("Expected 1 feedback record, got %d", len(records))
	}

	if records[0].CorrectedResponse != rec.CorrectedResponse {
		t.Errorf("Mismatch in decrypted feedback: got %s, want %s", records[0].CorrectedResponse, rec.CorrectedResponse)
	}

	if v2.GetSetting("default_model") != "llama3.1:8b" {
		t.Errorf("Mismatch in decrypted setting: got %s", v2.GetSetting("default_model"))
	}

	// Reopen with wrong password -> must fail
	_, err = OpenVault(vaultPath, "WrongPassword!")
	if err == nil {
		t.Fatalf("Opening vault with wrong password should fail")
	}
}
