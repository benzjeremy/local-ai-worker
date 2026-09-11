package knowledge

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// SearchResult contains a matched document with relevance score and highlight snippets.
type SearchResult struct {
	Document *Document `json:"document"`
	Score    float64   `json:"score"`
	Snippets []string  `json:"snippets"`
}

// Indexer stores documents and provides fast Okapi BM25 retrieval.
type Indexer struct {
	mu            sync.RWMutex
	docs          []*Document
	docTermFreqs  []map[string]int
	docLengths    []int
	avgDocLength  float64
	termDocFreq   map[string]int
	stopWords     map[string]struct{}
}

// NewIndexer creates an initialized BM25 indexer.
func NewIndexer() *Indexer {
	idx := &Indexer{
		docTermFreqs: make([]map[string]int, 0),
		docLengths:   make([]int, 0),
		termDocFreq:  make(map[string]int),
		stopWords:    defaultStopWords(),
	}
	return idx
}

// Build indexes a set of documents.
func (idx *Indexer) Build(docs []*Document) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.docs = docs
	n := len(docs)
	idx.docTermFreqs = make([]map[string]int, n)
	idx.docLengths = make([]int, n)
	idx.termDocFreq = make(map[string]int)

	totalLength := 0

	for i, doc := range docs {
		tokens := idx.tokenize(doc.CleanText + " " + doc.Title + " " + strings.Join(doc.Tags, " "))
		freqs := make(map[string]int)
		for _, t := range tokens {
			freqs[t]++
		}
		idx.docTermFreqs[i] = freqs
		idx.docLengths[i] = len(tokens)
		totalLength += len(tokens)

		for term := range freqs {
			idx.termDocFreq[term]++
		}
	}

	if n > 0 {
		idx.avgDocLength = float64(totalLength) / float64(n)
	} else {
		idx.avgDocLength = 0
	}
}

// Search evaluates query against all indexed documents using Okapi BM25.
func (idx *Indexer) Search(query string, topK int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tokens := idx.tokenize(query)
	if len(tokens) == 0 || len(idx.docs) == 0 {
		return nil
	}

	n := float64(len(idx.docs))
	type scoredDoc struct {
		index int
		score float64
	}

	scores := make([]scoredDoc, len(idx.docs))

	for i, doc := range idx.docs {
		var score float64
		freqs := idx.docTermFreqs[i]
		docLen := float64(idx.docLengths[i])

		for _, term := range tokens {
			tf := float64(freqs[term])
			if tf == 0 {
				continue
			}

			df := float64(idx.termDocFreq[term])
			idf := math.Log((n-df+0.5)/(df+0.5) + 1.0)
			if idf < 0 {
				idf = 0
			}

			numerator := tf * (bm25K1 + 1.0)
			denominator := tf + bm25K1*(1.0-bm25B+bm25B*(docLen/idx.avgDocLength))
			score += idf * (numerator / denominator)

			// Boost if term is in title or tags
			if strings.Contains(strings.ToLower(doc.Title), term) {
				score += 2.5
			}
			for _, tag := range doc.Tags {
				if strings.Contains(strings.ToLower(tag), term) {
					score += 3.0
				}
			}
		}

		scores[i] = scoredDoc{index: i, score: score}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	var results []SearchResult
	for i := 0; i < len(scores) && i < topK; i++ {
		if scores[i].score <= 0 {
			break
		}
		doc := idx.docs[scores[i].index]
		snippets := idx.extractSnippets(doc.Content, tokens, 3)
		results = append(results, SearchResult{
			Document: doc,
			Score:    scores[i].score,
			Snippets: snippets,
		})
	}

	return results
}

func (idx *Indexer) extractSnippets(content string, queryTerms []string, maxSnippets int) []string {
	lines := strings.Split(content, "\n")
	var snippets []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 15 {
			continue
		}
		lowerLine := strings.ToLower(trimmed)
		matched := false
		for _, term := range queryTerms {
			if strings.Contains(lowerLine, term) {
				matched = true
				break
			}
		}
		if matched {
			if len(trimmed) > 160 {
				trimmed = trimmed[:160] + "..."
			}
			snippets = append(snippets, trimmed)
			if len(snippets) >= maxSnippets {
				break
			}
		}
	}

	return snippets
}

func (idx *Indexer) tokenize(text string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}

	words := strings.FieldsFunc(strings.ToLower(text), f)
	var tokens []string

	for _, w := range words {
		if len(w) <= 1 {
			continue
		}
		if _, isStop := idx.stopWords[w]; isStop {
			continue
		}
		tokens = append(tokens, w)
	}

	return tokens
}

// Count returns number of indexed documents.
func (idx *Indexer) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.docs)
}

func defaultStopWords() map[string]struct{} {
	stops := []string{
		"und", "der", "die", "das", "ein", "eine", "ist", "sind", "mit", "fuer", "für", "von", "den", "dem", "des",
		"im", "in", "am", "an", "auf", "aus", "bei", "zu", "zur", "zum", "als", "wie", "so", "dass", "da",
		"the", "a", "an", "and", "or", "of", "to", "in", "is", "it", "that", "this", "for", "with", "on", "at",
	}
	m := make(map[string]struct{}, len(stops))
	for _, s := range stops {
		m[s] = struct{}{}
	}
	return m
}
