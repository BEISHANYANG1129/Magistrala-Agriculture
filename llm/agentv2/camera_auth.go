package agent

import (
	"context"
	"fmt"
	"magistrala-agri-agent/workbench"
)

func (p *PlatformAuth) checkWorkbenchCamera(ctx context.Context, workspace string, camera workbench.CameraSource, write bool) error {
	auth, _ := ctx.Value(authorizationContext).(string)
	if workspace != camera.WorkspaceID || requestWorkspace(ctx) != workspace || camera.BindingID != "jooan-g4s4" {
		return fmt.Errorf("camera workspace or binding mismatch")
	}
	var result struct {
		Entity struct {
			ID         string `json:"id"`
			TenantID   string `json:"tenantId"`
			Kind       string `json:"kind"`
			Status     string `json:"status"`
			Attributes struct {
				Metadata struct {
					Camera struct {
						Provider  string `json:"provider"`
						BindingID string `json:"bindingId"`
					} `json:"camera"`
				} `json:"metadata"`
			} `json:"attributes"`
		} `json:"entity"`
	}
	if err := p.query(ctx, auth, "query($id:ID!){entity(id:$id){id tenantId kind status attributes}}", map[string]string{"id": camera.DeviceID}, &result); err != nil {
		return err
	}
	d := result.Entity
	if d.ID != camera.DeviceID || d.TenantID != workspace || d.Kind != "device" || d.Status == "disabled" || d.Status == "deleted" || d.Attributes.Metadata.Camera.Provider != "jooan-tutk" || d.Attributes.Metadata.Camera.BindingID != camera.BindingID {
		return fmt.Errorf("camera binding unavailable")
	}
	if !write {
		return nil
	}
	subject, err := sessionSubject(auth)
	if err != nil {
		return err
	}
	var decision struct {
		Check struct {
			Allowed bool `json:"allowed"`
		} `json:"authzCheck"`
	}
	input := map[string]any{"subjectId": subject, "action": "write", "objectKind": "entity", "objectId": camera.DeviceID, "resourceId": camera.DeviceID, "context": map[string]string{"workspace_id": workspace}}
	if err = p.query(ctx, auth, "mutation($input:AuthzCheckInput!){authzCheck(input:$input){allowed}}", map[string]any{"input": input}, &decision); err != nil {
		return err
	}
	if !decision.Check.Allowed {
		return fmt.Errorf("camera control permission required")
	}
	return nil
}
