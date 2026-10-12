package models

// ResultData defines the standard API response envelope used by all handlers.
//
// Semantics:
// - Code: application-level status code (1000 = success, non-1000 = error)
// - Message: human-readable message describing the outcome
// - Data: optional payload; omitted when empty to reduce response size
//
// Notes:
// - Keep Code semantics consistent across endpoints for predictable clients.
// - Consider adding an `error` field with machine-readable error codes later.
type ResultData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
