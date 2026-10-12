import http from 'node:http'
import https from 'node:https'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'
import { createSessionHandler } from '../magistrala-ui/server/session-server.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const dist = path.join(root,'magistrala-ui','dist')
const session = await createSessionHandler()
if (!fs.existsSync(path.join(dist,'index.html'))) throw new Error('Build the UI first: cd magistrala-ui && npm ci && npm run build')
let agent
if (process.env.AGRI_AGENT_BINARY) {
  const directory=path.join(root,'llm','agentv2')
  const state=process.env.AGRI_AGENT_STATE_DIR || path.join(directory,'runtime','agri-ui')
  fs.mkdirSync(state,{recursive:true})
  const registry=path.join(state,'registry.json')
  if (!fs.existsSync(registry)) fs.copyFileSync(path.join(directory,'examples','empty-registry.json'),registry)
  agent=spawn(process.env.AGRI_AGENT_BINARY,['-listen','127.0.0.1:9025','-platform-url',process.env.AGRI_ATOM_URL || 'http://127.0.0.1:8080','-mode','reader-demo','-reader',process.env.AGRI_READER_URL || 'http://127.0.0.1:9011','-time-unit','nanoseconds','-registry',registry,'-edit-regions','-state-dir',path.join(state,'history')],{cwd:directory,stdio:'inherit'})
  agent.on('error',error=>{console.error('Agent failed:',error.message);process.exit(1)})
  agent.on('exit',code=>{console.error('Agent stopped:',code);process.exit(code || 1)})
}
const atom=process.env.AGRI_ATOM_URL || 'http://127.0.0.1:80'
const gateway=process.env.AGRI_GATEWAY_URL || 'http://127.0.0.1:80'
const platform={rules:process.env.AGRI_RULES_URL || 'http://127.0.0.1:9008',alarms:process.env.AGRI_ALARMS_URL || 'http://127.0.0.1:8050',reports:process.env.AGRI_REPORTS_URL || 'http://127.0.0.1:9017',bootstrap:process.env.AGRI_BOOTSTRAP_URL || 'http://127.0.0.1:9010',readers:process.env.AGRI_READER_URL || 'http://127.0.0.1:9011',ui:process.env.AGRI_UI_BACKEND_URL || 'http://127.0.0.1:9097',http:gateway,certs:gateway,notifiers:process.env.AGRI_NOTIFIER_URL || 'http://127.0.0.1:9014'}
function route(url) {
  const pathname=url.split('?')[0]
  if (pathname==='/api/agent' || pathname.startsWith('/api/agent/')) return [process.env.AGRI_AGENT_URL || 'http://127.0.0.1:9025',url.slice('/api/agent'.length) || '/',true]
  for (const [name,target] of Object.entries(platform)) {
    const prefix=`/api/platform/${name}`
    if (pathname===prefix || pathname.startsWith(prefix+'/')) return [target,url.slice(prefix.length) || '/']
  }
  for (const prefix of ['/graphql','/auth']) if (pathname===prefix || pathname.startsWith(prefix+'/')) return [atom,url]
  if (pathname==='/Users' || pathname.startsWith('/Users/')) return [atom,url.replace(/^\/Users/,'/users')]
  if (pathname==='/Messages' || pathname.startsWith('/Messages/')) return [platform.readers,url.replace(/^\/Messages/,'') || '/']
  if (pathname==='/image-upload' || pathname.startsWith('/image-upload/')) return [process.env.AGRI_IMAGE_URL || 'http://127.0.0.1:18080',url.replace(/^\/image-upload/,'') || '/']
  if (pathname==='/agri-api' || pathname.startsWith('/agri-api/')) return [process.env.AGRI_INTEGRATION_URL || 'http://127.0.0.1:8889',url.replace(/^\/agri-api/,'/api')]
}
function proxy(req,res,[base,url,preserveHost]) {
  const target=new URL(base)
  const headers={...req.headers}
  delete headers.connection
  if (!preserveHost) headers.host=target.host
  const upstream=(target.protocol==='https:'?https:http).request({hostname:target.hostname,port:target.port,protocol:target.protocol,method:req.method,path:(target.pathname==='/'?'':target.pathname)+url,headers},response=>{
    res.writeHead(response.statusCode,response.headers)
    response.pipe(res)
  })
  upstream.setTimeout(155000,()=>upstream.destroy(new Error('Upstream timeout')))
  upstream.on('error',()=>{if (!res.headersSent) res.writeHead(502,{'Content-Type':'application/json'});res.end(JSON.stringify({error:'Required backend is unavailable'}))})
  req.on('aborted',()=>upstream.destroy())
  req.pipe(upstream)
}
const types={'.html':'text/html; charset=utf-8','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg','.jpeg':'image/jpeg','.ico':'image/x-icon','.woff2':'font/woff2'}
const server=http.createServer(async(req,res)=>{
  try {
    if (req.url==='/healthz') {res.writeHead(200,{'Content-Type':'application/json'});res.end('{"status":"ok"}');return}
    if (req.url==='/api/session' || req.url.startsWith('/api/session/') || req.url.startsWith('/api/session?')) {await session(req,res);return}
    const target=route(req.url)
    if (target) {proxy(req,res,target);return}
    if (req.url.startsWith('/api/')) {res.writeHead(404);res.end();return}
    if (!['GET','HEAD'].includes(req.method)) {res.writeHead(405);res.end();return}
    const pathname=decodeURIComponent(req.url.split('?')[0])
    let file=path.resolve(dist,'.'+pathname)
    if (file!==dist && !file.startsWith(dist+path.sep)) {res.writeHead(403);res.end();return}
    if (!fs.existsSync(file) || !fs.statSync(file).isFile()) {
      if (path.extname(pathname)) {res.writeHead(404);res.end();return}
      file=path.join(dist,'index.html')
    }
    res.writeHead(200,{'Content-Type':types[path.extname(file)] || 'application/octet-stream','Cache-Control':file.endsWith('index.html')?'no-cache':'public, max-age=3600','X-Content-Type-Options':'nosniff'})
    if (req.method==='HEAD') res.end(); else fs.createReadStream(file).pipe(res)
  } catch {res.writeHead(500);res.end('Request failed')}
})
server.listen(Number(process.env.AGRI_PORT || 5173),process.env.AGRI_HOST || '127.0.0.1',()=>console.log('Agriculture UI and API proxy ready'))
for (const signal of ['SIGINT','SIGTERM']) process.on(signal,()=>{agent?.kill(signal);server.close(()=>process.exit(0));setTimeout(()=>process.exit(0),5000).unref()})

