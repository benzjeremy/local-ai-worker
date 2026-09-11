package knowledge

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	tagRegex     = regexp.MustCompile(`#([a-zA-Z0-9_\-\/]+)`)
	wikiRegex    = regexp.MustCompile(`\[\[([^\]\|]+)(?:\|([^\]]+))?\]\]`)
	headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
)

// Document represents an Obsidian Vault Markdown file with structured metadata.
type Document struct {
	RelPath   string    `json:"rel_path"`
	AbsPath   string    `json:"abs_path"`
	Title     string    `json:"title"`
	LevelTag  string    `json:"level_tag"` // #level-root, #level-hub, #level-category, #level-leaf
	Tags      []string  `json:"tags"`
	WikiLinks []string  `json:"wiki_links"`
	Headings  []string  `json:"headings"`
	Content   string    `json:"content"`
	CleanText string    `json:"clean_text"`
	WordCount int       `json:"word_count"`
	ModTime   time.Time `json:"mod_time"`
}

// ParseMarkdown parses raw Markdown text into a Document representation.
func ParseMarkdown(absPath, relPath, content string, modTime time.Time) *Document {
	doc := &Document{
		AbsPath:   absPath,
		RelPath:   relPath,
		Content:   content,
		ModTime:   modTime,
		Tags:      make([]string, 0),
		WikiLinks: make([]string, 0),
		Headings:  make([]string, 0),
	}

	// Fallback title is basename without extension
	doc.Title = strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))

	lines := strings.Split(content, "\n")
	var cleanLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Heading extraction
		if matches := headingRegex.FindStringSubmatch(trimmed); len(matches) == 3 {
			headingText := strings.TrimSpace(matches[2])
			doc.Headings = append(doc.Headings, headingText)
			if doc.Title == "" || doc.Title == strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath)) {
				doc.Title = headingText
			}
		}

		// Tags extraction
		if tagMatches := tagRegex.FindAllStringSubmatch(trimmed, -1); len(tagMatches) > 0 {
			for _, tm := range tagMatches {
				tag := tm[1]
				doc.Tags = append(doc.Tags, tag)
				if strings.HasPrefix(tag, "level-") {
					doc.LevelTag = "#" + tag
				}
			}
		}

		// WikiLinks extraction
		if linkMatches := wikiRegex.FindAllStringSubmatch(trimmed, -1); len(linkMatches) > 0 {
			for _, lm := range linkMatches {
				doc.WikiLinks = append(doc.WikiLinks, lm[1])
			}
		}

		// Strip Markdown syntax for clean fulltext
		cleanLine := stripMarkdownSyntax(trimmed)
		if cleanLine != "" {
			cleanLines = append(cleanLines, cleanLine)
		}
	}

	doc.CleanText = strings.Join(cleanLines, " ")
	words := strings.Fields(doc.CleanText)
	doc.WordCount = len(words)

	return doc
}

func stripMarkdownSyntax(s string) string {
	s = headingRegex.ReplaceAllString(s, "$2")
	s = wikiRegex.ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`).ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "---", "")
	return strings.TrimSpace(s)
}
