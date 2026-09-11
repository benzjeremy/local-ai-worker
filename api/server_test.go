package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/benzjeremy/local-ai-worker/client"
	"github.com/benzjeremy/local-ai-worker/feedback"
	"github.com/benzjeremy/local-ai-worker/knowledge"
	"github.com/benzjeremy/local-ai-worker/storage"
	"github.com/benzjeremy/local-ai-worker/tasks"
	"github.com/benzjeremy/local-ai-worker/vision"
)

func setupTestServer(t *testing.T) (*Server, *httptest.Server, func()) {
	// Mock Ollama
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := client.GenerateResponse{
			Response: "Lokale Inferenz Antwort für Test",
			Done:     true,
		}
		json.NewEncoder(w).Encode(resp)
	}))

	tmpDir, _ := os.MkdirTemp("", "api_test_*")

	// Create test markdown note
	docContent := "# 🛡️ Security Note\nZero-Dummy-Security mit AES-256-GCM.\n#level-leaf"
	os.WriteFile(filepath.Join(tmpDir, "Security.md"), []byte(docContent), 0644)

	vault, _ := storage.OpenVault(filepath.Join(tmpDir, "vault.enc"), "TestPass123!")
	fb := feedback.NewEngine(vault)
	cli := client.NewOllamaClient(mockOllama.URL)
	scn := knowledge.NewScanner(tmpDir)
	docs, _ := scn.Scan()
	idx := knowledge.NewIndexer()
	idx.Build(docs)
	vis := vision.NewAnalyzer(cli, "llava:7b")
	exec := tasks.NewExecutor(cli, idx, fb, "llama3.1:8b")

	cfg := ServerConfig{
		Port:         0,
		Token:        "fixed-secure-32-byte-testing-token-2026",
		VaultDir:     tmpDir,
		DefaultModel: "llama3.1:8b",
		Version:      "v1.0",
	}

	srv, err := NewServer(cfg, scn, idx, fb, vis, exec, cli)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	handler := srv.securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			srv.handleHealth(w, r)
		case "/ask":
			srv.handleAsk(w, r)
		case "/feedback":
			srv.handleFeedback(w, r)
		case "/task/run":
			srv.handleTask(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	ts := httptest.NewServer(handler)

	cleanup := func() {
		ts.Close()
		mockOllama.Close()
		os.RemoveAll(tmpDir)
	}

	return srv, ts, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	_, ts, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if data["service"] != "local-ai-worker" || data["version"] != "v1.0" {
		t.Errorf("Unexpected health data: %v", data)
	}
}

func TestSecurityTokenAndHeaders(t *testing.T) {
	_, ts, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Without token -> 401
	resp, err := http.Post(ts.URL+"/ask", "application/json", bytes.NewReader([]byte(`{"query":"test"}`)))
	if err != nil {
		t.Fatalf("POST /ask failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized without token, got %d", resp.StatusCode)
	}

	// 2. With valid token
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader([]byte(`{"query":"Security"}`)))
	req.Header.Set("X-Worker-Token", "fixed-secure-32-byte-testing-token-2026")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /ask with token failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK with token, got %d", resp2.StatusCode)
	}

	// Verify security headers
	if resp2.Header.Get("X-Frame-Options") != "DENY" || resp2.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Missing security headers: %v", resp2.Header)
	}
}

func TestAntiDNSRebindingAndCSRF(t *testing.T) {
	_, ts, cleanup := setupTestServer(t)
	defer cleanup()

	client := &http.Client{}

	// DNS Rebinding with malicious Host
	reqBadHost, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	reqBadHost.Host = "evil-hacker.com"
	respHost, err := client.Do(reqBadHost)
	if err != nil {
		t.Fatalf("Bad host request failed: %v", err)
	}
	respHost.Body.Close()
	if respHost.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden on DNS rebinding, got %d", respHost.StatusCode)
	}

	// CSRF with malicious Origin
	reqBadOrigin, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	reqBadOrigin.Header.Set("Origin", "http://attacker-site.com")
	respOrigin, err := client.Do(reqBadOrigin)
	if err != nil {
		t.Fatalf("Bad origin request failed: %v", err)
	}
	respOrigin.Body.Close()
	if respOrigin.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden on CSRF origin, got %d", respOrigin.StatusCode)
	}
}

func TestFeedbackAndTaskWorkflow(t *testing.T) {
	_, ts, cleanup := setupTestServer(t)
	defer cleanup()

	client := &http.Client{}

	// Post feedback
	fbPayload := map[string]interface{}{
		"query":              "Wie lautet die Regel?",
		"original_response":  "Es gibt keine Regel.",
		"corrected_response": "Immer strikte Second Brain Standards einhalten.",
		"tags":               []string{"standards"},
	}
	fbJSON, _ := json.Marshal(fbPayload)

	reqFb, _ := http.NewRequest(http.MethodPost, ts.URL+"/feedback", bytes.NewReader(fbJSON))
	reqFb.Header.Set("X-Worker-Token", "fixed-secure-32-byte-testing-token-2026")
	reqFb.Header.Set("Content-Type", "application/json")

	respFb, err := client.Do(reqFb)
	if err != nil {
		t.Fatalf("POST /feedback failed: %v", err)
	}
	respFb.Body.Close()
	if respFb.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK on feedback, got %d", respFb.StatusCode)
	}

	// Run Vault Audit Task
	taskPayload := map[string]interface{}{
		"type": "vault_audit",
	}
	taskJSON, _ := json.Marshal(taskPayload)

	reqTask, _ := http.NewRequest(http.MethodPost, ts.URL+"/task/run", bytes.NewReader(taskJSON))
	reqTask.Header.Set("X-Worker-Token", "fixed-secure-32-byte-testing-token-2026")
	reqTask.Header.Set("Content-Type", "application/json")

	respTask, err := client.Do(reqTask)
	if err != nil {
		t.Fatalf("POST /task/run failed: %v", err)
	}
	defer respTask.Body.Close()
	if respTask.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK on task execution, got %d", respTask.StatusCode)
	}
}
