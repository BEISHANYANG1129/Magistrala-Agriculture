"""Publish one synthetic sample per authorized test device, then verify exact stored samples."""
import argparse,base64,json,pathlib,time
from platform_inspect import login
from platform_smoke import request

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--env-file',required=True);args=parser.parse_args()
    runtime=pathlib.Path(__file__).resolve().parents[1]/'runtime'
    token=login(args.env_file);regions=[];report=[]
    for region,filename,value in [('north','test3-session.json',12),('south','test3-south-session.json',35)]:
        state=json.loads((runtime/filename).read_text())
        assert state['workspace_id']=='cccd00c1-a12c-4d6e-9aa9-d2c52906f2e5'
        assert state['channel_id']=='3e692d54-5ccc-4a83-9cfd-ac0080263327'
        payload=[{'n':'soil_moisture','u':'%','v':value,'t':round(time.time(),3)}]
        auth='Basic '+base64.b64encode((state['device_id']+':'+state['device_key']).encode()).decode()
        status,_=request('http://localhost/http/m/'+state['workspace_id']+'/c/'+state['channel_id'],auth,payload,'application/senml+json')
        if status!=200:raise RuntimeError('Publish failed for '+region)
        url='http://localhost:9011/'+state['workspace_id']+'/channels/'+state['channel_id']+'/messages?limit=100&order=time&dir=desc&publisher='+state['device_id']
        matched=[]
        for _ in range(10):
            read_status,page=request(url,'Bearer '+token)
            if read_status!=200:raise RuntimeError('Reader failed for '+region)
            matched=[m for m in page['messages'] if m.get('publisher')==state['device_id'] and m.get('name')=='soil_moisture' and m.get('value')==value and abs(m.get('time',0)/1e9-payload[0]['t'])<0.01]
            if matched:break
            time.sleep(.5)
        if not matched:raise RuntimeError('Stored sample not found for '+region)
        report.append({'region':region,'publish_status':status,'reader_status':read_status,'sample':matched[0]})
        regions.append({'id':region,'name':('北区' if region=='north' else '南区')+' · 平台合成数据','workspace_id':state['workspace_id'],'sensors':[{'channel_id':state['channel_id'],'publisher_id':state['device_id']}],'executors':['simulation-only-valve-'+region],'soil_moisture_min':20,'irrigation_duration_min':5})
    registry_path=runtime/'dual-region-registry.json'
    if not registry_path.exists():
        registry_path.write_text(json.dumps({'version':'dual-live-v1','regions':regions},ensure_ascii=False,indent=2),encoding='utf-8')
    (runtime/'dual-region-publish.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
    print(json.dumps(report,ensure_ascii=False))

if __name__=='__main__':main()
