package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 审计日志以 JSON Lines 形式追加写入 audit.log
// 单行结构：AuditRecord
// 设计：轻量无轮转；后续可根据大小增加简单轮转策略。

const auditLogPath = "internal/data/audit.log"

var auditMu sync.Mutex

// AuditRecord 记录一次执行或映射相关操作。
type AuditRecord struct {
	Timestamp  int64       `json:"ts"`
	Action     string      `json:"action"` // valveControl / modeUpdate / syncAdd / syncSkip / syncError 等
	ClientId   string      `json:"clientId"`
	DeviceAddr string      `json:"deviceAddr"`
	NodeId     int         `json:"nodeId"`
	Success    bool        `json:"success"`
	Detail     string      `json:"detail"`          // 错误或补充说明
	Extra      interface{} `json:"extra,omitempty"` // 可选扩展字段
}

// AppendAudit 追加审计记录。
func AppendAudit(rec AuditRecord) error {
	auditMu.Lock()
	defer auditMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(auditLogPath), 0o755); err != nil {
		return fmt.Errorf("创建审计目录失败: %w", err)
	}
	f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开审计文件失败: %w", err)
	}
	defer f.Close()
	if rec.Timestamp == 0 {
		rec.Timestamp = time.Now().Unix()
	}
	b, _ := json.Marshal(rec)
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("写入审计失败: %w", err)
	}
	return nil
}
