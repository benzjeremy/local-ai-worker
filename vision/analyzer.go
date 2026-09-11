package vision

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benzjeremy/local-ai-worker/client"
)

// AnalysisType defines the domain of inspection.
type AnalysisType string

const (
	TypeGeneralScreenshot AnalysisType = "general"
	TypeCADDrawing        AnalysisType = "cad_drawing"
	TypeArchitectureChart AnalysisType = "architecture_chart"
	TypeUIDesign          AnalysisType = "ui_design"
)

// Request defines vision inspection parameters.
type Request struct {
	Type        AnalysisType `json:"type"`
	Prompt      string       `json:"prompt"`
	Base64Image string       `json:"base64_image,omitempty"`
	FilePath    string       `json:"file_path,omitempty"`
	Model       string       `json:"model,omitempty"`
}

// Result holds the analyzed visual insight.
type Result struct {
	Analysis string `json:"analysis"`
	Model    string `json:"model"`
	Type     string `json:"type"`
}

// Analyzer orchestrates vision requests via local Ollama.
type Analyzer struct {
	ollama       *client.OllamaClient
	defaultModel string
}

// NewAnalyzer creates a vision analyzer.
func NewAnalyzer(c *client.OllamaClient, defaultModel string) *Analyzer {
	if defaultModel == "" {
		defaultModel = "llava:7b"
	}
	return &Analyzer{
		ollama:       c,
		defaultModel: defaultModel,
	}
}

// Analyze processes an image through the vision model.
func (a *Analyzer) Analyze(ctx context.Context, req Request) (*Result, error) {
	var b64Data string

	if req.Base64Image != "" {
		b64Data = cleanBase64(req.Base64Image)
	} else if req.FilePath != "" {
		data, err := os.ReadFile(req.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed reading image file: %w", err)
		}
		b64Data = base64.StdEncoding.EncodeToString(data)
	} else {
		return nil, errors.New("must provide either base64_image or file_path")
	}

	model := req.Model
	if model == "" {
		model = a.defaultModel
	}

	systemPrompt := a.buildSystemPrompt(req.Type)
	prompt := req.Prompt
	if prompt == "" {
		prompt = "Please analyze this image thoroughly and report key visual elements, measurements or text."
	}

	genReq := client.GenerateRequest{
		Model:  model,
		System: systemPrompt,
		Prompt: prompt,
		Images: []string{b64Data},
	}

	resp, err := a.ollama.Generate(ctx, genReq)
	if err != nil {
		return nil, fmt.Errorf("vision analysis failed: %w", err)
	}

	return &Result{
		Analysis: resp,
		Model:    model,
		Type:     string(req.Type),
	}, nil
}

func (a *Analyzer) buildSystemPrompt(t AnalysisType) string {
	switch t {
	case TypeCADDrawing:
		return "You are an expert CAD engineer. Inspect the technical drawing, dimension lines, tolerances, materials and annotations with engineering precision."
	case TypeArchitectureChart:
		return "You are a software architect. Inspect this architectural diagram, identify components, services, database layers, communication protocols and directional flows."
	case TypeUIDesign:
		return "You are a senior UI/UX designer. Inspect this interface mockup, identifying color palettes, typography, responsive layout hierarchy and accessibility contrast."
	default:
		return "You are a precise computer vision assistant. Describe what you see accurately, extracting any visible text or symbols."
	}
}

func cleanBase64(s string) string {
	// Strip data URI prefix if present (e.g. data:image/png;base64,)
	if idx := strings.Index(s, ","); idx != -1 && strings.HasPrefix(s, "data:") {
		return s[idx+1:]
	}
	return s
}

// SupportedExtensions lists valid visual formats.
func SupportedExtensions() []string {
	return []string{".png", ".jpg", ".jpeg", ".webp"}
}

// IsImageExtension checks if a path points to an image.
func IsImageExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, supported := range SupportedExtensions() {
		if ext == supported {
			return true
		}
	}
	return false
}
