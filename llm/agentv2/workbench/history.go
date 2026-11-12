package workbench

import (
	"sort"
	"time"
)

// Memory metadata is persisted with its run and kept out of history API responses.
type runFile struct {
	Run
	Owner      string      `json:"qa_owner,omitempty"`
	Pending    *queryPlan  `json:"pending_query,omitempty"`
	Query      *queryPlan  `json:"query_context,omitempty"`
	Archive    *chatMemory `json:"archived_conversation,omitempty"`
	Compaction *chatMemory `json:"compacted_conversation,omitempty"`
}

func (s *Service) store(r Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.save(r); err != nil {
		return err
	}
	s.runs[r.ID] = r
	return nil
}

func (s *Service) history(workspace, task string) []Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []Run{}
	for _, run := range s.runs {
		if run.WorkspaceID == workspace && (task == "" || run.Task == task) {
			rows = append(rows, run)
		}
	}
	return rows
}

func olderRun(a, b Run) bool {
	if a.CreatedAt.Equal(b.CreatedAt) {
		return a.ID < b.ID
	}
	return a.CreatedAt.Before(b.CreatedAt)
}

func visibleRuns(rows []Run, subject string) []Run {
	visible := rows[:0]
	for _, r := range rows {
		if r.owner == "" || r.owner == subject {
			visible = append(visible, r)
		}
	}
	return visible
}

type conversationEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Service) conversationGroups(workspace string, subject ...string) map[string][]Run {
	rows := s.history(workspace, "qa")
	if len(subject) > 0 {
		rows = visibleRuns(rows, subject[0])
	}
	sort.Slice(rows, func(i, j int) bool { return olderRun(rows[i], rows[j]) })
	byID := make(map[string]Run, len(rows))
	for _, run := range rows {
		byID[run.ID] = run
	}
	roots := make(map[string]string, len(rows))
	groups := map[string][]Run{}
	for _, run := range rows {
		id := run.ID
		var path []string
		seen := map[string]bool{}
		for roots[id] == "" && !seen[id] {
			seen[id] = true
			path = append(path, id)
			row := byID[id]
			if _, exists := byID[row.ParentID]; !exists {
				// Older versions could delete a root while preserving its prefix.
				if row.archive != nil && row.archive.Through == row.ParentID && row.ParentID != "" {
					id = row.ParentID
				}
				break
			}
			id = row.ParentID
		}
		if roots[id] != "" {
			id = roots[id]
		}
		for _, member := range path {
			roots[member] = id
		}
		groups[id] = append(groups[id], run)
	}
	return groups
}

func conversationEntries(groups map[string][]Run) []conversationEntry {
	list := make([]conversationEntry, 0, len(groups))
	for id, turns := range groups {
		list = append(list, conversationEntry{ID: id, Title: turns[0].Question, UpdatedAt: turns[len(turns)-1].CreatedAt})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].ID < list[j].ID
		}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}
