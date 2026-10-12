"""Test a copied legacy control service with real adapter output; no shared process or port used."""
import argparse,hashlib,json,os,pathlib,shutil,subprocess,uuid

TEST=r'''package api
import("bytes";"encoding/json";"net/http/httptest";"os";"path/filepath";"testing";"time";
"agri-control-service/internal/model";"agri-control-service/internal/service";"agri-control-service/internal/logstore")
func TestAuthorizedIsolatedContract(t *testing.T){
 raw,err:=os.ReadFile(os.Getenv("AGRI_CONTRACT_FIXTURE"));if err!=nil{t.Fatal(err)}
 var plans []struct{Task model.Task `json:"task"`};if err=json.Unmarshal(raw,&plans);err!=nil||len(plans)!=1{t.Fatal(err)}
 task:=plans[0].Task
 if task.Target!="valve-north"||task.Params["duration_min"]!=0.001{t.Fatal("unexpected test fixture")}
 store,err:=logstore.NewLogStore(filepath.Join(os.Getenv("AGRI_CONTRACT_LOG_DIR"),"commands.jsonl"));if err!=nil{t.Fatal(err)}
 handler:=NewHandler(service.NewControlService(store,1));body,_:=json.Marshal(task)
 send:=func(){r:=httptest.NewRequest("POST","http://localhost/control/task",bytes.NewReader(body));w:=httptest.NewRecorder();handler.HandleTask(w,r);if w.Code!=200||w.Body.String()!="task executed"{t.Fatal(w.Code,w.Body.String())}}
 wait:=func(count int)[]logstore.LogEntry{
  deadline:=time.Now().Add(3*time.Second)
  for time.Now().Before(deadline){entries,err:=store.ReadAll();if err==nil&&len(entries)>=count{return entries};time.Sleep(10*time.Millisecond)}
  t.Fatal("timed out waiting for legacy executor log");return nil
 }
 send();entries:=wait(2)
 if entries[0].Command!="open_valve"||entries[1].Command!="close_valve"{t.Fatal(entries)}
 for _,entry:=range entries{if entry.DeviceID!=task.Target||entry.TraceID!=task.TraceID||entry.TaskID!=task.TaskID{t.Fatal("routing mismatch",entry)}}
 t.Log("adapter payload accepted; actual legacy planner/executor produced open and close for north only")
 // Characterize a real legacy limitation: queue admission is not a completion receipt, nor idempotent.
 send();entries=wait(4)
 if len(entries)!=4{t.Fatal("unexpected command count")}
 t.Log("legacy same-ID resubmission produced another open/close; agent-level deduplication remains required")
}
'''

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--go',required=True);args=parser.parse_args()
 module=pathlib.Path(__file__).resolve().parents[1];source=module.parents[1]/'agriControlService'
 runtime=module/'runtime';destination=runtime/('legacy-contract-'+uuid.uuid4().hex[:10]);destination.mkdir()
 assert destination.resolve().is_relative_to(runtime.resolve())
 paths=[p for p in source.rglob('*.go') if 'vendor' not in p.parts]+[source/'go.mod',source/'go.sum']
 before={str(p.relative_to(source)):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}
 for path in paths:
  target=destination/path.relative_to(source);target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(path,target)
 registry=json.loads((module/'examples/registry.json').read_text(encoding='utf-8'))
 registry['regions'][0]['irrigation_duration_min']=0.001
 (destination/'fixture-registry.json').write_text(json.dumps(registry),encoding='utf-8')
 env=os.environ.copy();env.update({'GOMAXPROCS':'2','GOTOOLCHAIN':'local','GOPROXY':'off'})
 fixture=subprocess.run([args.go,'run','-p','1','./cmd/control-contract','-registry',str(destination/'fixture-registry.json')],cwd=module,env=env,check=True,capture_output=True)
 fixture_path=destination/'adapter-payload.json';fixture_path.write_bytes(fixture.stdout)
 (destination/'internal/api/agent_contract_test.go').write_text(TEST,encoding='utf-8')
 env['AGRI_CONTRACT_FIXTURE']=str(fixture_path)
 env['AGRI_CONTRACT_LOG_DIR']=str(destination)
 result=subprocess.run([args.go,'test','-mod=readonly','-p','1','-count=1','-run','TestAuthorizedIsolatedContract','-v','./internal/api'],cwd=destination,env=env,capture_output=True,text=True)
 after={str(p.relative_to(source)):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}
 report={'exit_code':result.returncode,'source_unchanged':before==after,'source_hashes':before,'isolated_copy':str(destination),'output':result.stdout+result.stderr}
 (runtime/'legacy-contract-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
 print(result.stdout+result.stderr);print('Original source unchanged:',before==after)
 if result.returncode or before!=after:raise SystemExit(1)

if __name__=='__main__':main()
