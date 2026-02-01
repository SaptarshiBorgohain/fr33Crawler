package search

import (
	"math"
	"sort"
)

// SearchResult holds a matching document and its score
type SearchResult struct {
	Document *Document
	Score    float64
}

// VectorStore holds the indexed documents
type VectorStore struct {
	Documents  []*Document
	Vectorizer Vectorizer
}

func NewVectorStore() *VectorStore {
	return &VectorStore{
		Documents:  make([]*Document, 0),
		Vectorizer: NewTFIDFVectorizer(),
	}
}

// AddDocuments trains the vectorizer and indexes the documents
func (vs *VectorStore) AddDocuments(docs []*Document) {
	// 1. Extract raw text for training
	rawTexts := make([]string, len(docs))
	for i, d := range docs {
		rawTexts[i] = d.Content
	}

	// 2. Fit the vectorizer
	vs.Vectorizer.Fit(rawTexts)

	// 3. Vectorize all documents
	for _, d := range docs {
		d.Vector = vs.Vectorizer.Transform(d.Content)
		vs.Documents = append(vs.Documents, d)
	}
}

// Search finds the most similar documents to the query
func (vs *VectorStore) Search(query string, topK int) []SearchResult {
	queryVector := vs.Vectorizer.Transform(query)
	var results []SearchResult

	for _, doc := range vs.Documents {
		score := CosineSimilarity(queryVector, doc.Vector)
		if score > 0 {
			results = append(results, SearchResult{
				Document: doc,
				Score:    score,
			})
		}
	}

	// Sort by descending score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		return results[:topK]
	}
	return results
}

// CosineSimilarity calculates the cosine similarity between two vectors
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
