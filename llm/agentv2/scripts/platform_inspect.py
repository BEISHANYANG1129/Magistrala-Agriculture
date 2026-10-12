"""Scoped, read-only API inspection. Credentials stay in memory and are never printed."""
import argparse,json,pathlib,urllib.request,urllib.error
WORKSPACE="f7764dd9-f5e4-4902-8737-cf0a8146e7c5"
def settings(path):
    result={}
    for line in pathlib.Path(path).read_text(encoding="utf-8").splitlines():
        if "=" in line and not line.lstrip().startswith("#"):
            k,v=line.split("=",1);result[k]=v.strip().strip('"').strip("'")
    return result
def gql(query,variables=None,token=None):
    headers={"Content-Type":"application/json"}
    if token:headers["Authorization"]="Bearer "+token
    req=urllib.request.Request("http://localhost:8080/graphql",json.dumps({"query":query,"variables":variables or {}}).encode(),headers)
    with urllib.request.urlopen(req,timeout=20) as response:return json.load(response)
def login(envfile):
    s=settings(envfile)
    result=gql('mutation($input:LoginInput!){login(input:$input){token}}',{"input":{"identifier":s["ATOM_ADMIN_USERNAME"],"secret":s["ATOM_ADMIN_SECRET"],"kind":"password"}})
    if result.get("errors"):raise RuntimeError("local Atom login failed")
    return result["data"]["login"]["token"]
if __name__=="__main__":
    parser=argparse.ArgumentParser();parser.add_argument("--env-file",required=True);args=parser.parse_args()
    token=login(args.env_file)
    for key,query in {
        "workspace":'query($id:ID!){tenant(id:$id){id name alias status}}',
        "devices":'query($id:ID!){entities(tenantId:$id,limit:100){items{id name kind profileId attributes}}}',
        "channels":'query($id:ID!){resources(tenantId:$id,kind:"channel",limit:100){items{id name attributes}}}',
        "groups":'query($id:ID!){groups(tenantId:$id,limit:100){items{id name groupType}}}',
    }.items():print(key,json.dumps(gql(query,{"id":WORKSPACE},token),ensure_ascii=False))
