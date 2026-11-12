// Package workbench implements the evidence-first agricultural workbench.
// Models propose finite camera plans; physical tools require explicit plan confirmation.
package workbench

import (
	"context"
	"time"
)

const DefaultModel = "qwen3.5:9b"

type Metric struct {
	Key              string `json:"key"`
	Label            string `json:"label"`
	Unit             string `json:"unit"`
	Admission        string `json:"admission"`
	UnitConfirmation string `json:"unit_confirmation"`
	PublisherID      string `json:"publisher_id"`
}
type Frame struct {
	ID         string             `json:"id"`
	ReceivedAt time.Time          `json:"received_at"`
	Source     string             `json:"source"`
	Values     map[string]float64 `json:"values"`
	Units      map[string]string  `json:"units,omitempty"`
}
type Dataset struct {
	Version     string            `json:"version"`
	WorkspaceID string            `json:"workspace_id"`
	StationID   string            `json:"station_id"`
	ChannelID   string            `json:"channel_id"`
	TimeKind    string            `json:"time_kind"`
	Metrics     []Metric          `json:"metrics"`
	Frames      []Frame           `json:"frames"`
	Sources     map[string]string `json:"sources"`
}
type Chunk struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Text        string  `json:"text"`
	Source      string  `json:"source"`
	Version     string  `json:"version"`
	WorkspaceID string  `json:"workspace_id,omitempty"`
	Crop        string  `json:"crop,omitempty"`
	Status      string  `json:"status"`
	Kind        string  `json:"kind"`
	Score       float64 `json:"score,omitempty"`
}
type Summary struct {
	ID string `json:"id"`
	Metric
	Count      int       `json:"count"`
	Min        float64   `json:"min"`
	Max        float64   `json:"max"`
	Mean       *float64  `json:"mean,omitempty"`
	Distinct   int       `json:"distinct"`
	BelowRange int       `json:"below_range"`
	FirstAt    time.Time `json:"first_at"`
	LastAt     time.Time `json:"last_at"`
	Warnings   []string  `json:"warnings"`
}
type Evidence struct {
	Mode           string            `json:"mode"`
	DatasetVersion string            `json:"dataset_version"`
	TimeKind       string            `json:"time_kind"`
	Start          time.Time         `json:"start"`
	End            time.Time         `json:"end"`
	Rows           int               `json:"rows"`
	Metrics        []Summary         `json:"metrics"`
	Warnings       []string          `json:"warnings"`
	Sources        map[string]string `json:"sources"`
}
type Trace struct {
	Role   string `json:"role"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
type Claim struct {
	Text      string   `json:"text"`
	Kind      string   `json:"kind"`
	Citations []string `json:"citations"`
	facts     []string
}
type ModelOutput struct {
	Claims            []Claim  `json:"claims"`
	Limitations       []string `json:"limitations"`
	ImageObservations []Claim  `json:"image_observations,omitempty"`
}
type Review struct {
	Approved bool     `json:"approved"`
	Issues   []string `json:"issues"`
}
type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}
type Model interface {
	Generate(context.Context, []Message, any, any) error
}
type Query struct {
	Metric      string    `json:"-"`
	Mode        string    `json:"mode"`
	ChannelID   string    `json:"channel_id,omitempty"`
	PublisherID string    `json:"publisher_id,omitempty"`
	StationID   string    `json:"station_id,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}
type Input struct {
	cameraSource  *CameraSource
	ParentID      string  `json:"parent_id,omitempty"`
	Task          string  `json:"task"`
	Question      string  `json:"question"`
	Crop          string  `json:"crop,omitempty"`
	Query         *Query  `json:"query,omitempty"`
	Compare       *Window `json:"compare,omitempty"`
	Image         string  `json:"image,omitempty"`
	ImageSource   string  `json:"image_source,omitempty"`
	sceneImage    bool
	platformPhoto bool
}
type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type Plan struct {
	Target          string    `json:"target"`
	Action          string    `json:"action"`
	DurationSeconds int       `json:"duration_seconds"`
	StopAction      string    `json:"stop_action"`
	Mode            string    `json:"mode"`
	Revision        string    `json:"revision"`
	ExpiresAt       time.Time `json:"expires_at"`
	Capability      string    `json:"capability_version,omitempty"`
	Basis           string    `json:"basis_run_id,omitempty"`
	BasisSnapshot   string    `json:"basis_snapshot,omitempty"`
}
type Run struct {
	CameraSource  *CameraSource      `json:"camera_source,omitempty"`
	Synthesized   bool               `json:"synthesized,omitempty"`
	AnswerNotes   []string           `json:"answer_notes,omitempty"`
	Clarification string             `json:"clarification,omitempty"`
	QueryResults  []QueryResult      `json:"query_results,omitempty"`
	ParentID      string             `json:"parent_id,omitempty"`
	ID            string             `json:"id"`
	WorkspaceID   string             `json:"workspace_id"`
	CreatedAt     time.Time          `json:"created_at"`
	Task          string             `json:"task"`
	Question      string             `json:"question"`
	Model         string             `json:"model"`
	Status        string             `json:"status"`
	Claims        []Claim            `json:"claims"`
	Limitations   []string           `json:"limitations"`
	Knowledge     []Chunk            `json:"knowledge"`
	Retrieval     string             `json:"retrieval"`
	Evidence      *Evidence          `json:"evidence,omitempty"`
	Comparison    *Evidence          `json:"comparison,omitempty"`
	Workflow      *Workflow          `json:"workflow,omitempty"`
	Trace         []Trace            `json:"trace"`
	ImageHash     string             `json:"image_hash,omitempty"`
	ImageSource   string             `json:"image_source,omitempty"`
	Plan          *Plan              `json:"plan,omitempty"`
	Receipt       string             `json:"receipt,omitempty"`
	Approval      *Approval          `json:"approval,omitempty"`
	Feedback      *ExecutionFeedback `json:"execution_feedback,omitempty"`
	Automation    *Automation        `json:"automation,omitempty"`

	owner           string
	pending         *queryPlan
	query           *queryPlan
	archive         *chatMemory
	compaction      *chatMemory
	automationImage string
}

// ReadLive must enforce caller credentials and publisher/channel ownership.
type ReadLive func(context.Context, string, string, Query) ([]Frame, error)
