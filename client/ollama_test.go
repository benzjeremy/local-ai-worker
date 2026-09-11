package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClientGenerateAndPing(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ollama is running"))
			return
		}

		if r.URL.Path == "/api/generate" {
			var req GenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			resp := GenerateResponse{
				Model:    req.Model,
				Response: "Antwort auf: " + req.Prompt,
				Done:     true,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	cli := NewOllamaClient(mockServer.URL)

	if !cli.Ping(context.Background()) {
		t.Fatalf("Ping failed on mock server")
	}

	ans, err := cli.Generate(context.Background(), GenerateRequest{
		Model:  "llama3.1:8b",
		Prompt: "Was ist Zero-Dummy-Security?",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if ans != "Antwort auf: Was ist Zero-Dummy-Security?" {
		t.Errorf("Unexpected response: %s", ans)
	}
}
