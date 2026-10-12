package agent

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

//go:embed web/index.html web/app.js
var assets embed.FS

type Server struct {
	Engine          *Engine
	Platform        *PlatformAuth
	Control         SimulationControl
	EditableRegions bool
	stateDir        string
	gate            chan struct{}
	mu              sync.Mutex
	results         map[string]Result
	simulations     map[string]Simulation
}
type Simulation struct {
	AnalysisID  string           `json:"analysis_id"`
	Status      string           `json:"status"`
	Mode        string           `json:"mode"`
	Tasks       []Task           `json:"tasks"`
	CompletedAt time.Time        `json:"completed_at"`
	Plans       []ControlPlan    `json:"control_plans"`
	Receipts    []ControlReceipt `json:"receipts"`
}

func NewServer(e *Engine) *Server {
	return &Server{Engine: e, Control: VirtualControl{}, gate: make(chan struct{}, 1), results: map[string]Result{}, simulations: map[string]Simulation{}}
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	reply(w, status, map[string]string{"error": err.Error()})
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /app.js", func(w http.ResponseWriter, r *http.Request) {
		b, _ := assets.ReadFile("web/app.js")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, _ := assets.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		historyMode := "memory"
		if s.stateDir != "" {
			historyMode = "persistent"
		}
		reply(w, 200, map[string]string{"status": "ok", "mode": s.Engine.Mode, "execution": "simulation_only", "history": historyMode})
	})
	mux.HandleFunc("GET /v1/regions", func(w http.ResponseWriter, r *http.Request) {
		reg, hash, err := LoadRegistry(s.Engine.RegistryPath)
		if err != nil {
			fail(w, 500, err)
			return
		}
		reply(w, 200, struct {
			Registry
			Revision string `json:"revision"`
			Editable bool   `json:"editable"`
		}{scopedRegistry(reg, requestWorkspace(r.Context())), hash, s.EditableRegions})
	})
	mux.HandleFunc("POST /v1/regions", s.saveRegions)
	mux.HandleFunc("GET /v1/history", func(w http.ResponseWriter, r *http.Request) {
		workspace := r.URL.Query().Get("workspace_id")
		if scope := requestWorkspace(r.Context()); scope != "" && scope != workspace {
			fail(w, 403, fmt.Errorf("workspace mismatch"))
			return
		}
		if workspace == "" {
			fail(w, 400, fmt.Errorf("workspace_id required"))
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		reply(w, 200, s.history(workspace, r.URL.Query().Get("region_id")))
	})
	mux.HandleFunc("POST /v1/analyze", s.analyze)
	mux.HandleFunc("POST /v1/simulate", s.simulate)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This preview is loopback-only. Reject cross-origin browser writes and DNS rebinding.
		host := r.Host
		if strings.Contains(host, ":") {
			u, err := url.Parse("http://" + host)
			if err == nil {
				host = u.Hostname()
			}
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			fail(w, 403, fmt.Errorf("loopback host required"))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || u.Scheme != "http" {
				fail(w, 403, fmt.Errorf("cross-origin requests are disabled"))
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == "POST" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			fail(w, 415, fmt.Errorf("application/json required"))
			return
		}
		s.authenticated(mux).ServeHTTP(w, r)
	})
}
func readRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		return err
	}
	return decodeStrict(b, dst)
}
func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {
	var input Request
	if err := readRequest(w, r, &input); err != nil {
		fail(w, 400, err)
		return
	}
	if scope := requestWorkspace(r.Context()); scope != "" && scope != input.WorkspaceID {
		fail(w, 403, fmt.Errorf("workspace mismatch"))
		return
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		fail(w, 429, fmt.Errorf("analysis already running"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	s.mu.Lock()
	full := len(s.results) >= 200
	s.mu.Unlock()
	if full {
		fail(w, 409, fmt.Errorf("history full; start with a new history directory"))
		return
	}
	result, err := s.Engine.Analyze(ctx, input)
	if err != nil {
		fail(w, 422, err)
		return
	}
	s.mu.Lock()
	if err := s.persist(result, nil); err != nil {
		s.mu.Unlock()
		fail(w, 500, fmt.Errorf("cannot save analysis: %w", err))
		return
	}
	s.results[result.AnalysisID] = result
	s.mu.Unlock()
	reply(w, 200, result)
}
func (s *Server) simulate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AnalysisID string `json:"analysis_id"`
	}
	if err := readRequest(w, r, &input); err != nil {
		fail(w, 400, err)
		return
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		fail(w, 429, fmt.Errorf("analysis or configuration operation running"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if scope := requestWorkspace(r.Context()); scope != "" {
		result, ok := s.results[input.AnalysisID]
		if !ok || result.WorkspaceID != scope {
			fail(w, 403, fmt.Errorf("analysis is outside workspace"))
			return
		}
	}
	if previous, ok := s.simulations[input.AnalysisID]; ok {
		reply(w, 200, previous)
		return
	}
	result, ok := s.results[input.AnalysisID]
	if !ok {
		fail(w, 404, fmt.Errorf("unknown analysis"))
		return
	}
	if len(result.Tasks) == 0 {
		fail(w, 409, fmt.Errorf("analysis has no eligible task proposal"))
		return
	}
	reg, hash, err := LoadRegistry(s.Engine.RegistryPath)
	if err != nil || hash != result.registryHash {
		fail(w, 409, fmt.Errorf("mapping changed; reanalyze before simulation"))
		return
	}
	for _, row := range result.Evidence {
		if row.ObservedAt.Before(s.Engine.now().Add(-s.Engine.MaxAge)) {
			fail(w, 409, fmt.Errorf("evidence expired; reanalyze"))
			return
		}
	}
	plans, err := BuildControlPlans(reg, result)
	if err != nil {
		fail(w, 409, err)
		return
	}
	sim := Simulation{AnalysisID: result.AnalysisID, Status: "simulating", Mode: "simulation", Tasks: result.Tasks, Plans: plans, Receipts: []ControlReceipt{}}
	// Save intent before any simulated work. An interrupted request must never auto-replay.
	if err = s.persist(result, &sim); err != nil {
		fail(w, 500, err)
		return
	}
	s.simulations[input.AnalysisID] = sim
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	for _, plan := range plans {
		receipt, callErr := s.Control.Simulate(ctx, plan)
		if callErr != nil {
			sim.Status = "simulation_unknown"
			sim.Receipts = append(sim.Receipts, ControlReceipt{TaskID: plan.Task.TaskID, DeviceID: plan.Task.Target, Status: "unknown", Detail: callErr.Error()})
			break
		}
		if receipt.TaskID != plan.Task.TaskID || receipt.DeviceID != plan.Task.Target || receipt.Status != "simulated_completed" {
			sim.Status = "simulation_failed"
			sim.Receipts = append(sim.Receipts, ControlReceipt{TaskID: plan.Task.TaskID, DeviceID: plan.Task.Target, Status: "failed", Detail: "invalid simulation receipt"})
			break
		}
		sim.Receipts = append(sim.Receipts, receipt)
	}
	if sim.Status == "simulating" {
		sim.Status = "simulated_completed"
	}
	sim.CompletedAt = s.Engine.now()
	s.simulations[input.AnalysisID] = sim
	if err = s.persist(result, &sim); err != nil {
		fail(w, 500, fmt.Errorf("simulation result could not be saved: %w", err))
		return
	}
	reply(w, 200, sim)
}

func (s *Server) saveRegions(w http.ResponseWriter, r *http.Request) {
	if !s.EditableRegions {
		fail(w, 403, fmt.Errorf("region editing is disabled for this instance"))
		return
	}
	var input struct {
		ExpectedRevision string   `json:"expected_revision"`
		Registry         Registry `json:"registry"`
	}
	if err := readRequest(w, r, &input); err != nil {
		fail(w, 400, err)
		return
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		fail(w, 409, fmt.Errorf("analysis or simulation is running; retry after it finishes"))
		return
	}
	reg, hash, err := LoadRegistry(s.Engine.RegistryPath)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if hash != input.ExpectedRevision {
		fail(w, 409, fmt.Errorf("configuration changed; reload before saving"))
		return
	}
	if scope := requestWorkspace(r.Context()); scope != "" {
		for _, region := range input.Registry.Regions {
			if region.WorkspaceID != scope {
				fail(w, 403, fmt.Errorf("region is outside workspace"))
				return
			}
		}
		changed := Registry{Version: input.Registry.Version}
		for _, region := range input.Registry.Regions {
			previous, findErr := reg.Find(scope, region.ID)
			before, _ := json.Marshal(previous)
			after, _ := json.Marshal(region)
			if findErr != nil || string(before) != string(after) {
				changed.Regions = append(changed.Regions, region)
			}
		}
		if err := s.Platform.validateBindings(r.Context(), changed); err != nil {
			fail(w, 403, err)
			return
		}
		for _, region := range reg.Regions {
			if region.WorkspaceID != scope {
				input.Registry.Regions = append(input.Registry.Regions, region)
			}
		}
	}
	input.Registry.Version = newID()
	if err = input.Registry.Validate(); err != nil {
		fail(w, 422, err)
		return
	}
	if err = atomicJSON(s.Engine.RegistryPath+".backup.json", reg); err != nil {
		fail(w, 500, err)
		return
	}
	if err = atomicJSON(s.Engine.RegistryPath, input.Registry); err != nil {
		fail(w, 500, err)
		return
	}
	_, newHash, err := LoadRegistry(s.Engine.RegistryPath)
	if err != nil {
		fail(w, 500, err)
		return
	}
	reply(w, 200, struct {
		Registry
		Revision string `json:"revision"`
		Editable bool   `json:"editable"`
	}{scopedRegistry(input.Registry, requestWorkspace(r.Context())), newHash, true})
}
