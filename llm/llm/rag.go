package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// RAGConfig 控制检索开关、知识库与 embedding 模型。
type RAGConfig struct {
	Enabled        bool   `json:"enabled"`
	StorePath      string `json:"storePath"`      // 知识库文本，空行分段
	TopK           int    `json:"topK"`           // 返回段数
	OllamaEndpoint string `json:"ollamaEndpoint"` // 如 http://localhost:11434
	OllamaModel    string `json:"ollamaModel"`    // 如 nomic-embed-text
}

// Retriever 基于 Ollama Embedding + 余弦相似度的本地检索。
type Retriever struct {
	cfg  RAGConfig
	docs []string
	vecs [][]float64
	hc   *http.Client
}

// NewRetriever 构造检索器；配置不可用时返回 nil（不影响主流程）。
// 启动时为每个段落生成一次 embedding，存内存（语料大时可后续做缓存）。
func NewRetriever(cfg RAGConfig) *Retriever {
	if !cfg.Enabled {
		return nil
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 3
	}
	if strings.TrimSpace(cfg.StorePath) == "" {
		return nil
	}
	if cfg.OllamaEndpoint == "" {
		cfg.OllamaEndpoint = "http://localhost:11434"
	}
	if cfg.OllamaModel == "" {
		cfg.OllamaModel = "nomic-embed-text"
	}
	b, err := os.ReadFile(cfg.StorePath)
	if err != nil {
		return nil
	}
	docs := splitDocs(string(b))
	if len(docs) == 0 {
		return nil
	}
	r := &Retriever{
		cfg:  cfg,
		docs: docs,
		hc:   &http.Client{Timeout: 20 * time.Second},
	}
	for _, d := range docs {
		vec, err := r.embed(d)
		if err != nil || len(vec) == 0 {
			continue
		}
		r.vecs = append(r.vecs, vec)
	}
	// 若有失败的段落，保持长度对齐
	if len(r.vecs) != len(r.docs) {
		nd := make([]string, 0, len(r.vecs))
		for i := range r.vecs {
			nd = append(nd, r.docs[i])
		}
		r.docs = nd
	}
	if len(r.docs) == 0 {
		return nil
	}
	return r
}

// Retrieve 依据消息生成查询并返回 TopK 参考文本。
func (r *Retriever) Retrieve(messages []map[string]interface{}) []string {
	if r == nil || len(r.docs) == 0 {
		return nil
	}
	query := buildQuery(messages)
	if query == "" {
		return nil
	}
	qvec, err := r.embed(query)
	if err != nil || len(qvec) == 0 {
		return nil
	}
	type scored struct {
		idx   int
		score float64
	}
	scores := make([]scored, 0, len(r.docs))
	for i, dv := range r.vecs {
		s := cosine(qvec, dv)
		if s > 0 {
			scores = append(scores, scored{idx: i, score: s})
		}
	}
	if len(scores) == 0 {
		return nil
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].idx < scores[j].idx
		}
		return scores[i].score > scores[j].score
	})
	if len(scores) > r.cfg.TopK {
		scores = scores[:r.cfg.TopK]
	}
	out := make([]string, 0, len(scores))
	for _, s := range scores {
		out = append(out, r.docs[s.idx])
	}
	return out
}

// 调用 Ollama embeddings API
func (r *Retriever) embed(text string) ([]float64, error) {
	payload := map[string]interface{}{
		"model":  r.cfg.OllamaModel,
		"prompt": text,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", strings.TrimRight(r.cfg.OllamaEndpoint, "/")+"/api/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("embed status %d", resp.StatusCode)
	}
	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// buildQuery 基于精简后的消息字段拼查询串。
func buildQuery(messages []map[string]interface{}) string {
	if len(messages) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, m := range messages {
		if v, ok := m["partition_name"]; ok {
			sb.WriteString(fmt.Sprint(v))
			sb.WriteByte(' ')
		}
		if v, ok := m["name"]; ok {
			sb.WriteString(fmt.Sprint(v))
			sb.WriteByte(' ')
		}
		if v, ok := m["value"]; ok {
			sb.WriteString(fmt.Sprint(v))
			sb.WriteByte(' ')
		}
		if v, ok := m["unit"]; ok {
			sb.WriteString(fmt.Sprint(v))
			sb.WriteByte(' ')
		}
		if v, ok := m["string_value"]; ok {
			sb.WriteString(fmt.Sprint(v))
			sb.WriteByte(' ')
		}
	}
	return strings.TrimSpace(sb.String())
}

func splitDocs(s string) []string {
	parts := strings.Split(s, "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
