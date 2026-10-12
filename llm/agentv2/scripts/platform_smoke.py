"""Publish synthetic telemetry to an explicitly authorized test scope; save redacted evidence."""
import argparse,base64,json,pathlib,time,urllib.request,urllib.error
from platform_inspect import login,WORKSPACE

def request(url,auth,data=None,content_type="application/json"):
    req=urllib.request.Request(url,json.dumps(data).encode() if data is not None else None,{"Authorization":auth,"Content-Type":content_type})
    try:
        with urllib.request.urlopen(req,timeout=15) as response:
            raw=response.read().decode()
            return response.status,json.loads(raw) if raw else None
    except urllib.error.HTTPError as error:
        return error.code,{"error":error.read().decode()[:500]}
def main():
    parser=argparse.ArgumentParser();parser.add_argument("--env-file",required=True)
    parser.add_argument("--scope",choices=["isolated","test3"],default="isolated")
    parser.add_argument("--time-unit",choices=["seconds","nanoseconds"],default="nanoseconds")
    args=parser.parse_args()
    runtime=pathlib.Path("runtime")
    prefix="test3-" if args.scope=="test3" else ""
    state=json.loads((runtime/("test3-session.json" if prefix else "platform-session.json")).read_text())
    workspace="cccd00c1-a12c-4d6e-9aa9-d2c52906f2e5" if prefix else WORKSPACE
    if state["workspace_id"]!=workspace:raise RuntimeError("refusing to use another workspace")
    if prefix and state["channel_id"]!="3e692d54-5ccc-4a83-9cfd-ac0080263327":raise RuntimeError("unexpected test3 channel")
    token=login(args.env_file)
    read_url="http://localhost:9011/"+workspace+"/channels/"+state["channel_id"]+"/messages?offset=0&limit=100&order=time&dir=desc&publisher="+state["device_id"]
    rules_status,rules=request("http://localhost:9008/"+workspace+"/rules","Bearer "+token)
    if rules_status!=200:raise RuntimeError("cannot read rules")
    (runtime/(prefix+"rules-response.json")).write_text(json.dumps(rules,indent=2),encoding="utf-8")
    payload=[{"n":"soil_moisture","u":"%","v":12,"t":round(time.time(),3)}]
    auth="Basic "+base64.b64encode((state["device_id"]+":"+state["device_key"]).encode()).decode()
    publish_url="http://localhost/http/m/"+workspace+"/c/"+state["channel_id"]
    status,published=request(publish_url,auth,payload,"application/senml+json")
    result={"workspace_id":workspace,"publish":{"url":publish_url,"authorization":"Basic <device_id:shared_key>","request":payload,"status":status,"response":published}}
    matched=[]
    for _ in range(10):
        read_status,messages=request(read_url,"Bearer "+token)
        scale=1e9 if args.time_unit=="nanoseconds" else 1
        matched=[m for m in messages.get("messages",[]) if m.get("publisher")==state["device_id"] and m.get("name")=="soil_moisture" and abs(float(m.get("time",0))/scale-payload[0]["t"])<0.01]
        if matched:break
        time.sleep(1)
    result["read"]={"url":read_url,"authorization":"Bearer <user_token>","time_unit":args.time_unit,"status":read_status,"response":messages,"exact_sample_found":bool(matched)}
    (runtime/(prefix+"platform-smoke.json")).write_text(json.dumps(result,indent=2),encoding="utf-8")
    registry={"version":"live-smoke-v1","regions":[{"id":"north","name":"北区 · 平台合成数据","workspace_id":workspace,"sensors":[{"channel_id":state["channel_id"],"publisher_id":state["device_id"]}],"executors":["simulation-only-valve-north"],"soil_moisture_min":20}]}
    (runtime/(prefix+"registry.live.json")).write_text(json.dumps(registry,ensure_ascii=False,indent=2),encoding="utf-8")
    print(json.dumps(result,ensure_ascii=False))
    if status not in (200,202) or not matched:raise SystemExit(1)
if __name__=="__main__":main()
