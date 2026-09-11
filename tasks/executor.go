package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/benzjeremy/local-ai-worker/client"
	"github.com/benzjeremy/local-ai-worker/feedback"
	"github.com/benzjeremy/local-ai-worker/knowledge"
)

// TaskType represents supported assistant workflows.
type TaskType string

const (
	TaskSummarize       TaskType = "summarize"
	TaskEmailDraft      TaskType = "email_draft"
	TaskCADSpecExtract  TaskType = "cad_spec_extract"
	TaskVaultAudit      TaskType = "vault_audit"
)

// TaskRequest encapsulates input for a predefined task.
type TaskRequest struct {
	Type        TaskType `json:"type"`
	InputText   string   `json:"input_text,omitempty"`
	VaultTarget string   `json:"vault_target,omitempty"` // Note title or query
	Model       string   `json:"model,omitempty"`
}

// TaskResult contains the output of the executed task.
type TaskResult struct {
	Type     string            `json:"type"`
	Output   string            `json:"output"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Executor coordinates specialized assistance tasks.
type Executor struct {
	ollama   *client.OllamaClient
	indexer  *knowledge.Indexer
	feedback *feedback.Engine
	model    string
}

// NewExecutor creates a new task executor.
func NewExecutor(c *client.OllamaClient, idx *knowledge.Indexer, fb *feedback.Engine, defaultModel string) *Executor {
	if defaultModel == "" {
		defaultModel = "llama3.1:8b"
	}
	return &Executor{
		ollama:   c,
		indexer:  idx,
		feedback: fb,
		model:    defaultModel,
	}
}

// Execute processes a predefined assistant task.
func (e *Executor) Execute(ctx context.Context, req TaskRequest) (*TaskResult, error) {
	model := req.Model
	if model == "" {
		model = e.model
	}

	switch req.Type {
	case TaskVaultAudit:
		return e.runVaultAudit()

	case TaskSummarize:
		return e.runSummarize(ctx, req, model)

	case TaskEmailDraft:
		return e.runEmailDraft(ctx, req, model)

	case TaskCADSpecExtract:
		return e.runCADSpecExtract(ctx, req, model)

	default:
		return nil, fmt.Errorf("unknown task type: %s", req.Type)
	}
}

func (e *Executor) runVaultAudit() (*TaskResult, error) {
	results := e.indexer.Search("", 1000)
	total := e.indexer.Count()

	// Analyze all indexed documents via search
	// We can search for generic terms or query indexer directly
	allDocs := e.indexer.Search("level root hub category leaf", 1000)
	
	missingLevelTags := 0
	leafCount := 0
	categoryCount := 0
	hubCount := 0
	rootCount := 0

	for _, r := range allDocs {
		doc := r.Document
		switch doc.LevelTag {
		case "#level-root":
			rootCount++
		case "#level-hub":
			hubCount++
		case "#level-category":
			categoryCount++
		case "#level-leaf":
			leafCount++
		default:
			missingLevelTags++
		}
	}

	report := fmt.Sprintf("Obsidian Vault Audit Report:\n"+
		"- Total Indexed Notes: %d\n"+
		"- Root Nodes (#level-root): %d\n"+
		"- Hub Nodes (#level-hub): %d\n"+
		"- Category Nodes (#level-category): %d\n"+
		"- Leaf Nodes (#level-leaf): %d\n"+
		"- Notes Missing Level Tag: %d\n",
		total, rootCount, hubCount, categoryCount, leafCount, missingLevelTags)

	if missingLevelTags == 0 {
		report += "Status: 🟢 PERFECT GRAPH HIERARCHY (Mindmap radial tree compliant)."
	} else {
		report += "Status: 🟡 COMPLIANCE NOTICE: Add explicit #level-... tags to unclassified notes."
	}

	_ = results
	return &TaskResult{
		Type:   string(TaskVaultAudit),
		Output: report,
		Metadata: map[string]string{
			"total_notes": fmt.Sprintf("%d", total),
		},
	}, nil
}

func (e *Executor) runSummarize(ctx context.Context, req TaskRequest, model string) (*TaskResult, error) {
	contextText := req.InputText

	if req.VaultTarget != "" {
		matches := e.indexer.Search(req.VaultTarget, 3)
		if len(matches) > 0 {
			var sb strings.Builder
			for _, m := range matches {
				sb.WriteString(fmt.Sprintf("\n--- Note: %s ---\n%s\n", m.Document.Title, m.Document.CleanText))
			}
			contextText = sb.String() + "\n" + req.InputText
		}
	}

	fbContext := e.feedback.BuildPromptContext("summarize " + req.VaultTarget)
	prompt := fmt.Sprintf("Please provide a concise, high-impact technical summary with key bullets of the following material:%s\n\n%s", fbContext, contextText)

	resp, err := e.ollama.Generate(ctx, client.GenerateRequest{
		Model:  model,
		System: "You are a concise executive assistant. Summarize technical content accurately without corporate fluff.",
		Prompt: prompt,
	})
	if err != nil {
		return nil, err
	}

	return &TaskResult{
		Type:   string(TaskSummarize),
		Output: resp,
	}, nil
}

func (e *Executor) runEmailDraft(ctx context.Context, req TaskRequest, model string) (*TaskResult, error) {
	contextText := req.InputText
	if req.VaultTarget != "" {
		matches := e.indexer.Search(req.VaultTarget, 2)
		if len(matches) > 0 {
			contextText = fmt.Sprintf("Context from Vault (%s):\n%s\n\nRequest:\n%s", matches[0].Document.Title, matches[0].Document.CleanText, req.InputText)
		}
	}

	fbContext := e.feedback.BuildPromptContext("email draft " + req.VaultTarget)
	prompt := fmt.Sprintf("Draft a crisp, professional and polite email based on the following instruction:%s\n\n%s", fbContext, contextText)

	resp, err := e.ollama.Generate(ctx, client.GenerateRequest{
		Model:  model,
		System: "You are a professional assistant writing in the authentic, respectful and precise tone of Jeremy Benz (benzjeremy@pm.me).",
		Prompt: prompt,
	})
	if err != nil {
		return nil, err
	}

	return &TaskResult{
		Type:   string(TaskEmailDraft),
		Output: resp,
	}, nil
}

func (e *Executor) runCADSpecExtract(ctx context.Context, req TaskRequest, model string) (*TaskResult, error) {
	fbContext := e.feedback.BuildPromptContext("cad spec extraction")
	prompt := fmt.Sprintf("Extract all technical specifications, dimensional tolerances, material types, surface finish requirements and part IDs as structured JSON from this input:%s\n\n%s", fbContext, req.InputText)

	resp, err := e.ollama.Generate(ctx, client.GenerateRequest{
		Model:  model,
		System: "You are a mechanical engineering parser. Extract technical specifications into clean structured key-value pairs.",
		Prompt: prompt,
	})
	if err != nil {
		return nil, err
	}

	return &TaskResult{
		Type:   string(TaskCADSpecExtract),
		Output: resp,
	}, nil
}
