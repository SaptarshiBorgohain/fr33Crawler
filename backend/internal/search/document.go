package search

import (
	"strings"
	"unicode"
)

// Document represents a searchable item
type Document struct {
	ID      string
	Content string
	Title   string // Metadata
	Vector  []float64
}

// Tokenize splits text into normalized tokens (lowercase words)
func Tokenize(text string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	fields := strings.FieldsFunc(text, f)
	var tokens []string
	for _, field := range fields {
		if len(field) > 2 { // Skip very short words
			tokens = append(tokens, strings.ToLower(field))
		}
	}
	return tokens
}
