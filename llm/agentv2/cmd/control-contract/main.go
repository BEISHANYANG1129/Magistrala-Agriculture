// Export an actual adapter payload for a separate copy of the legacy control service.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	agent "magistrala-agri-agent"
	"os"
	"time"
)

func main() {
	path := flag.String("registry", "", "explicit synthetic test registry")
	flag.Parse()
	reg, _, err := agent.LoadRegistry(*path)
	if err != nil {
		log.Fatal(err)
	}
	engine := agent.Engine{RegistryPath: *path, Source: agent.DemoSource{}, Analyst: agent.DemoAnalyst{}, MaxAge: time.Minute, Mode: "contract_fixture"}
	r, err := engine.Analyze(context.Background(), agent.Request{WorkspaceID: reg.Regions[0].WorkspaceID, RegionID: reg.Regions[0].ID, MappingVersion: reg.Version})
	if err != nil {
		log.Fatal(err)
	}
	plans, err := agent.BuildControlPlans(reg, r)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(plans); err != nil {
		log.Fatal(err)
	}
}
