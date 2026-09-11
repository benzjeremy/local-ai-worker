package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/benzjeremy/local-ai-worker/client"
)

func TestVisionAnalyze(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			var req client.GenerateRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Images) == 0 {
				http.Error(w, "missing images", http.StatusBadRequest)
				return
			}

			resp := client.GenerateResponse{
				Model:    req.Model,
				Response: "CAD Zeichnung: Bohrung Ø 12mm, Toleranz H7, Material AlMg3.",
				Done:     true,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	cli := client.NewOllamaClient(mockServer.URL)
	analyzer := NewAnalyzer(cli, "llava:7b")

	// Test with Base64
	fakeImage := base64.StdEncoding.EncodeToString([]byte("fake png binary data"))
	res, err := analyzer.Analyze(context.Background(), Request{
		Type:        TypeCADDrawing,
		Base64Image: "data:image/png;base64," + fakeImage,
		Prompt:      "Analysiere Maße und Toleranzen",
	})
	if err != nil {
		t.Fatalf("Analyze with Base64 failed: %v", err)
	}

	if res.Analysis == "" {
		t.Errorf("Empty analysis result")
	}

	// Test with File
	tmpDir, _ := os.MkdirTemp("", "vision_test_*")
	defer os.RemoveAll(tmpDir)
	filePath := filepath.Join(tmpDir, "test.png")
	os.WriteFile(filePath, []byte("raw image data"), 0644)

	resFile, err := analyzer.Analyze(context.Background(), Request{
		Type:     TypeGeneralScreenshot,
		FilePath: filePath,
	})
	if err != nil {
		t.Fatalf("Analyze with FilePath failed: %v", err)
	}

	if resFile.Type != string(TypeGeneralScreenshot) {
		t.Errorf("Type mismatch: %s", resFile.Type)
	}
}

func TestIsImageExtension(t *testing.T) {
	if !IsImageExtension("test.png") || !IsImageExtension("sample.JPG") {
		t.Errorf("Expected valid images to be detected")
	}
	if IsImageExtension("file.md") || IsImageExtension("app.go") {
		t.Errorf("Non-images falsely identified as image")
	}
}
