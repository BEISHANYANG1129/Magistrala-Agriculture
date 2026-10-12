package logstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogEntry 表示一次动作执行的记录，按 JSONL 持久化。
type LogEntry struct {
	Timestamp string                 `json:"ts"`
	TaskID    string                 `json:"task_id"`
	TraceID   string                 `json:"trace_id"`
	DeviceID  string                 `json:"device_id"`
	Command   string                 `json:"command"`
	Params    map[string]interface{} `json:"params"`
	Status    string                 `json:"status"`
	Error     string                 `json:"error,omitempty"`
	ElapsedMs int64                  `json:"elapsed_ms"`
}

// LogStore 负责将执行日志追加到 JSONL 文件。
type LogStore struct {
	path string
	file *os.File
	enc  *json.Encoder
	mu   sync.Mutex
}

// NewLogStore 确保日志目录存在，并绑定到指定路径。
func NewLogStore(path string) (*LogStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &LogStore{path: path, file: f, enc: json.NewEncoder(f)}, nil
}

// Append 追加一条日志；出现错误会返回给调用方自行处理。
func (s *LogStore) Append(entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	return s.enc.Encode(entry)
}

// ReadAll 读取全部日志，便于重放或排查。
func (s *LogStore) ReadAll() ([]LogEntry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase buffer for large params payloads
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var out []LogEntry
	for scanner.Scan() {
		var e LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("unmarshal log line: %w", err)
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}
	return out, nil
}
