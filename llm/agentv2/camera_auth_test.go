package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"magistrala-agri-agent/workbench"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCameraAuthorizationChecksNativeBindingAndDeviceWrite(t *testing.T) {
	for _, kind := range []string{"read", "write", "write_denied", "foreign", "disabled", "wrong_binding"} {
		t.Run(kind, func(t *testing.T) {
			checks := 0
			source := workbench.CameraSource{WorkspaceID: "workspace", DeviceID: "device", BindingID: "jooan-g4s4"}
			token := "Bearer header." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"verified-user"}`)) + ".signature"
			platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != token {
					t.Error("caller identity missing")
				}
				var body struct {
					Query     string         `json:"query"`
					Variables map[string]any `json:"variables"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if strings.Contains(body.Query, "authzCheck") {
					checks++
					input := body.Variables["input"].(map[string]any)
					if input["action"] != "write" || input["objectId"] != "device" || input["resourceId"] != "device" {
						t.Error("not checking device write")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"authzCheck": map[string]bool{"allowed": kind != "write_denied"}}})
					return
				}
				tenant, status, binding := "workspace", "enabled", "jooan-g4s4"
				if kind == "foreign" {
					tenant = "other"
				}
				if kind == "disabled" {
					status = "disabled"
				}
				if kind == "wrong_binding" {
					binding = "other"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"entity": map[string]any{"id": "device", "tenantId": tenant, "status": status, "kind": "device", "attributes": map[string]any{"metadata": map[string]any{"camera": map[string]string{"provider": "jooan-tutk", "bindingId": binding}}}}}})
			}))
			defer platform.Close()
			ctx := context.WithValue(context.Background(), workspaceContext, "workspace")
			ctx = context.WithValue(ctx, authorizationContext, token)
			err := (&PlatformAuth{URL: platform.URL}).checkWorkbenchCamera(ctx, "workspace", source, kind != "read")
			if (err == nil) != (kind == "read" || kind == "write") {
				t.Fatalf("unexpected access: %v", err)
			}
			if kind == "read" && checks != 0 {
				t.Fatal("read required write permission")
			}
			if kind == "write" && checks != 1 {
				t.Fatal("missing write check")
			}
		})
	}
}
