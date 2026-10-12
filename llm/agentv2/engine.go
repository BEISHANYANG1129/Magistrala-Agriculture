package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Engine struct {
	RegistryPath string
	Source       Source
	Analyst      Analyst
	Knowledge    []Knowledge
	MaxAge       time.Duration
	Now          func() time.Time
	Mode         string
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}
func (e *Engine) Analyze(ctx context.Context, req Request) (Result, error) {
	now := e.now()
	reg, hash, err := LoadRegistry(e.RegistryPath)
	if err != nil {
		return Result{}, err
	}
	if req.MappingVersion == "" || req.MappingVersion != reg.Version {
		return Result{}, errors.New("mapping version mismatch; refresh regions")
	}
	region, err := reg.Find(req.WorkspaceID, req.RegionID)
	if err != nil {
		return Result{}, err
	}
	if e.Source == nil || e.Analyst == nil || e.MaxAge <= 0 {
		return Result{}, errors.New("engine not configured")
	}
	out := Result{AnalysisID: newID(), WorkspaceID: req.WorkspaceID, RegionID: region.ID, MappingVersion: reg.Version, CreatedAt: now, Mode: e.Mode, Status: "insufficient_data", Evidence: []Evidence{}, Knowledge: []Knowledge{}, Assessments: []Assessment{}, Tasks: []Task{}, Trace: []Trace{}, registryHash: hash}
	out.Trace = append(out.Trace, Trace{"resolve_region", "workspace and region binding validated"})
	complete := true
	seen := map[[3]string]Evidence{}
	for _, binding := range region.Sensors {
		rows, err := e.Source.Read(ctx, req.WorkspaceID, binding, now.Add(-e.MaxAge))
		if err != nil {
			return Result{}, err
		}
		found := false
		for _, row := range rows {
			if row.ChannelID != binding.ChannelID || row.PublisherID != binding.PublisherID {
				return Result{}, errors.New("source returned evidence outside requested binding")
			}
			if row.ObservedAt.Before(now.Add(-e.MaxAge)) || row.ObservedAt.After(now.Add(30*time.Second)) {
				continue
			}
			if row.ID == "" || row.Metric == "" || math.IsNaN(row.Value) || math.IsInf(row.Value, 0) {
				return Result{}, errors.New("invalid evidence")
			}
			if row.Metric == "soil_moisture" && (row.Unit != "%" || row.Value < 0 || row.Value > 100) {
				return Result{}, errors.New("soil_moisture requires a percent value between 0 and 100")
			}
			key := [3]string{row.ChannelID, row.PublisherID, row.Metric}
			old, ok := seen[key]
			if ok && old.ObservedAt.Equal(row.ObservedAt) && (old.Value != row.Value || old.Unit != row.Unit) {
				return Result{}, errors.New("conflicting observations at the same timestamp")
			}
			if !ok || row.ObservedAt.After(old.ObservedAt) {
				seen[key] = row
			}
			found = true
		}
		if !found {
			complete = false
		}
	}
	for _, row := range seen {
		out.Evidence = append(out.Evidence, row)
	}
	sort.Slice(out.Evidence, func(i, j int) bool { return out.Evidence[i].ID < out.Evidence[j].ID })
	out.Trace = append(out.Trace, Trace{"read_evidence", fmt.Sprintf("%d current observations; all bindings covered=%t", len(out.Evidence), complete)})
	if !complete || len(out.Evidence) == 0 {
		return out, nil
	}
	groups := map[string][]Evidence{}
	for _, row := range out.Evidence {
		role := "environment"
		if row.Metric == "soil_moisture" || row.Metric == "soil_temperature" {
			role = "soil"
		}
		groups[role] = append(groups[role], row)
	}
	usedKnowledge := map[string]bool{}
	for _, role := range []string{"soil", "environment"} {
		rows := groups[role]
		if len(rows) == 0 {
			continue
		}
		docs := retrieve(e.Knowledge, rows)
		for _, doc := range docs {
			if !usedKnowledge[doc.ID] {
				usedKnowledge[doc.ID] = true
				out.Knowledge = append(out.Knowledge, doc)
			}
		}
		assessment, err := e.Analyst.Analyze(ctx, role, region, reg.Version, rows, docs)
		if err != nil {
			return Result{}, fmt.Errorf("%s analyst: %w", role, err)
		}
		if err = validateAssessment(assessment, region, reg.Version, rows, docs); err != nil {
			return Result{}, err
		}
		out.Assessments = append(out.Assessments, assessment)
		out.Trace = append(out.Trace, Trace{"expert:" + role, "structured response and evidence references validated"})
	}
	if _, currentHash, err := LoadRegistry(e.RegistryPath); err != nil || currentHash != hash {
		return Result{}, errors.New("registry changed during analysis; retry with current mapping")
	}
	// Refuse to plan if inference took so long that the evidence expired.
	for _, row := range out.Evidence {
		if row.ObservedAt.Before(e.now().Add(-e.MaxAge)) {
			return Result{}, errors.New("evidence expired during analysis")
		}
	}
	out.Status = "analyzed"
	if region.SoilMoistureMin != nil && len(region.Executors) > 0 {
		rowsByID := map[string]Evidence{}
		for _, row := range out.Evidence {
			rowsByID[row.ID] = row
		}
		dry := false
		uncertain := false
		for _, a := range out.Assessments {
			for _, f := range a.Findings {
				if f.Kind == "uncertain" {
					uncertain = true
				}
				if f.Kind == "soil_dry" {
					for _, id := range f.EvidenceIDs {
						row := rowsByID[id]
						if row.Metric == "soil_moisture" && row.Unit == "%" && row.Value < *region.SoilMoistureMin {
							dry = true
						}
					}
				}
			}
		}
		if dry && !uncertain {
			out.Tasks = append(out.Tasks, Task{TaskID: newID(), TraceID: out.AnalysisID, TaskType: "irrigation", Target: region.ID, Params: map[string]any{"requires_control_policy": true}, Source: "llm_advisory", WorkspaceID: region.WorkspaceID, MappingVersion: reg.Version, ExecutorIDs: append([]string{}, region.Executors...)})
			out.Status = "proposal_ready"
		}
	}
	out.Trace = append(out.Trace, Trace{"policy", "advisory tasks only; no duration inferred and no device command sent"})
	return out, nil
}
func retrieve(docs []Knowledge, rows []Evidence) []Knowledge {
	result := []Knowledge{}
	for _, doc := range docs {
		matched := false
		for _, metric := range doc.Metrics {
			for _, row := range rows {
				if metric == row.Metric {
					matched = true
				}
			}
		}
		if matched && doc.ID != "" && doc.Source != "" && doc.Text != "" {
			result = append(result, doc)
		}
		if len(result) >= 4 {
			break
		}
	}
	return result
}
func validateAssessment(a Assessment, r Region, version string, rows []Evidence, docs []Knowledge) error {
	if a.RegionID != r.ID || a.MappingVersion != version {
		return errors.New("model returned another region or mapping version")
	}
	if len(a.Findings) == 0 || len(a.Findings) > 20 {
		return errors.New("model must return between 1 and 20 findings")
	}
	ids := map[string]Evidence{}
	for _, row := range rows {
		ids[row.ID] = row
	}
	knowledge := map[string]bool{}
	for _, doc := range docs {
		knowledge[doc.ID] = true
	}
	for _, id := range a.KnowledgeIDs {
		if !knowledge[id] {
			return errors.New("unknown knowledge reference")
		}
	}
	for _, f := range a.Findings {
		if f.Kind != "soil_dry" && f.Kind != "normal" && f.Kind != "observation" && f.Kind != "uncertain" {
			return errors.New("invalid finding kind")
		}
		if strings.TrimSpace(f.Explanation) == "" || len(f.EvidenceIDs) == 0 {
			return errors.New("finding requires explanation and evidence")
		}
		drySupported := false
		for _, id := range f.EvidenceIDs {
			row, exists := ids[id]
			if !exists {
				return errors.New("model cited evidence outside selected region/expert")
			}
			if row.Metric == "soil_moisture" && row.Unit == "%" {
				below := r.SoilMoistureMin != nil && row.Value < *r.SoilMoistureMin
				drySupported = drySupported || below
				if f.Kind == "normal" && (r.SoilMoistureMin == nil || below) {
					return errors.New("model normal finding contradicts soil threshold evidence")
				}
			}
		}
		if f.Kind == "soil_dry" && !drySupported {
			return errors.New("model soil_dry finding lacks below-threshold soil evidence")
		}
	}
	return nil
}
