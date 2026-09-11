package knowledge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Scanner scans a directory for Obsidian Markdown notes.
type Scanner struct {
	rootDir string
}

// NewScanner creates a new vault scanner.
func NewScanner(rootDir string) *Scanner {
	return &Scanner{rootDir: rootDir}
}

// Scan traverses the root directory and parses all Markdown files.
func (s *Scanner) Scan() ([]*Document, error) {
	var docs []*Document

	if _, err := os.Stat(s.rootDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("vault directory does not exist: %s", s.rootDir)
	}

	err := filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}

		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			relPath = path
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable file
		}

		doc := ParseMarkdown(path, relPath, string(contentBytes), info.ModTime())
		docs = append(docs, doc)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed scanning vault: %w", err)
	}

	return docs, nil
}
