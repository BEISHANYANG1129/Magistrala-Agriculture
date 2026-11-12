package workbench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMicrosoftTimeRecognizerAndPlatformPolicy(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is not installed")
	}
	module := filepath.Join("..", "time-recognizer")
	if _, err := os.Stat(filepath.Join(module, "node_modules", "@microsoft", "recognizers-text-date-time")); err != nil {
		t.Skip("run npm ci --prefix time-recognizer to exercise the local component")
	}
	recognizer := &TimeRecognizer{Node: node, Script: filepath.Join(module, "index.cjs")}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, zone)
	reference := time.Date(2026, 10, 26, 0, 0, 0, 0, zone)
	for _, test := range []struct{ Text, Start, End, Failure string }{
		{"10月26日最高温度", "2026-10-26 00:00", "2026-10-27 00:00", ""},
		{"十月二十六日", "2026-10-26 00:00", "2026-10-27 00:00", ""},
		{"10月26日上午9点到下午3点", "2026-10-26 09:00", "2026-10-26 15:00", ""},
		{"10月26日到27日", "2026-10-26 00:00", "2026-10-28 00:00", ""},
		{"上周", "2026-10-26 00:00", "2026-11-02 00:00", ""},
		{"那下午呢", "2026-10-26 12:00", "2026-10-26 18:00", ""},
		{"换成27号呢", "2026-10-27 00:00", "2026-10-28 00:00", ""},
		{"最近两个小时", "2026-11-05 10:00", "2026-11-05 12:00", ""},
		{"2月30日", "", "", "日期无效，请核对月份和天数"},
		{"最近8天", "", "", "单次查询最多7天，请缩小时间范围"},
		{"12月26日", "", "", "所选日期尚未到来，请提供已发生的观测时间"},
	} {
		t.Run(test.Text, func(t *testing.T) {
			results, err := recognizer.Recognize(context.Background(), test.Text, now, reference)
			if err != nil || len(results) != 1 {
				t.Fatalf("unexpected entities %+v, %v", results, err)
			}
			got := results[0]
			if test.Failure != "" {
				if got.Window != nil || got.Error != test.Failure {
					t.Fatalf("invalid date/range accepted %+v", got)
				}
				return
			}
			if got.Error != "" || got.Window == nil || got.Window.Start.Format("2006-01-02 15:04") != test.Start || got.Window.End.Format("2006-01-02 15:04") != test.End {
				t.Fatalf("unexpected component resolution %+v", got)
			}
		})
	}
	results, err := recognizer.Recognize(context.Background(), "10月26日和10月27日的平均温度分别是多少", now, reference)
	if err != nil || len(results) != 2 || results[0].Window.Start.Day() != 26 || results[1].Window.Start.Day() != 27 {
		t.Fatalf("multiple dates were lost %+v, %v", results, err)
	}
	results, err = recognizer.Recognize(context.Background(), "10月26日到27日", now, now)
	if err != nil || len(results) != 1 || results[0].Window == nil || results[0].Window.End.Format("2006-01-02") != "2026-10-28" {
		t.Fatalf("range end inherited the current month instead of its start %+v, %v", results, err)
	}
}
