package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type Ollama struct {
	URL, Name     string
	Python, Guard string
	Client        *http.Client
}

const (
	modelContextTokens   = 216244
	modelOutputTokens    = 14000
	thinkingOutputTokens = 28000
)

func (m *Ollama) Generate(ctx context.Context, messages []Message, schema any, dst any) error {
	return m.generate(ctx, messages, schema, dst, nil, nil)
}

func (m *Ollama) generate(ctx context.Context, messages []Message, schema any, dst any, onContent, onThinking func(string) error) error {
	if m.Guard != "" {
		guard, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := exec.CommandContext(guard, m.Python, m.Guard, "--check-only").Run()
		cancel()
		if err != nil {
			return fmt.Errorf("GPU is busy or idle status cannot be verified; retry later")
		}
	}
	name := m.Name
	if name == "" {
		name = DefaultModel
	}
	stream := onContent != nil || onThinking != nil
	think := onThinking != nil
	numPredict, numContext := modelOutputTokens, modelContextTokens
	if onThinking != nil {
		numPredict = thinkingOutputTokens
	}
	if _, planning := dst.(*queryPlan); planning {
		numPredict, numContext = 3072, 16384
	}
	if _, planning := dst.(*goalOutline); planning {
		numPredict, numContext = 768, 8192
	}
	if _, classifying := dst.(*speechAct); classifying {
		numPredict, numContext = 512, 8192
	}
	if _, planning := dst.(*CameraIntent); planning {
		numPredict, numContext = 3072, 16384
	}
	if _, planning := dst.(*DeviceRequest); planning {
		numPredict, numContext = 4096, 16384
	}
	if _, planning := dst.(*ObservationRequest); planning {
		numPredict, numContext = 1536, 16384
	}
	if _, planning := dst.(*PhotoRequest); planning {
		numPredict, numContext = 1024, 16384
	}
	if _, review := dst.(*deviceIntentReview); review {
		numPredict, numContext = 768, 16384
	}
	if _, matching := dst.(*ExecutionMatch); matching {
		numPredict, numContext = 768, 8192
	}
	if _, analysis := dst.(*qaAnalysis); analysis {
		numPredict, numContext = 4096, 32768
	}
	if _, review := dst.(*qaReview); review {
		numPredict, numContext = 2048, 32768
	}
	body, err := json.Marshal(map[string]any{"model": name, "messages": messages, "stream": stream, "think": think, "format": schema, "keep_alive": 0, "options": map[string]any{"num_ctx": numContext, "num_batch": 128, "num_predict": numPredict, "temperature": 0.1}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(m.URL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 300 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Ollama returned HTTP %d; ensure the configured model is installed", resp.StatusCode)
	}
	responseLimit := int64(2 << 20)
	if stream {
		responseLimit = 16 << 20 // Includes the per-chunk NDJSON envelope.
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, responseLimit))
	var content strings.Builder
	for {
		var result struct {
			Message struct {
				Content  string `json:"content"`
				Thinking string `json:"thinking"`
			} `json:"message"`
			Done       bool   `json:"done"`
			DoneReason string `json:"done_reason"`
			Error      string `json:"error"`
		}
		if err = decoder.Decode(&result); err != nil {
			return fmt.Errorf("incomplete model response: %w", err)
		}
		if result.Error != "" {
			return fmt.Errorf("Ollama: %s", result.Error)
		}
		if onThinking != nil && result.Message.Thinking != "" {
			if err := onThinking(result.Message.Thinking); err != nil {
				return err
			}
		}
		content.WriteString(result.Message.Content)
		if onContent != nil && result.Message.Content != "" {
			if err := onContent(content.String()); err != nil {
				return err
			}
		}
		if result.Done {
			if result.DoneReason == "length" {
				return fmt.Errorf("incomplete model response")
			}
			break
		}
		if !stream {
			return fmt.Errorf("incomplete model response")
		}
	}
	d := json.NewDecoder(strings.NewReader(content.String()))
	d.DisallowUnknownFields()
	if err = d.Decode(dst); err != nil {
		return fmt.Errorf("invalid structured model output: %w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("trailing model output")
	}
	return nil
}

var answerSchema = json.RawMessage(`{"type":"object","properties":{"claims":{"type":"array","maxItems":8,"items":{"type":"object","properties":{"text":{"type":"string"},"kind":{"type":"string","enum":["observation","knowledge","common","image"]},"citations":{"type":"array","items":{"type":"string"}}},"required":["text","kind","citations"],"additionalProperties":false}},"limitations":{"type":"array","items":{"type":"string"}}},"required":["claims","limitations"],"additionalProperties":false}`)
var reviewSchema = json.RawMessage(`{"type":"object","properties":{"approved":{"type":"boolean"},"issues":{"type":"array","items":{"type":"string"}}},"required":["approved","issues"],"additionalProperties":false}`)

// Constrain citations to the actual evidence snapshot. The common-knowledge
// branch has an empty citation list; every other branch requires a real ID.
func schemaFor(r Run) any {
	var schema map[string]any
	_ = json.Unmarshal(answerSchema, &schema)
	variants := []any{}
	variant := func(kind string, ids []string) {
		citations := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
		if kind == "common" {
			citations["maxItems"] = 0
		} else {
			if len(ids) == 0 {
				return
			}
			citations["minItems"] = 1
			citations["items"] = map[string]any{"type": "string", "enum": ids}
		}
		variants = append(variants, map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string", "const": kind}, "citations": citations}, "required": []string{"text", "kind", "citations"}, "additionalProperties": false})
	}
	for _, c := range r.Knowledge {
		variants = append(variants, map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string", "enum": sentences(c.Text)}, "kind": map[string]any{"type": "string", "const": "knowledge"}, "citations": map[string]any{"type": "array", "minItems": 1, "maxItems": 1, "items": map[string]any{"type": "string", "enum": []string{c.ID}}}}, "required": []string{"text", "kind", "citations"}, "additionalProperties": false})
	}
	ids := []string{}
	if r.Evidence != nil && len(r.QueryResults) == 0 {
		for _, m := range r.Evidence.Metrics {
			if m.Admission != "hold_for_validation" {
				ids = append(ids, m.ID)
			}
		}
	}
	variant("observation", ids)
	if r.ImageHash != "" {
		variant("image", []string{r.ImageHash})
	}
	// A retrieval hit may be incidental. Keep general knowledge available
	// alongside source quotes, with separate validation for each claim kind.
	if r.Task != "image" && r.Workflow == nil && len(r.QueryResults) == 0 {
		variant("common", nil)
	}
	if r.Task == "qa" && len(r.QueryResults) == 0 {
		variants = append(variants, map[string]any{
			"type": "object", "required": []string{"text", "kind", "citations"}, "additionalProperties": false,
			"properties": map[string]any{
				"text":      map[string]any{"type": "string", "const": "无法可靠回答"},
				"kind":      map[string]any{"type": "string", "const": "unanswered"},
				"citations": map[string]any{"type": "array", "maxItems": 0},
			},
		})
	}
	claims := schema["properties"].(map[string]any)["claims"].(map[string]any)
	claims["minItems"] = 1
	if len(r.QueryResults) > 0 {
		claims["minItems"] = 0
	}
	claims["items"] = map[string]any{"anyOf": variants}
	if r.Task == "image" {
		properties := schema["properties"].(map[string]any)
		properties["image_observations"] = map[string]any{"type": "array", "minItems": 1, "maxItems": 4, "items": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string", "const": "image"}, "citations": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []string{r.ImageHash}}}}, "required": []string{"text", "kind", "citations"}, "additionalProperties": false}}
		claims["minItems"] = 0
		claims["maxItems"] = 4
		schema["required"] = []string{"claims", "limitations", "image_observations"}
	}
	return schema
}
