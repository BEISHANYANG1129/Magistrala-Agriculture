package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

func decodeStrict(b []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}
func LoadRegistry(path string) (Registry, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, "", err
	}
	var reg Registry
	if err = decodeStrict(b, &reg); err != nil {
		return reg, "", err
	}
	if err = reg.Validate(); err != nil {
		return reg, "", err
	}
	sum := sha256.Sum256(b)
	return reg, hex.EncodeToString(sum[:]), nil
}
func (reg Registry) Validate() error {
	if strings.TrimSpace(reg.Version) == "" {
		return errors.New("registry version is required")
	}
	regions := map[[2]string]bool{}
	executors := map[[2]string]bool{}
	for _, r := range reg.Regions {
		if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.WorkspaceID) == "" || strings.TrimSpace(r.Name) == "" || len(r.Sensors) == 0 {
			return errors.New("region requires id, workspace, name and sensors")
		}
		key := [2]string{r.WorkspaceID, r.ID}
		if regions[key] {
			return fmt.Errorf("duplicate region %s", key)
		}
		regions[key] = true
		sensors := map[Binding]bool{}
		for _, s := range r.Sensors {
			if strings.TrimSpace(s.ChannelID) == "" || strings.TrimSpace(s.PublisherID) == "" || sensors[s] {
				return fmt.Errorf("invalid or duplicate sensor in %s", key)
			}
			sensors[s] = true
		}
		for _, id := range r.Executors {
			k := [2]string{r.WorkspaceID, id}
			if strings.TrimSpace(id) == "" || executors[k] {
				return fmt.Errorf("executor has duplicate or ambiguous region binding in %s", key)
			}
			executors[k] = true
		}
		if r.SoilMoistureMin != nil && (math.IsNaN(*r.SoilMoistureMin) || math.IsInf(*r.SoilMoistureMin, 0) || *r.SoilMoistureMin < 0 || *r.SoilMoistureMin > 100) {
			return errors.New("soil_moisture_min must be between 0 and 100")
		}
		if r.IrrigationDurationMin != nil && (math.IsNaN(*r.IrrigationDurationMin) || math.IsInf(*r.IrrigationDurationMin, 0) || *r.IrrigationDurationMin <= 0 || *r.IrrigationDurationMin > 60) {
			return errors.New("irrigation_duration_min must be greater than 0 and at most 60")
		}
	}
	return nil
}
func (reg Registry) Find(workspace, id string) (Region, error) {
	for _, r := range reg.Regions {
		if r.WorkspaceID == workspace && r.ID == id {
			return r, nil
		}
	}
	return Region{}, errors.New("region does not exist in requested workspace")
}
