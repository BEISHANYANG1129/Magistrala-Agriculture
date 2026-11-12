import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'
import { agentOptions } from './agent-options.mjs'

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..')
const service=process.argv[2]
const config={image:['image_upload/image_upload_server_main',['run','.']],control:['agriControlService',['run','./cmd/server']],llm:['llm',['run','./llm_api']],executor:['agriDeviceExecutor',['run','./cmd']],integration:['agriDataIntegration',['run','./cmd']]}
let command='go',directory,args
if (service==='ui') {
  command=process.platform==='win32'?'npm.cmd':'npm'
  directory='magistrala-ui';args=['run','dev']
} else if(service==='agent-ui') {
  const agent = agentOptions(root)
  directory=agent.directory
  args=['run','-p','1','./cmd/agent',...agent.args]
} else if(config[service]) [directory,args]=config[service]
else {console.error('Usage: node scripts/run-local.mjs ui|agent-ui|control|image|llm|executor|integration');process.exit(1)}
if (command==='npm.cmd') {command=process.env.ComSpec || 'cmd.exe';args=['/d','/s','/c','npm.cmd run dev']}
const child=spawn(command,args,{cwd:path.join(root,directory),stdio:'inherit'})
child.on('error',error=>{console.error(error.message);process.exit(1)})
child.on('exit',code=>process.exit(code ?? 1))
for(const signal of ['SIGINT','SIGTERM']) process.on(signal,()=>child.kill(signal))
