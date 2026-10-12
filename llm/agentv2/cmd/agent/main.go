package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	agent "magistrala-agri-agent"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "loopback listen address; port 0 selects a free port")
	registry := flag.String("registry", "examples/registry.json", "region registry")
	knowledge := flag.String("knowledge", "examples/knowledge.json", "curated knowledge JSON")
	mode := flag.String("mode", "demo", "demo, reader-demo or ollama")
	reader := flag.String("reader", "", "explicit Magistrala Reader base URL")
	timeUnit := flag.String("time-unit", "seconds", "seconds or nanoseconds (match deployed Reader)")
	model := flag.String("model", "", "explicit Ollama model, no automatic pulls")
	ollamaURL := flag.String("ollama-url", "http://127.0.0.1:11434", "Ollama base URL")
	maxAge := flag.Duration("max-age", 15*time.Minute, "maximum telemetry age")
	stateDir := flag.String("state-dir", "", "optional dedicated history directory")
	editRegions := flag.Bool("edit-regions", false, "enable editing the explicitly selected registry file")
	platformURL := flag.String("platform-url", "", "Atom URL; authenticate integrated UI requests and use caller Reader credentials")
	guardPython := flag.String("gpu-python", "python", "Python executable for GPU guard")
	guardScript := flag.String("gpu-guard", "scripts/evaluate_model.py", "GPU idle checker; required before integrated real inference")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatal(err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		log.Fatal("only an explicit loopback IP may be used")
	}
	if _, _, err := agent.LoadRegistry(*registry); err != nil {
		log.Fatal(err)
	}
	var docs []agent.Knowledge
	b, err := os.ReadFile(*knowledge)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.Unmarshal(b, &docs); err != nil {
		log.Fatal(err)
	}
	engine := &agent.Engine{RegistryPath: *registry, Knowledge: docs, MaxAge: *maxAge, Mode: *mode}
	switch *mode {
	case "demo":
		engine.Source = agent.DemoSource{}
		engine.Analyst = agent.DemoAnalyst{}
	case "ollama":
		if *reader == "" || (*platformURL == "" && os.Getenv("AGRI_READER_AUTH") == "") {
			log.Fatal("ollama mode requires -reader and AGRI_READER_AUTH")
		}
		engine.Source = &agent.Reader{BaseURL: *reader, Authorization: os.Getenv("AGRI_READER_AUTH"), TimeUnit: *timeUnit}
		engine.Analyst, err = agent.NewOllama(*ollamaURL, *model)
		if err != nil {
			log.Fatal(err)
		}
		if *platformURL != "" {
			engine.Analyst = agent.GuardedAnalyst{Inner: engine.Analyst, Python: *guardPython, Script: *guardScript}
		}
	case "reader-demo":
		if *reader == "" || (*platformURL == "" && os.Getenv("AGRI_READER_AUTH") == "") {
			log.Fatal("reader-demo requires -reader and AGRI_READER_AUTH")
		}
		engine.Source = &agent.Reader{BaseURL: *reader, Authorization: os.Getenv("AGRI_READER_AUTH"), TimeUnit: *timeUnit}
		engine.Analyst = agent.DemoAnalyst{}
	default:
		log.Fatal("mode must be demo, reader-demo or ollama")
	}
	if *platformURL != "" && *mode != "demo" {
		engine.Source = agent.RequestReader{BaseURL: *reader, TimeUnit: *timeUnit}
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("agent ready http://%s mode=%s execution=simulation_only", ln.Addr(), *mode)
	handler := agent.NewServer(engine)
	if *stateDir != "" {
		handler, err = agent.NewPersistentServer(engine, *stateDir)
		if err != nil {
			log.Fatal(err)
		}
	}
	handler.EditableRegions = *editRegions
	if *platformURL != "" {
		handler.Platform = &agent.PlatformAuth{URL: *platformURL}
	}
	srv := &http.Server{Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 130 * time.Second, IdleTimeout: 30 * time.Second}
	log.Fatal(srv.Serve(ln))
}
