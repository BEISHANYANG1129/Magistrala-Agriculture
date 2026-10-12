package main

import (
	"flag"
	"fmt"
	"log"

	"agri-control-service/internal/executor"
	"agri-control-service/internal/logstore"
	"agri-control-service/internal/model"
)

// 重放工具：读取执行日志并按筛选条件重放设备命令。
func main() {
	// CLI 参数：日志路径、按 task/trace 过滤、重放条数限制。
	logPath := flag.String("log", "data/execution.log", "execution log file (jsonl)")
	taskID := flag.String("task", "", "replay only this task_id (optional)")
	traceID := flag.String("trace", "", "replay only this trace_id (optional)")
	limit := flag.Int("limit", 0, "max records to replay (0 = all)")
	flag.Parse()

	// 读取日志（仅消费，不再写回）。
	store, err := logstore.NewLogStore(*logPath)
	if err != nil {
		log.Fatalf("init log store: %v", err)
	}

	entries, err := store.ReadAll()
	if err != nil {
		log.Fatalf("read log: %v", err)
	}

	exec := executor.NewExecutor(nil) // 重放时不再写日志，避免污染原记录

	count := 0
	for _, e := range entries {
		// 按 task/trace 过滤。
		if *taskID != "" && e.TaskID != *taskID {
			continue
		}
		if *traceID != "" && e.TraceID != *traceID {
			continue
		}
		// limit>0 时限制重放条数。
		if *limit > 0 && count >= *limit {
			break
		}

		cmd := model.DeviceCommand{
			DeviceID: e.DeviceID,
			Command:  e.Command,
			Params:   e.Params,
			TaskID:   e.TaskID,
			TraceID:  e.TraceID,
		}

		fmt.Printf("[REPLAY] trace=%s task=%s device=%s command=%s params=%v\n",
			cmd.TraceID, cmd.TaskID, cmd.DeviceID, cmd.Command, cmd.Params)

		// 直接调用执行器，当前实现为打印；可替换为真实设备调用。
		if err := exec.Execute(cmd); err != nil {
			log.Printf("replay failed task=%s trace=%s: %v", cmd.TaskID, cmd.TraceID, err)
		}
		count++
	}

	log.Printf("replay finished, executed %d commands", count)
}
