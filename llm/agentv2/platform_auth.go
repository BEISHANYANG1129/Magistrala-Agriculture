package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type platformContextKey string

const workspaceContext platformContextKey = "workspace"
const authorizationContext platformContextKey = "authorization"

// PlatformAuth delegates token validation and permissions to the running Atom service.
// JWT claims are used only to name the subject of a server-authorized check.
type PlatformAuth struct {
	URL    string
	Client *http.Client
}

func (p *PlatformAuth) query(ctx context.Context, auth, query string, variables any, dst any) error {
	b, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(p.URL, "/")+"/graphql", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("platform unavailable: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return err
	}
	if resp.StatusCode != 200 || len(result.Errors) > 0 || string(result.Data) == "null" {
		return fmt.Errorf("platform authorization denied")
	}
	return json.Unmarshal(result.Data, dst)
}
func (p *PlatformAuth) authorize(ctx context.Context, auth, workspace string, manage bool) error {
	if !strings.HasPrefix(auth, "Bearer ") || workspace == "" {
		return fmt.Errorf("login and workspace required")
	}
	var view struct {
		Tenant struct {
			ID string `json:"id"`
		} `json:"tenant"`
	}
	if err := p.query(ctx, auth, "query($id:ID!){tenant(id:$id){id}}", map[string]string{"id": workspace}, &view); err != nil {
		return err
	}
	if view.Tenant.ID != workspace {
		return fmt.Errorf("workspace access denied")
	}
	if !manage {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(auth, "Bearer "), ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid session token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("invalid session token")
	}
	var claims struct {
		Subject string `json:"sub"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Subject == "" {
		return fmt.Errorf("invalid session subject")
	}
	var decision struct {
		Check struct {
			Allowed bool `json:"allowed"`
		} `json:"authzCheck"`
	}
	input := map[string]any{"subjectId": claims.Subject, "action": "manage", "objectKind": "tenant", "objectId": workspace}
	if err = p.query(ctx, auth, "mutation($input:AuthzCheckInput!){authzCheck(input:$input){allowed}}", map[string]any{"input": input}, &decision); err != nil {
		return err
	}
	if !decision.Check.Allowed {
		return fmt.Errorf("workspace management permission required")
	}
	return nil
}
func requestWorkspace(ctx context.Context) string {
	s, _ := ctx.Value(workspaceContext).(string)
	return s
}

func (p *PlatformAuth) validateBindings(ctx context.Context, reg Registry) error {
	auth, _ := ctx.Value(authorizationContext).(string)
	seen := map[string]bool{}
	for _, region := range reg.Regions {
		for _, binding := range region.Sensors {
			for _, object := range []struct{ kind, id string }{{"entity", binding.PublisherID}, {"resource", binding.ChannelID}} {
				key := object.kind + ":" + object.id
				if seen[key] {
					continue
				}
				seen[key] = true
				var data map[string]struct {
					TenantID string `json:"tenantId"`
				}
				if err := p.query(ctx, auth, "query($id:ID!){"+object.kind+"(id:$id){tenantId}}", map[string]string{"id": object.id}, &data); err != nil {
					return fmt.Errorf("cannot verify sensor binding: %w", err)
				}
				if data[object.kind].TenantID != region.WorkspaceID {
					return fmt.Errorf("sensor binding is outside workspace")
				}
			}
		}
		for _, id := range region.Executors {
			if strings.HasPrefix(id, "simulation-only-") {
				continue
			}
			var data struct {
				Entity struct {
					TenantID string `json:"tenantId"`
				} `json:"entity"`
			}
			if err := p.query(ctx, auth, "query($id:ID!){entity(id:$id){tenantId}}", map[string]string{"id": id}, &data); err != nil {
				return fmt.Errorf("cannot verify actuator binding: %w", err)
			}
			if data.Entity.TenantID != region.WorkspaceID {
				return fmt.Errorf("actuator binding is outside workspace")
			}
		}
	}
	return nil
}
func scopedRegistry(reg Registry, workspace string) Registry {
	if workspace == "" {
		return reg
	}
	out := Registry{Version: reg.Version, Regions: []Region{}}
	for _, r := range reg.Regions {
		if r.WorkspaceID == workspace {
			out.Regions = append(out.Regions, r)
		}
	}
	return out
}
func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Platform == nil || r.URL.Path == "/health" || r.URL.Path == "/" || r.URL.Path == "/app.js" {
			next.ServeHTTP(w, r)
			return
		}
		workspace := r.Header.Get("X-Workspace-ID")
		auth := r.Header.Get("Authorization")
		if auth == "" {
			fail(w, 401, fmt.Errorf("login required"))
			return
		}
		if err := s.Platform.authorize(r.Context(), auth, workspace, r.Method != "GET"); err != nil {
			fail(w, 403, err)
			return
		}
		ctx := context.WithValue(r.Context(), workspaceContext, workspace)
		ctx = context.WithValue(ctx, authorizationContext, auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestReader never keeps a shared administrator credential in the integrated service.
type RequestReader struct{ BaseURL, TimeUnit string }

func (r RequestReader) Read(ctx context.Context, workspace string, b Binding, since time.Time) ([]Evidence, error) {
	auth, _ := ctx.Value(authorizationContext).(string)
	if auth == "" || requestWorkspace(ctx) != workspace {
		return nil, fmt.Errorf("authenticated workspace required")
	}
	return (&Reader{BaseURL: r.BaseURL, TimeUnit: r.TimeUnit, Authorization: auth}).Read(ctx, workspace, b, since)
}

// GuardedAnalyst checks before each expert call, including a second call in one analysis.
type GuardedAnalyst struct {
	Inner          Analyst
	Python, Script string
}

func (g GuardedAnalyst) Analyze(ctx context.Context, role string, region Region, version string, evidence []Evidence, knowledge []Knowledge) (Assessment, error) {
	check, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := exec.CommandContext(check, g.Python, g.Script, "--check-only").Run(); err != nil {
		return Assessment{}, fmt.Errorf("GPU is busy or idle status cannot be verified; wait and retry")
	}
	return g.Inner.Analyze(ctx, role, region, version, evidence, knowledge)
}
