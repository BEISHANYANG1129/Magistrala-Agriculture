import test from 'node:test'
import assert from 'node:assert/strict'
import http from 'node:http'
import { cameraPlugin, createCameraGateway } from './camera-gateway.mjs'

test('Vite middleware registration does not return a post-configure hook', () => {
  let registrations = 0
  const server = { middlewares: { use(handler) { assert.equal(typeof handler, 'function'); registrations++; return this } } }
  const plugin = cameraPlugin()
  assert.equal(plugin.configureServer(server), undefined)
  assert.equal(plugin.configurePreviewServer(server), undefined)
  assert.equal(registrations, 2)
})

const workspace = '11111111-1111-1111-1111-111111111111'
const deviceId = '22222222-2222-2222-2222-222222222222'
const endpoint = `/api/cameras/${workspace}/${deviceId}/status`
const authorization = 'Bearer test-token-012345678901234567890'
const device = { id: deviceId, tenantId: workspace, kind: 'device', status: 'active',
  attributes: { metadata: { camera: { provider: 'jooan-tutk', bindingId: 'jooan-g4s4' } } } }

async function fixture(record, run, allowControl = true) {
  let authorizationCalls = 0, bridgeCalls = 0, bridgeHeaders
  const bridge = http.createServer((req, res) => {
    bridgeCalls++; bridgeHeaders = req.headers
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ state: 'idle' }))
  })
  await new Promise((resolve) => bridge.listen(0, '127.0.0.1', resolve))
  const gateway = createCameraGateway({ bridgePort: bridge.address().port, fetch: async (_url, options) => {
    authorizationCalls++
    assert.equal(options.headers.Authorization, authorization)
    if (_url.endsWith('/auth/introspect')) return { ok:true, json:async()=>({active:true, entity_id:workspace}) }
    if (options.body?.includes('authzCheck')) return { ok:true, json:async()=>({data:{authzCheck:{allowed:allowControl}}}) }
    return { ok: true, json: async () => ({ data: { entity: record } }) }
  } })
  const server = http.createServer((req, res) => gateway(req, res, () => { res.writeHead(404); res.end() }))
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  try {
    await run({ origin: `http://127.0.0.1:${server.address().port}`,
      calls: () => ({ authorizationCalls, bridgeCalls, bridgeHeaders }) })
  } finally {
    await Promise.all([new Promise((r) => server.close(r)), new Promise((r) => bridge.close(r))])
  }
}

test('unauthenticated video requests never reach Atom or the bridge', () => fixture(device, async ({ origin, calls }) => {
  const response = await fetch(origin + endpoint)
  assert.equal(response.status, 401)
  assert.equal(calls().authorizationCalls, 0)
  assert.equal(calls().bridgeCalls, 0)
}))

test('view-only device access cannot move the camera', () => fixture(device, async ({origin,calls}) => {
  const response = await fetch(origin + `/api/cameras/${workspace}/${deviceId}/ptz/move`, {method:'POST', headers:{Authorization:authorization}, body:JSON.stringify({requestId:'a'.repeat(32),direction:'left',durationMs:400})})
  assert.equal(response.status,403)
  assert.equal(calls().bridgeCalls,0)
}, false))

test('PTZ rejects unbounded commands and forwards only a verified actor with valid commands', () => fixture(device, async ({origin,calls}) => {
  const url = origin + `/api/cameras/${workspace}/${deviceId}/ptz/move`
  for (const command of [{direction:'left',durationMs:1001},{direction:'reset',durationMs:400},{direction:'left',durationMs:400,command:69633}]) {
    assert.equal((await fetch(url,{method:'POST',headers:{Authorization:authorization},body:JSON.stringify({requestId:'a'.repeat(32),...command})})).status,400)
  }
  assert.equal(calls().bridgeCalls,0)
  assert.equal((await fetch(url,{method:'POST',headers:{Authorization:authorization},body:JSON.stringify({requestId:'b'.repeat(32),direction:'left',durationMs:400})})).status,200)
  assert.equal(calls().bridgeHeaders['x-agri-camera-actor'],workspace)
  assert.equal(calls().bridgeHeaders.authorization,undefined)
}))

test('continuous playback controls require login and POST without forwarding platform credentials', () => fixture(device, async ({ origin, calls }) => {
  const base = `/api/cameras/${workspace}/${deviceId}/playback/`
  assert.equal((await fetch(origin + base + 'start', { method: 'POST' })).status, 401)
  assert.equal((await fetch(origin + base + 'start', { headers: { Authorization: authorization } })).status, 404)
  assert.equal(calls().bridgeCalls, 0)
  assert.equal((await fetch(origin + base + 'stop', { method: 'POST', headers: { Authorization: authorization } })).status, 200)
  assert.equal(calls().bridgeHeaders.authorization, undefined)
}))

test('disabled camera devices cannot enable continuous playback', () => fixture({ ...device, status: 'disabled' }, async ({ origin, calls }) => {
  const path = `/api/cameras/${workspace}/${deviceId}/playback/start`
  assert.equal((await fetch(origin + path, { method: 'POST', headers: { Authorization: authorization } })).status, 403)
  assert.equal(calls().bridgeCalls, 0)
}))

test('device read permission does not authorize a different workspace', () => fixture({ ...device, tenantId: 'another-workspace' }, async ({ origin, calls }) => {
  assert.equal((await fetch(origin + endpoint, { headers: { Authorization: authorization } })).status, 404)
  assert.equal(calls().bridgeCalls, 0)
}))

test('a regular sensor cannot be used to reach the camera binding', () => fixture({ ...device, attributes: {} }, async ({ origin, calls }) => {
  assert.equal((await fetch(origin + endpoint, { headers: { Authorization: authorization } })).status, 404)
  assert.equal(calls().bridgeCalls, 0)
}))

test('disabled devices cannot start or view camera sessions', () => fixture({ ...device, status: 'disabled' }, async ({ origin, calls }) => {
  assert.equal((await fetch(origin + endpoint, { headers: { Authorization: authorization } })).status, 403)
  assert.equal(calls().bridgeCalls, 0)
}))

test('the authorized gateway does not send platform credentials to the native bridge', () => fixture(device, async ({ origin, calls }) => {
  const response = await fetch(origin + endpoint, { headers: { Authorization: authorization } })
  assert.equal(response.status, 200)
  assert.equal((await response.json()).state, 'idle')
  assert.equal(calls().bridgeHeaders.authorization, undefined)
  assert.equal(calls().bridgeHeaders['x-agri-camera-proxy'], '1')
}))

test('cross-origin requests are rejected before authentication or camera activity', () => fixture(device, async ({ origin, calls }) => {
  const response = await fetch(origin + endpoint, { headers: { Authorization: authorization, Origin: 'https://unrelated.example' } })
  assert.equal(response.status, 403)
  assert.equal(calls().authorizationCalls, 0)
  assert.equal(calls().bridgeCalls, 0)
}))

test('arbitrary media paths cannot be proxied to local files', () => fixture(device, async ({ origin, calls }) => {
  const path = `/api/cameras/${workspace}/${deviceId}/sessions/${'a'.repeat(32)}/media/g-${'b'.repeat(32)}/0/private.dpapi`
  assert.equal((await fetch(origin + path, { headers: { Authorization: authorization } })).status, 404)
  assert.equal(calls().authorizationCalls, 0)
  assert.equal(calls().bridgeCalls, 0)
}))

test('all three presentation views are authorized while unknown views are rejected', () => fixture(device, async ({ origin, calls }) => {
  const base = `/api/cameras/${workspace}/${deviceId}/sessions/${'a'.repeat(32)}/media/g-${'b'.repeat(32)}/`
  for (const view of [0, 1, 2])
    assert.equal((await fetch(origin + base + `${view}/index.m3u8`, { headers: { Authorization: authorization } })).status, 200)
  assert.equal((await fetch(origin + base + '3/index.m3u8', { headers: { Authorization: authorization } })).status, 404)
  assert.equal(calls().bridgeCalls, 3)
}))


test('server snapshots require read access and identity but not motor write permission', () => fixture(device, async ({origin,calls}) => {
 const url=origin+`/api/cameras/${workspace}/${deviceId}/snapshots`
 assert.equal((await fetch(url,{method:'POST',headers:{Authorization:authorization},body:JSON.stringify({requestId:'a'.repeat(32),generation:'g-'+ 'b'.repeat(32),view:0,taskId:'c'.repeat(32)})})).status,200)
 assert.equal(calls().bridgeHeaders['x-agri-camera-actor'],workspace)
 assert.equal(calls().bridgeHeaders.authorization,undefined)
 assert.equal((await fetch(url,{method:'POST',headers:{Authorization:authorization},body:JSON.stringify({requestId:'a'.repeat(32),generation:'../../private',view:0})})).status,400)
 assert.equal(calls().bridgeCalls,1)
}, false))

test('task-scoped emergency stop forwards a bounded task ID and rejects arbitrary metadata', () => fixture(device, async ({origin,calls})=>{
 const url=origin+`/api/cameras/${workspace}/${deviceId}/ptz/stop`
 assert.equal((await fetch(url,{method:'POST',headers:{Authorization:authorization},body:JSON.stringify({requestId:'a'.repeat(32),taskId:'b'.repeat(32)})})).status,200)
 assert.equal((await fetch(url,{method:'POST',headers:{Authorization:authorization},body:JSON.stringify({requestId:'a'.repeat(32),taskId:'../../other',command:1})})).status,400)
 assert.equal(calls().bridgeCalls,1)
}))
