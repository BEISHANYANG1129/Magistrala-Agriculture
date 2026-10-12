"""Explicit local model evaluation. Never restarts services or controls real devices."""
import argparse
import datetime
import json
import os
import pathlib
import re
import subprocess
import sys
import time
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]


def command(*args):
    return subprocess.check_output(args, text=True, encoding="utf-8", errors="replace", timeout=20)


def gpu_check():
    # This conservative check also treats a loaded Ollama model as potentially
    # owned by another task. Never evict it to make room for this evaluation.
    with urllib.request.urlopen("http://localhost:11434/api/ps", timeout=5) as response:
        if json.load(response).get("models"):
            raise RuntimeError("Ollama already has a loaded model; wait without unloading it")
    gpu = command("nvidia-smi", "--query-gpu=utilization.gpu,memory.used", "--format=csv,noheader,nounits").strip()
    for row in gpu.splitlines():
        utilization, memory = (int(x.strip()) for x in row.split(","))
        if utilization > 15 or memory > 4096:
            raise RuntimeError("GPU activity exceeds evaluation idle limits; wait")
    table = command("nvidia-smi")
    if re.search(r"\|\s*\d+\s+N/A\s+N/A\s+\d+\s+C\s+", table):
        raise RuntimeError("GPU compute process found; wait")
    checked = []
    names = command("docker", "ps", "--format", "{{.Names}}").splitlines()
    inspections = command("docker", "inspect", "--format", "{{.Name}} {{json .HostConfig.DeviceRequests}}", *names).splitlines() if names else []
    for inspection in inspections:
        name, encoded_requests = inspection.split(" ", 1)
        name = name.lstrip("/")
        device_requests = json.loads(encoded_requests)
        if device_requests and any("gpu" in capability for request in device_requests for capability in request.get("Capabilities", [])):
            processes = command("docker", "exec", name, "nvidia-smi", "--query-compute-apps=pid,process_name", "--format=csv,noheader").strip()
            if processes:
                raise RuntimeError("A GPU-enabled container reports compute processes; wait")
            checked.append(name)
    return {"checked_at": datetime.datetime.now(datetime.timezone.utc).isoformat(), "gpu": gpu, "gpu_containers": checked, "ollama_models": []}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check-only", action="store_true")
    parser.add_argument("--env-file")
    parser.add_argument("--model", default="gemma3:12b")
    parser.add_argument("--cases", nargs="+", choices=["north", "south", "no_threshold", "two_experts", "missing", "stale"], default=["north", "south", "no_threshold", "two_experts", "missing", "stale"])
    args = parser.parse_args()
    check = wait_for_idle()
    if args.check_only:
        print(json.dumps(check))
        return
    if not args.env_file:
        parser.error("--env-file is required for evaluation")
    from platform_inspect import login
    token = login(args.env_file)
    output = ROOT / "runtime" / ("model-eval-" + datetime.datetime.now().strftime("%Y%m%d-%H%M%S-%f"))
    output.mkdir()
    env = os.environ.copy()
    env.update(AGRI_MODEL_EVAL="yes", AGRI_GPU_IDLE_CONFIRMED="yes", AGRI_LIVE_MODEL=args.model,
               AGRI_READER_AUTH="Bearer " + token, AGRI_LIVE_REGISTRY=str(ROOT / "runtime/dual-region-registry.json"),
               AGRI_EVAL_OUTPUT=str(output), AGRI_EVAL_GPU_GUARD=str(pathlib.Path(__file__).resolve()),
               AGRI_EVAL_PYTHON=sys.executable, GOMAXPROCS="2")
    summary = {"model": args.model, "initial_gpu_check": check, "cases": [], "output": str(output)}
    print("Evaluation output: " + str(output), flush=True)
    for case in args.cases:
        before = wait_for_idle()
        print("Running " + case, flush=True)
        run = subprocess.run([str(ROOT / "runtime/model-eval.test.exe"), "-test.v", "-test.count=1", "-test.timeout=240s", "-test.run=^TestLiveModelEvaluation$/^" + case + "$"], cwd=ROOT, env=env, text=True, encoding="utf-8", errors="replace", stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=250)
        (output / (case + ".log")).write_text(run.stdout, encoding="utf-8")
        summary["cases"].append({"case": case, "exit_code": run.returncode, "gpu_check": before})
        (output / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(run.stdout, flush=True)
        if run.returncode:
            raise SystemExit(run.returncode)
    print("All selected cases passed; no physical device dispatch", flush=True)


def wait_for_idle():
    # Driver utilization and released VRAM can lag the completed response.
    # Briefly wait, never cancel processes or unload another task's model.
    deadline = time.monotonic() + 20
    while True:
        try:
            return gpu_check()
        except RuntimeError:
            if time.monotonic() >= deadline:
                raise
            time.sleep(2)


if __name__ == "__main__":
    main()
