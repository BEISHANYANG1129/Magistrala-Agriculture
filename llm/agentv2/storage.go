package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var recordName = regexp.MustCompile(`^[a-f0-9]{32}\.json$`)

type savedRecord struct {
	Result       Result      `json:"result"`
	RegistryHash string      `json:"registry_hash"`
	Simulation   *Simulation `json:"simulation,omitempty"`
}

// Atomic replacement leaves the old file intact if serialization or writing fails.
func atomicJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".agent-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func NewPersistentServer(e *Engine, dir string) (*Server, error) {
	s := NewServer(e)
	s.stateDir = dir
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || !recordName.MatchString(file.Name()) {
			continue
		}
		if len(s.results) >= 200 {
			return nil, errors.New("history exceeds 200 records; use a new history directory")
		}
		info, err := file.Info()
		if err != nil {
			return nil, err
		}
		if info.Size() > 4<<20 {
			return nil, errors.New("history record too large")
		}
		b, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}
		var record savedRecord
		if err = decodeStrict(b, &record); err != nil {
			return nil, err
		}
		if file.Name() != record.Result.AnalysisID+".json" || record.RegistryHash == "" {
			return nil, errors.New("invalid history identity")
		}
		record.Result.registryHash = record.RegistryHash
		s.results[record.Result.AnalysisID] = record.Result
		if record.Simulation != nil {
			sim := *record.Simulation
			if sim.AnalysisID != record.Result.AnalysisID {
				return nil, errors.New("simulation identity mismatch")
			}
			// Never replay a request whose completion was interrupted or unknown.
			if sim.Status == "simulating" {
				sim.Status = "simulation_unknown"
			}
			s.simulations[sim.AnalysisID] = sim
		}
	}
	return s, nil
}
func (s *Server) persist(r Result, sim *Simulation) error {
	if s.stateDir == "" {
		return nil
	}
	return atomicJSON(filepath.Join(s.stateDir, r.AnalysisID+".json"), savedRecord{r, r.registryHash, sim})
}
func (s *Server) history(workspace, region string) []savedRecord {
	out := []savedRecord{}
	for id, r := range s.results {
		if r.WorkspaceID != workspace || (region != "" && r.RegionID != region) {
			continue
		}
		item := savedRecord{Result: r}
		if sim, ok := s.simulations[id]; ok {
			copy := sim
			item.Simulation = &copy
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Result.CreatedAt.After(out[j].Result.CreatedAt) })
	return out
}
