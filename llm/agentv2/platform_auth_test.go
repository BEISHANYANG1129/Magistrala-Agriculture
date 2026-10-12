package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegratedWorkspaceIsolation(t *testing.T) {
	token := "Bearer header." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"alice"}`)) + ".signature"
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != token {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"errors":[{"message":"denied"}]}`))
			return
		}
		var body struct {
			Query     string                     `json:"query"`
			Variables map[string]json.RawMessage `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.Contains(body.Query, "authzCheck") {
			_, _ = w.Write([]byte(`{"data":{"authzCheck":{"allowed":true}}}`))
			return
		}
		var id string
		_ = json.Unmarshal(body.Variables["id"], &id)
		if strings.Contains(body.Query, "tenant(") {
			if id == "A" {
				_, _ = w.Write([]byte(`{"data":{"tenant":{"id":"A"}}}`))
			} else {
				_, _ = w.Write([]byte(`{"errors":[{"message":"denied"}]}`))
			}
			return
		}
		kind := "entity"
		if strings.Contains(body.Query, "resource(") {
			kind = "resource"
		}
		workspace := "A"
		if id == "foreign" {
			workspace = "B"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{kind: map[string]string{"tenantId": workspace}}})
	}))
	defer platform.Close()
	path := filepath.Join(t.TempDir(), "regions.json")
	initial := Registry{Version: "v1", Regions: []Region{{ID: "north", Name: "North", WorkspaceID: "A", Sensors: []Binding{{ChannelID: "c", PublisherID: "d"}}}, {ID: "south", Name: "South", WorkspaceID: "B", Sensors: []Binding{{ChannelID: "c2", PublisherID: "d2"}}}}}
	if err := atomicJSON(path, initial); err != nil {
		t.Fatal(err)
	}
	s := NewServer(&Engine{RegistryPath: path})
	s.Platform = &PlatformAuth{URL: platform.URL}
	s.EditableRegions = true
	call := func(method, path, auth, workspace string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(b))
		r.Header.Set("Authorization", auth)
		r.Header.Set("X-Workspace-ID", workspace)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/v1/regions", "", "A", nil); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := call("GET", "/v1/regions", token, "B", nil); w.Code != 403 {
		t.Fatalf("unauthorized workspace: %d", w.Code)
	}
	w := call("GET", "/v1/regions", token, "A", nil)
	var view struct {
		Registry
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(view.Regions) != 1 || view.Regions[0].WorkspaceID != "A" {
		t.Fatalf("leaked registry: %s", w.Body)
	}
	if w := call("GET", "/v1/history?workspace_id=B", token, "A", nil); w.Code != 403 {
		t.Fatalf("history mismatch: %d", w.Code)
	}
	if w := call("POST", "/v1/analyze", token, "A", Request{WorkspaceID: "B"}); w.Code != 403 {
		t.Fatalf("analysis mismatch: %d", w.Code)
	}
	s.results["other"] = Result{WorkspaceID: "B"}
	s.simulations["other"] = Simulation{Status: "completed"}
	if w := call("POST", "/v1/simulate", token, "A", map[string]string{"analysis_id": "other"}); w.Code != 403 {
		t.Fatalf("simulation receipt leak: %d", w.Code)
	}
	view.Regions[0].Name = "Edited"
	input := map[string]any{"expected_revision": view.Revision, "registry": view.Registry}
	if w := call("POST", "/v1/regions", token, "A", input); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	saved, hash, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	other, err := saved.Find("B", "south")
	if err != nil || other.Name != "South" {
		t.Fatal("other workspace was overwritten")
	}
	view.Regions[0].Sensors[0].PublisherID = "foreign"
	input["registry"] = view.Registry
	input["expected_revision"] = hash
	if w := call("POST", "/v1/regions", token, "A", input); w.Code != 403 {
		t.Fatalf("foreign binding: %d", w.Code)
	}
	_, after, _ := LoadRegistry(path)
	if after != hash {
		t.Fatal("rejected save changed registry")
	}
}

func TestRequestReaderRequiresMatchingIdentity(t *testing.T) {
	reader := RequestReader{BaseURL: "http://invalid.invalid", TimeUnit: "nanoseconds"}
	ctx := context.WithValue(context.Background(), workspaceContext, "A")
	if _, err := reader.Read(ctx, "A", Binding{}, time.Now()); err == nil {
		t.Fatal("missing caller token accepted")
	}
	ctx = context.WithValue(ctx, authorizationContext, "Bearer token")
	if _, err := reader.Read(ctx, "B", Binding{}, time.Now()); err == nil {
		t.Fatal("cross-workspace read accepted")
	}
}
