package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKnowledgeParseAndBM25(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault_knowledge_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 2 test markdown files
	doc1Content := `🏠 [[Standards]]

# 🛡️ Jeremy Benz – Coding & Security Standards

Zero-Dummy-Security ist unser Standard:
- Passwörter werden niemals im Klartext gespeichert.
- AES-256-GCM Verschlüsselung für alle sensiblen Nutzereinstellungen.
- PBKDF2 mit 100000 Runden für Key Derivation.

#level-leaf #security #standards
`

	doc2Content := `🏠 [[Jeremy Benz]]

# 📦 Release & Versioning Workflow

Immer SemVer ohne Trailing Zeros:
- Richtig: v1.0, v1.6, v2.0, v2.1
- Falsch: v1.0.0, v2.0.0

#level-leaf #releases #semver
`

	p1 := filepath.Join(tmpDir, "Coding-Standards.md")
	p2 := filepath.Join(tmpDir, "Release-Workflow.md")

	if err := os.WriteFile(p1, []byte(doc1Content), 0644); err != nil {
		t.Fatalf("WriteFile p1 failed: %v", err)
	}
	if err := os.WriteFile(p2, []byte(doc2Content), 0644); err != nil {
		t.Fatalf("WriteFile p2 failed: %v", err)
	}

	// Test Scanner
	scanner := NewScanner(tmpDir)
	docs, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scanner failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("Expected 2 docs, found %d", len(docs))
	}

	// Test Document parsing
	d1 := docs[0]
	if d1.RelPath == "Release-Workflow.md" {
		d1 = docs[1]
	}
	if d1.LevelTag != "#level-leaf" {
		t.Errorf("Expected LevelTag #level-leaf, got %s", d1.LevelTag)
	}
	if len(d1.WikiLinks) == 0 || d1.WikiLinks[0] != "Standards" {
		t.Errorf("Expected WikiLink Standards, got %v", d1.WikiLinks)
	}

	// Test Indexer
	idx := NewIndexer()
	idx.Build(docs)
	if idx.Count() != 2 {
		t.Errorf("Expected 2 indexed docs, got %d", idx.Count())
	}

	// Query 1: Security
	res1 := idx.Search("AES-256-GCM PBKDF2 Verschlüsselung", 5)
	if len(res1) == 0 {
		t.Fatalf("Expected match for AES-256-GCM")
	}
	if res1[0].Document.Title != "🛡️ Jeremy Benz – Coding & Security Standards" {
		t.Errorf("Top result mismatch: got %s", res1[0].Document.Title)
	}
	if len(res1[0].Snippets) == 0 {
		t.Errorf("Expected snippet extraction")
	}

	// Query 2: SemVer
	res2 := idx.Search("SemVer Trailing Zeros", 5)
	if len(res2) == 0 {
		t.Fatalf("Expected match for SemVer")
	}
	if res2[0].Document.Title != "📦 Release & Versioning Workflow" {
		t.Errorf("Top result mismatch: got %s", res2[0].Document.Title)
	}
}

func TestStripMarkdown(t *testing.T) {
	raw := "## Headline\n[[Link|Text]] und **fett** mit `code`."
	doc := ParseMarkdown("/tmp/test.md", "test.md", raw, time.Now())
	if doc.Title != "Headline" {
		t.Errorf("Title parsed incorrectly: %s", doc.Title)
	}
	if !doc.ModTime.Before(time.Now().Add(time.Second)) {
		t.Errorf("ModTime is invalid")
	}
}
