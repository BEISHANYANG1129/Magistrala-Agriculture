import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..')
const service=process.argv[2]
const config={image:['image_upload/image_upload_server_main',['run','.']],control:['agriControlService',['run','./cmd/server']],llm:['llm',['run','./llm_api']],executor:['agriDeviceExecutor',['run','./cmd']],integration:['agriDataIntegration',['run','./cmd']]}
let command='go',directory,args
if (service==='ui') {
  command=process.platform==='win32'?'npm.cmd':'npm'
  directory='magistrala-ui';args=['run','dev']
} else if(service==='agent-ui') {
  directory='llm/agentv2'
  const state=path.join(root,directory,'runtime/agri-ui')
  fs.mkdirSync(state,{recursive:true})
  const registry=path.join(state,'registry.json')
  if (!fs.existsSync(registry)) fs.copyFileSync(path.join(root,directory,'examples/empty-registry.json'),registry)
  args=['run','./cmd/agent','-listen','127.0.0.1:9025','-platform-url',process.env.AGRI_ATOM_URL || 'http://127.0.0.1:8080','-mode',process.env.AGRI_UI_MODEL?'ollama':'reader-demo','-reader',process.env.AGRI_READER_URL || 'http://127.0.0.1:9011','-time-unit','nanoseconds','-registry',registry,'-edit-regions','-state-dir',path.join(state,'history')]
  if(process.env.AGRI_UI_MODEL) args.push('-model',process.env.AGRI_UI_MODEL)
} else if(config[service]) [directory,args]=config[service]
else {console.error('Usage: node scripts/run-local.mjs ui|agent-ui|control|image|llm|executor|integration');process.exit(1)}
if (command==='npm.cmd') {command=process.env.ComSpec || 'cmd.exe';args=['/d','/s','/c','npm.cmd run dev']}
const child=spawn(command,args,{cwd:path.join(root,directory),stdio:'inherit'})
child.on('error',error=>{console.error(error.message);process.exit(1)})
child.on('exit',code=>process.exit(code ?? 1))
for(const signal of ['SIGINT','SIGTERM']) process.on(signal,()=>child.kill(signal))
