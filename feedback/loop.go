package feedback

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/benzjeremy/local-ai-worker/storage"
)

// Engine manages feedback collection and dynamic prompt augmentation.
type Engine struct {
	mu    sync.RWMutex
	vault *storage.Vault
}

// NewEngine initializes the feedback learning engine.
func NewEngine(v *storage.Vault) *Engine {
	return &Engine{vault: v}
}

// RecordCorrection saves an explicit user or supervisor correction.
func (e *Engine) RecordCorrection(query, original, corrected string, tags []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := fmt.Sprintf("corr-%d", time.Now().UnixNano())
	rec := storage.FeedbackRecord{
		ID:                id,
		Query:             strings.TrimSpace(query),
		OriginalResponse:  strings.TrimSpace(original),
		CorrectedResponse: strings.TrimSpace(corrected),
		ContextTags:       tags,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
	}

	return e.vault.AddFeedback(rec)
}

// ListCorrections returns all stored corrections.
func (e *Engine) ListCorrections() []storage.FeedbackRecord {
	return e.vault.GetFeedback()
}

// GetRelevantCorrections finds previous corrections matching the query terms.
func (e *Engine) GetRelevantCorrections(query string, maxResults int) []storage.FeedbackRecord {
	all := e.vault.GetFeedback()
	if len(all) == 0 {
		return nil
	}

	qTokens := filterSignificantTokens(query)
	if len(qTokens) == 0 {
		return nil
	}

	type scoredRec struct {
		rec   storage.FeedbackRecord
		score int
	}

	var scored []scoredRec
	for _, r := range all {
		s := 0
		rText := strings.ToLower(r.Query + " " + strings.Join(r.ContextTags, " "))
		for _, token := range qTokens {
			if strings.Contains(rText, token) {
				s++
			}
		}
		if s > 0 {
			scored = append(scored, scoredRec{rec: r, score: s})
		}
	}

	var res []storage.FeedbackRecord
	for i := 0; i < len(scored) && i < maxResults; i++ {
		res = append(res, scored[i].rec)
	}

	return res
}

// BuildPromptContext creates a system instruction snippet containing learned supervisor corrections.
func (e *Engine) BuildPromptContext(query string) string {
	relevant := e.GetRelevantCorrections(query, 3)
	if len(relevant) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n### 🚨 BINDING SUPERVISOR RULES & LEARNED CORRECTIONS:\n")
	sb.WriteString("The following past responses were explicitly corrected by the supervisor. You MUST follow these corrections strictly:\n")

	for i, r := range relevant {
		sb.WriteString(fmt.Sprintf("%d. For query topic: \"%s\"\n", i+1, r.Query))
		sb.WriteString(fmt.Sprintf("   - DO NOT say: \"%s\"\n", r.OriginalResponse))
		sb.WriteString(fmt.Sprintf("   - MUST say / adhere to: \"%s\"\n", r.CorrectedResponse))
	}
	sb.WriteString("### END OF SUPERVISOR CORRECTIONS\n\n")

	return sb.String()
}

var commonStopWords = map[string]struct{}{
	"wie": {}, "was": {}, "wer": {}, "wo": {}, "wann": {}, "warum": {}, "weshalb": {},
	"der": {}, "die": {}, "das": {}, "ein": {}, "eine": {}, "eines": {}, "einem": {}, "einen": {},
	"und": {}, "oder": {}, "aber": {}, "denn": {}, "mit": {}, "von": {}, "für": {}, "fuer": {},
	"ist": {}, "sind": {}, "war": {}, "waren": {}, "wird": {}, "werden": {}, "haben": {}, "hat": {},
	"man": {}, "wir": {}, "ihr": {}, "sie": {}, "ich": {}, "du": {}, "er": {}, "es": {},
	"how": {}, "what": {}, "who": {}, "where": {}, "when": {}, "why": {}, "the": {}, "and": {},
}

func filterSignificantTokens(s string) []string {
	words := strings.Fields(strings.ToLower(s))
	var clean []string
	for _, w := range words {
		w = strings.Trim(w, ",.!?\"'();:-_")
		if len(w) <= 2 {
			continue
		}
		if _, isStop := commonStopWords[w]; isStop {
			continue
		}
		clean = append(clean, w)
	}
	return clean
}
