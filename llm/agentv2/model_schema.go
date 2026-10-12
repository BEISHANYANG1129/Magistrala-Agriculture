package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const assessmentJSONSchema = `{"type":"object","additionalProperties":false,"required":["region_id","mapping_version","findings","knowledge_ids"],"properties":{"region_id":{"type":"string"},"mapping_version":{"type":"string"},"findings":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"object","additionalProperties":false,"required":["kind","explanation","evidence_ids"],"properties":{"kind":{"type":"string","enum":["soil_dry","normal","observation","uncertain"]},"explanation":{"type":"string"},"evidence_ids":{"type":"array","minItems":1,"items":{"type":"string"}}}}},"knowledge_ids":{"type":"array","items":{"type":"string"}}}}`

// LangChainGo v0.1.15's Ollama request exposes format only as a string.
// This client-local transport adds Ollama's JSON-schema format while retaining
// LangChainGo's model interface, messages, cancellation and response handling.
// Semantic/evidence validation still happens in the engine after generation.
type assessmentSchemaTransport struct{ base http.RoundTripper }

func (s assessmentSchemaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || req.URL.Path != "/api/chat" {
		return s.base.RoundTrip(req)
	}
	if req.Body == nil {
		return nil, errors.New("missing Ollama request body")
	}
	defer req.Body.Close()
	const maxRequest = 1 << 20
	body, err := io.ReadAll(io.LimitReader(req.Body, maxRequest+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxRequest {
		return nil, errors.New("Ollama request exceeds size limit")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, errors.New("invalid Ollama request object")
	}
	payload["format"] = json.RawMessage(assessmentJSONSchema)
	body, err = json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	cloned := req.Clone(req.Context())
	cloned.Body = io.NopCloser(bytes.NewReader(body))
	cloned.ContentLength = int64(len(body))
	cloned.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return s.base.RoundTrip(cloned)
}
