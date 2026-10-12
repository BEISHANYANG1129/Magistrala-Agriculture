package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agri-control-service/internal/model"

	"gopkg.in/yaml.v3"
)

// registry 包：维护 TaskType → Action 序列的注册表，是新增场景/任务的主要扩展点。

// TaskType -> Action 映射注册表（运行时表）；启动时从配置克隆，未加载或失败则用内置默认表。
var TaskActionRegistry = cloneRegistry(defaultTaskActionRegistry)

// defaultTaskActionRegistry: 内置的兜底映射，加载外部配置失败时使用。
var defaultTaskActionRegistry = map[string][]model.Action{
	"irrigation": {
		{ActionType: "open_valve", DeviceType: "irrigation"},
		{ActionType: "wait", DeviceType: "system"},
		{ActionType: "close_valve", DeviceType: "irrigation"},
	},
}

// registryConfig: 配置文件结构，仅关心 actions 段。
type registryConfig struct {
	Actions map[string][]model.Action `json:"actions" yaml:"actions"`
}

// LoadFromFile 尝试从 YAML/JSON 配置加载 Task → Action 映射，成功则替换运行时表，失败保留默认表。
func LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read registry config: %w", err)
	}

	var cfg registryConfig
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("unmarshal yaml registry: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("unmarshal json registry: %w", err)
		}
	default:
		return fmt.Errorf("unsupported registry file type: %s", path)
	}

	if len(cfg.Actions) == 0 {
		return errors.New("registry config has no actions")
	}

	// 采用深拷贝后的配置作为新的运行时表，避免外部修改影响。
	TaskActionRegistry = cloneRegistry(cfg.Actions)
	return nil
}

// cloneRegistry: 对映射和动作切片做浅层值拷贝，避免共享底层切片。
func cloneRegistry(src map[string][]model.Action) map[string][]model.Action {
	dst := make(map[string][]model.Action, len(src))
	for k, v := range src {
		copied := make([]model.Action, len(v))
		copy(copied, v)
		dst[k] = copied
	}
	return dst
}
