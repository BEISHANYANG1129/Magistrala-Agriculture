// Real camera gateway checks without invoking or changing the busy local model.
// The complete bounded test plan is fixed here: capture, left 100 ms, capture,
// right 100 ms, capture. No position/angle verification is inferred from SDK sends.
import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
const root='D:/personal/magistrala',out=root+'/output/multi-agent-rebuild/implementation'
const workspace='a14a9eeb-da40-4f82-babe-aa3a5204ecbe',device='b6d1c497-3e75-43af-8ab6-6ba723290e2d',base='http://127.0.0.1:5173'
const env=Object.fromEntries((await fs.readFile(root+'/magistrala/docker/.env','utf8')).split(/\r?\n/).filter(line=>line.includes('=')&&!line.trim().startsWith('#')).map(line=>{const at=line.indexOf('=');return [line.slice(0,at),line.slice(at+1).trim().replace(/^['"]|['"]$/g,'')]}))
const browser=await chromium.launch({headless:true,channel:'msedge'}),page=await browser.newPage()
const results={checked_at:new Date().toISOString(),mode:'authenticated_real_tools_without_model',checks:[]}
const taskId=crypto.randomUUID().replaceAll('-','')
async function camera(tail,body){return page.evaluate(async({tail,body,workspace,device})=>{const {request}=await import('/src/api/platform.js');return request(`/api/cameras/${workspace}/${device}/${tail}`,{method:body===undefined?'GET':'POST',body,timeout:45000})},{tail,body,workspace,device})}
async function snapshot(label,generation,afterRequestId){const image=await camera('snapshots',{requestId:crypto.randomUUID().replaceAll('-',''),taskId,generation,view:0,...(afterRequestId?{afterRequestId}:{})});const {image:encoded,...metadata}=image;await fs.writeFile(out+'/camera-tools-'+label+'.jpg',Buffer.from(encoded,'base64'));results.checks.push({label,...metadata});console.log('captured',label,image.segment)}
try{
 await page.goto(base+'/login');await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME);await page.getByLabel('密码',{exact:true}).fill(env.ATOM_ADMIN_SECRET);await page.getByRole('button',{name:'登录',exact:true}).click();await page.waitForURL('**/domains')
 let status=await camera('status');const readyUntil=Date.now()+60000;while((!status.enabled||status.state!=='ready')&&status.enabled&&Date.now()<readyUntil){await page.waitForTimeout(2000);status=await camera('status')}results.initial_status=status
 if(!status.enabled||status.state!=='ready')throw new Error('Camera is not ready; saved playback switch was not changed.')
 results.capabilities=await camera('capabilities');if(!results.capabilities.serverSnapshot)throw new Error('Snapshot API is unavailable.')
 await snapshot('before',status.generation)
 for(const direction of ['left','right']){
  const requestId=crypto.randomUUID().replaceAll('-','');const receipt=await camera('ptz/move',{requestId,taskId,direction,durationMs:100});results.checks.push({label:direction,receipt});console.log('receipt',direction,receipt.status)
  await page.waitForTimeout(1600);await snapshot(direction,status.generation,requestId)
 }
 const stop=await camera('ptz/stop',{requestId:crypto.randomUUID().replaceAll('-',''),taskId});results.checks.push({label:'emergency_stop',receipt:stop})
 try{await camera('ptz/move',{requestId:crypto.randomUUID().replaceAll('-',''),taskId,direction:'left',durationMs:100});throw new Error('Stopped task accepted a new move')}catch(error){if(error.message==='Stopped task accepted a new move')throw error;results.checks.push({label:'late_command_rejected',message:error.message})}
 results.passed=true
}catch(error){results.passed=false;results.error=error.message;process.exitCode=1;console.error(error.message)}finally{await fs.writeFile(out+'/camera-tools-smoke.json',JSON.stringify(results,null,2));await fs.writeFile(out+'/camera-tools-'+Date.now()+'.json',JSON.stringify(results,null,2));await browser.close()}
