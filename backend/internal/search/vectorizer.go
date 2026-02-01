package search

import (
	"math"
)

// Vectorizer turns text into a vector
type Vectorizer interface {
	Fit(docs []string)
	Transform(text string) []float64
}

// TFIDFVectorizer implements Term Frequency - Inverse Document Frequency
type TFIDFVectorizer struct {
	Vocabulary map[string]int
	IDF        map[string]float64
}

func NewTFIDFVectorizer() *TFIDFVectorizer {
	return &TFIDFVectorizer{
		Vocabulary: make(map[string]int),
		IDF:        make(map[string]float64),
	}
}

// Fit analyzes the corpus to build vocabulary and IDF stats
func (v *TFIDFVectorizer) Fit(docs []string) {
	docCount := float64(len(docs))
	wordDocCounts := make(map[string]int)

	// 1. Build Vocabulary and count document occurrences
	for _, doc := range docs {
		tokens := Tokenize(doc)
		seenInDoc := make(map[string]bool)
		for _, token := range tokens {
			if !seenInDoc[token] {
				wordDocCounts[token]++
				seenInDoc[token] = true
			}
			if _, exists := v.Vocabulary[token]; !exists {
				v.Vocabulary[token] = len(v.Vocabulary)
			}
		}
	}

	// 2. Calculate IDF
	for word, count := range wordDocCounts {
		// idf = log(N / (df + 1)) + 1
		v.IDF[word] = math.Log(docCount/(float64(count)+1)) + 1
	}
}

// Transform converts text to a vector based on the learned vocabulary
func (v *TFIDFVectorizer) Transform(text string) []float64 {
	vector := make([]float64, len(v.Vocabulary))
	tokens := Tokenize(text)
	
	// Calculate Term Frequency (TF)
	tf := make(map[string]float64)
	for _, token := range tokens {
		tf[token]++
	}

	// Calculate TF-IDF
	for token, count := range tf {
		if idx, exists := v.Vocabulary[token]; exists {
			idf := v.IDF[token]
			// using basic tf * idf
			vector[idx] = (count / float64(len(tokens))) * idf
		}
	}
	
	return vector
}
