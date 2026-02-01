package search_test

import (
	"math"
	"testing"
	"github.com/knowledge-engine/backend/internal/search"
)

func TestTokenize(t *testing.T) {
	text := "Hello, World! This is a test."
	tokens := search.Tokenize(text)
	
	expected := []string{"hello", "world", "this", "test"}
	
	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}
	
	for i, token := range tokens {
		if token != expected[i] {
			t.Errorf("At index %d: expected %s, got %s", i, expected[i], token)
		}
	}
}

func TestTFIDFVectorizer(t *testing.T) {
	docs := []string{
		"apple banana",
		"apple orange",
	}
	
	vectorizer := search.NewTFIDFVectorizer()
	vectorizer.Fit(docs)
	
	// Check vocabulary
	if len(vectorizer.Vocabulary) != 3 {
		t.Errorf("Expected vocabulary size 3 (apple, banana, orange), got %d", len(vectorizer.Vocabulary))
	}
	
	// 'apple' appears in both, 'banana' in one.
	// idf(apple) = log(2 / (2+1)) + 1 = log(0.66) + 1 ≈ 0.59
	// idf(banana) = log(2 / (1+1)) + 1 = log(1) + 1 = 1.0
	
	vec1 := vectorizer.Transform("apple banana")
	// Check that we got a vector
	if len(vec1) != 3 {
		t.Errorf("Expected vector length 3, got %d", len(vec1))
	}
}

func TestCosineSimilarity(t *testing.T) {
	vecA := []float64{1, 0, 1}
	vecB := []float64{0, 1, 1}
	
	// Dot product: 1*0 + 0*1 + 1*1 = 1
	// NormA: sqrt(1^2 + 0 + 1^2) = sqrt(2)
	// NormB: sqrt(0 + 1^2 + 1^2) = sqrt(2)
	// Cosine: 1 / (sqrt(2)*sqrt(2)) = 0.5
	
	score := search.CosineSimilarity(vecA, vecB)
	
	if math.Abs(score - 0.5) > 0.0001 {
		t.Errorf("Expected similarity 0.5, got %f", score)
	}
}

func TestVectorStore_Search(t *testing.T) {
	store := search.NewVectorStore()
	
	docs := []*search.Document{
		{ID: "doc1", Content: "go programming language"},
		{ID: "doc2", Content: "python programming language"},
		{ID: "doc3", Content: "banana fruit split"},
	}
	
	store.AddDocuments(docs)
	
	// Search for "go"
	results := store.Search("go language", 10)
	
	if len(results) == 0 {
		t.Fatal("Expected results, got none")
	}
	
	// doc1 should be first
	if results[0].Document.ID != "doc1" {
		t.Errorf("Expected top result to be doc1, got %s", results[0].Document.ID)
	}
	
	// Search for "python"
	results = store.Search("python", 10)
	if results[0].Document.ID != "doc2" {
		t.Errorf("Expected top result to be doc2, got %s", results[0].Document.ID)
	}
}
