package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/benzjeremy/local-ai-worker/client"
	"github.com/benzjeremy/local-ai-worker/feedback"
	"github.com/benzjeremy/local-ai-worker/knowledge"
	"github.com/benzjeremy/local-ai-worker/tasks"
	"github.com/benzjeremy/local-ai-worker/vision"
)

// ServerConfig holds runtime parameters for the API server.
type ServerConfig struct {
	Port         int
	Token        string
	VaultDir     string
	DefaultModel string
	Version      string
}

// Server provides the hardened REST interface.
type Server struct {
	config   ServerConfig
	token    string
	scanner  *knowledge.Scanner
	indexer  *knowledge.Indexer
	feedback *feedback.Engine
	vision   *vision.Analyzer
	tasks    *tasks.Executor
	ollama   *client.OllamaClient
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
	docs     []*knowledge.Document
}

// NewServer initializes a new secured API server.
func NewServer(
	cfg ServerConfig,
	scanner *knowledge.Scanner,
	indexer *knowledge.Indexer,
	fb *feedback.Engine,
	vis *vision.Analyzer,
	exec *tasks.Executor,
	ollama *client.OllamaClient,
) (*Server, error) {
	tok := cfg.Token
	if tok == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("failed generating CSPRNG token: %w", err)
		}
		tok = hex.EncodeToString(b)
	}

	if cfg.Port <= 0 {
		cfg.Port = 8081
	}
	if cfg.Version == "" {
		cfg.Version = "v1.0"
	}

	s := &Server{
		config:   cfg,
		token:    tok,
		scanner:  scanner,
		indexer:  indexer,
		feedback: fb,
		vision:   vis,
		tasks:    exec,
		ollama:   ollama,
		docs:     make([]*knowledge.Document, 0),
	}

	return s, nil
}

// Token returns the active 32-byte security token.
func (s *Server) Token() string {
	return s.token
}

// Start begins serving on 127.0.0.1:port.
func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.config.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ask", s.handleAsk)
	mux.HandleFunc("/feedback", s.handleFeedback)
	mux.HandleFunc("/vision/analyze", s.handleVision)
	mux.HandleFunc("/task/run", s.handleTask)
	mux.HandleFunc("/knowledge/scan", s.handleScan)
	mux.HandleFunc("/knowledge/stats", s.handleStats)

	// Apply zero-dummy security middleware
	handler := s.securityMiddleware(mux)

	s.server = &http.Server{
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	go s.server.Serve(ln)
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Addr returns the actual listening address.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf("127.0.0.1:%d", s.config.Port)
}

func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Mandatory Security Headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")

		// 2. Anti-DNS-Rebinding: Validate Host header strictly
		host := r.Host
		if strings.Contains(host, ":") {
			host, _, _ = net.SplitHostPort(host)
		}
		if host != "127.0.0.1" && host != "localhost" && host != "" {
			http.Error(w, `{"error":"forbidden: dns rebinding protection"}`, http.StatusForbidden)
			return
		}

		// 3. Anti-CSRF: Validate Origin if present
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "null" {
			if !strings.HasPrefix(origin, "http://127.0.0.1") && !strings.HasPrefix(origin, "http://localhost") {
				http.Error(w, `{"error":"forbidden: cross-origin request blocked"}`, http.StatusForbidden)
				return
			}
		}

		// 4. Token validation (health endpoint exempt for monitoring)
		if r.URL.Path != "/health" {
			authHeader := r.Header.Get("X-Worker-Token")
			if authHeader == "" {
				authHeader = r.URL.Query().Get("token")
			}

			if authHeader != s.token {
				http.Error(w, `{"error":"unauthorized: invalid or missing security token"}`, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "ok",
		"service":       "local-ai-worker",
		"version":       s.config.Version,
		"indexed_notes": s.indexer.Count(),
		"vault_dir":     s.config.VaultDir,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

type AskRequest struct {
	Query string `json:"query"`
	Model string `json:"model,omitempty"`
	TopK  int    `json:"top_k,omitempty"`
}

type AskResponse struct {
	Answer   string                  `json:"answer"`
	Model    string                  `json:"model"`
	Sources  []knowledge.SearchResult `json:"sources"`
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 3
	}

	// 1. BM25 Search in Obsidian Vault
	sources := s.indexer.Search(req.Query, topK)

	var contextSB strings.Builder
	for i, src := range sources {
		contextSB.WriteString(fmt.Sprintf("\n[Source %d: %s (%s)]\n%s\n", i+1, src.Document.Title, src.Document.RelPath, src.Document.CleanText))
	}

	// 2. Feedback prompt augmentation
	fbContext := s.feedback.BuildPromptContext(req.Query)

	// 3. Compose Final Prompt
	systemPrompt := "You are Local AI Worker, an intelligent, privacy-first Second Brain assistant running entirely locally on Jeremy Benz's infrastructure. Answer queries strictly using the provided Vault sources and supervisor rules. If information is missing, state so directly."
	userPrompt := fmt.Sprintf("%s\n\n### RELEVANT VAULT KNOWLEDGE:\n%s\n\n### USER QUERY:\n%s", fbContext, contextSB.String(), req.Query)

	model := req.Model
	if model == "" {
		model = s.config.DefaultModel
	}
	if model == "" {
		model = "llama3.1:8b"
	}

	ans, err := s.ollama.Generate(r.Context(), client.GenerateRequest{
		Model:  model,
		System: systemPrompt,
		Prompt: userPrompt,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("failed calling local LLM: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AskResponse{
		Answer:  ans,
		Model:   model,
		Sources: sources,
	})
}

type FeedbackPayload struct {
	Query             string   `json:"query"`
	OriginalResponse  string   `json:"original_response"`
	CorrectedResponse string   `json:"corrected_response"`
	Tags              []string `json:"tags,omitempty"`
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var fb FeedbackPayload
	if err := json.NewDecoder(r.Body).Decode(&fb); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	if fb.Query == "" || fb.CorrectedResponse == "" {
		http.Error(w, `{"error":"query and corrected_response are required"}`, http.StatusBadRequest)
		return
	}

	if err := s.feedback.RecordCorrection(fb.Query, fb.OriginalResponse, fb.CorrectedResponse, fb.Tags); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed saving feedback: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Feedback recorded into encrypted vault and applied to learning engine.",
	})
}

func (s *Server) handleVision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req vision.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	res, err := s.vision.Analyze(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req tasks.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	res, err := s.tasks.Execute(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	docs, err := s.scanner.Scan()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"scan failed: %v"}`, err), http.StatusInternalServerError)
		return
	}

	s.indexer.Build(docs)
	s.mu.Lock()
	s.docs = docs
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "success",
		"scanned_notes": len(docs),
		"vault_dir":     s.config.VaultDir,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_documents": s.indexer.Count(),
		"vault_dir":       s.config.VaultDir,
		"corrections":     len(s.feedback.ListCorrections()),
	})
}
