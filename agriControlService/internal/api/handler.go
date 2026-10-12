package api

import (
	"encoding/json"
	"net/http"

	"agri-control-service/internal/model"
	"agri-control-service/internal/service"

	"github.com/google/uuid"
)

type Handler struct {
	ctrl *service.ControlService
}

// NewHandler 绑定控制服务，用于对外提供 HTTP 接口。
func NewHandler(ctrl *service.ControlService) *Handler {
	return &Handler{ctrl: ctrl}
}

// HandleTask 接收 POST /control/task，解析任务、补全标识并入队。
func (h *Handler) HandleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var task model.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ensureIDs(&task)

	if err := h.ctrl.HandleTask(&task); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("task executed"))
}

// ensureIDs 确保任务有 task_id/trace_id，便于链路追踪。
func ensureIDs(task *model.Task) {
	if task.TaskID == "" {
		task.TaskID = uuid.NewString()
	}
	if task.TraceID == "" {
		task.TraceID = task.TaskID
	}
}
