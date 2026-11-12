package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRetrieverKeepsTextPairedAfterEmbeddingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Prompt string `json:"prompt"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Prompt == "second" {
			http.Error(w, "embedding failed", 500)
			return
		}
		vec := []float64{1, 0}
		if in.Prompt == "third" {
			vec = []float64{0, 1}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vec})
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "knowledge.txt")
	if err := os.WriteFile(path, []byte("first\n\nsecond\n\nthird"), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewRetriever(RAGConfig{Enabled: true, StorePath: path, OllamaEndpoint: server.URL})
	if r == nil || len(r.docs) != 2 || r.docs[1] != "third" || r.vecs[1][1] != 1 {
		t.Fatal("embedding paired with the wrong source text")
	}
}
