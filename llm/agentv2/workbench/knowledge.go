package workbench

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

func LoadKnowledge(path string) ([]Chunk, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
	if err = json.Unmarshal(b, &chunks); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for i := range chunks {
		c := &chunks[i]
		if c.ID == "" || seen[c.ID] || c.Source == "" || c.Text == "" || c.Status != "reviewed" {
			return nil, fmt.Errorf("invalid knowledge chunk %d", i)
		}
		seen[c.ID] = true
		c.Version = fingerprint(struct{ Text, Source, Version string }{c.Text, c.Source, c.Version})
	}
	return chunks, nil
}

// Chinese bigrams and Latin terms avoid depending on a tokenizer or a GPU.
// This is lexical RAG; scores express relevance, never factual confidence.
func terms(s string) map[string]bool {
	out := map[string]bool{}
	r := []rune(strings.ToLower(s))
	for i, c := range r {
		if unicode.Is(unicode.Han, c) && i+1 < len(r) && unicode.Is(unicode.Han, r[i+1]) {
			out[string(r[i:i+2])] = true
		}
	}
	for _, s := range strings.FieldsFunc(strings.ToLower(s), func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) || unicode.Is(unicode.Han, c) }) {
		if len(s) > 1 {
			out[s] = true
		}
	}
	return out
}

func sentences(text string) []string {
	out := []string{}
	var part strings.Builder
	for _, r := range text {
		part.WriteRune(r)
		if strings.ContainsRune("。！？\n", r) {
			if s := strings.TrimSpace(part.String()); s != "" {
				out = append(out, s)
			}
			part.Reset()
		}
	}
	if s := strings.TrimSpace(part.String()); s != "" {
		out = append(out, s)
	}
	return out
}
func applicableChunk(c Chunk, workspace, crop string) bool {
	return c.Status == "reviewed" && (c.WorkspaceID == "" || c.WorkspaceID == workspace) && (c.Crop == "" || c.Crop == crop)
}

func retrieve(all []Chunk, question, workspace, crop string) []Chunk {
	q := terms(question)
	out := []Chunk{}
	for _, c := range all {
		if !applicableChunk(c, workspace, crop) {
			continue
		}
		t := terms(c.Title + " " + c.Text)
		hits := 0
		for term := range q {
			if t[term] {
				hits++
			}
		}
		required := 2
		if len(q) < required {
			required = len(q)
		}
		if hits < required || hits == 0 {
			continue
		}
		c.Score = float64(hits) / float64(len(q))
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > 0 {
		cutoff := out[0].Score * 0.6
		filtered := []Chunk{}
		for _, c := range out {
			if c.Score >= cutoff {
				filtered = append(filtered, c)
			}
		}
		out = filtered
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}
