import http from 'node:http'
import { registerCameraAutostart } from './camera-startup.mjs'

const uuid = '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}'
const routePattern = new RegExp(`^/api/cameras/(${uuid})/(${uuid})/(status|capabilities|snapshots|ptz/(?:move|stop)|playback/(?:start|stop)|sessions(?:/[a-f0-9]{32}(?:/heartbeat|/media/g-[a-f0-9]{32}/[012]/(?:index\\.m3u8|segment-\\d+\\.ts))?)?)$`)

export function createCameraGateway({
  atomUrl = process.env.AGRI_ATOM_URL || 'http://127.0.0.1:80',
  bridgePort = Number(process.env.AGRI_CAMERA_BRIDGE_PORT || 9030),
  fetch: fetcher = globalThis.fetch,
} = {}) {
  const reply = (res, status, message) => {
    res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'no-store' })
    res.end(JSON.stringify({ message }))
  }
  return async function cameraGateway(req, res, next) {
    const path = (req.url || '').split('?')[0]
    if (!path.startsWith('/api/cameras/')) return next?.()
    const match = routePattern.exec(path)
    if (!match || !['GET', 'POST', 'DELETE'].includes(req.method)) return reply(res, 404, 'Camera endpoint not found.')
    if ((match[3] === 'snapshots' || match[3].startsWith('playback/') || match[3].startsWith('ptz/')) && req.method !== 'POST') return reply(res, 404, 'Camera endpoint not found.')
    const origin = req.headers.origin
    if (origin && origin !== `http://${req.headers.host}` && origin !== `https://${req.headers.host}`)
      return reply(res, 403, 'Invalid request origin.')
    const authorization = req.headers.authorization
    if (!/^Bearer [A-Za-z0-9._~+-]{20,4096}$/i.test(authorization || ''))
      return reply(res, 401, 'Sign in to view camera video.')
    const [, workspaceId, deviceId, tail] = match
    try {
      // Use the existing platform token only with Atom; never forward it to the device bridge.
      const result = await fetcher(`${atomUrl}/graphql`, {
        method: 'POST', signal: AbortSignal.timeout(10000),
        headers: { Authorization: authorization, 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: 'query($id:ID!){entity(id:$id){id tenantId kind status attributes}}', variables: { id: deviceId } }),
      })
      const data = await result.json()
      if (!result.ok || data.errors?.length) return reply(res, 403, 'You cannot access this camera device.')
      const device = data.data?.entity
      const camera = device?.attributes?.metadata?.camera || device?.attributes?.camera
      if (!device || device.tenantId !== workspaceId || device.kind !== 'device' ||
          camera?.provider !== 'jooan-tutk' || camera?.bindingId !== 'jooan-g4s4')
        return reply(res, 404, 'Camera device not found in this workspace.')
      if (['disabled', 'deleted'].includes(device.status)) return reply(res, 403, 'This camera device is disabled.')
      let body, actor
      if (tail.startsWith('ptz/') || tail === 'snapshots') {
        // Physical movement requires write permission, beyond permission to view video.
        const identityResult = await fetcher(`${atomUrl}/auth/introspect`, { headers: { Authorization: authorization }, signal: AbortSignal.timeout(10000) })
        const identity = await identityResult.json()
        if (!identityResult.ok || identity.active !== true || !new RegExp(`^${uuid}$`).test(identity.entity_id || ''))
          return reply(res, 403, 'Camera control identity could not be verified.')
        if (tail.startsWith('ptz/')) {
        const permissionResult = await fetcher(`${atomUrl}/graphql`, { method: 'POST', signal: AbortSignal.timeout(10000),
          headers: { Authorization: authorization, 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: 'mutation($input:AuthzCheckInput!){authzCheck(input:$input){allowed}}', variables: { input: {
            subjectId: identity.entity_id, action: 'write', objectKind: 'entity', objectId: deviceId, resourceId: deviceId,
            context: { workspace_id: workspaceId },
          } } }) })
        const permission = await permissionResult.json()
        if (!permissionResult.ok || permission.errors?.length || permission.data?.authzCheck?.allowed !== true)
          return reply(res, 403, 'You do not have permission to control this camera.')
        }
        actor = identity.entity_id
        let raw = ''
        for await (const chunk of req) { raw += chunk; if (Buffer.byteLength(raw) > 1024) return reply(res, 413, 'PTZ request is too large.') }
        let command
        try { command = JSON.parse(raw) } catch { return reply(res, 400, 'Invalid PTZ JSON.') }
        const move = tail === 'ptz/move'
        const snapshot = tail === 'snapshots'
        const keys = snapshot ? ['requestId', 'generation', 'view'] : move ? ['requestId', 'direction', 'durationMs'] : ['requestId']
        if (command?.taskId !== undefined) keys.push('taskId')
        if (snapshot && command?.afterRequestId !== undefined) keys.push('afterRequestId')
        if (!command || typeof command !== 'object' || Array.isArray(command) || Object.keys(command).sort().join() !== keys.sort().join() ||
            !/^[a-f0-9]{32}$/.test(command.requestId || '') ||
            (command.taskId !== undefined && !/^[a-f0-9]{32}$/.test(command.taskId)) ||
            (snapshot && (!/^g-[a-f0-9]{32}$/.test(command.generation || '') || ![0, 1, 2].includes(command.view) || (command.afterRequestId !== undefined && !/^[a-f0-9]{32}$/.test(command.afterRequestId)))) ||
            (move && (!['left', 'right', 'up', 'down'].includes(command.direction) || !Number.isInteger(command.durationMs) || command.durationMs < 100 || command.durationMs > 1000)))
          return reply(res, 400, 'Invalid PTZ fields, direction or duration (100..1000 ms).')
        body = JSON.stringify(command)
      }
      const proxy = http.request({ hostname: '127.0.0.1', port: bridgePort,
        path: `/v1/${workspaceId}/${deviceId}/${tail}`, method: req.method,
        headers: { 'X-Agri-Camera-Proxy': '1', 'Content-Type': 'application/json',
          ...(body ? { 'Content-Length': Buffer.byteLength(body), 'X-Agri-Camera-Actor': actor } : {}) }, timeout: tail === 'snapshots' ? 35000 : 20000,
      }, (response) => {
        res.writeHead(response.statusCode || 502, {
          'Content-Type': response.headers['content-type'] || 'application/json',
          'Cache-Control': 'no-store',
          ...(response.headers['content-length'] ? { 'Content-Length': response.headers['content-length'] } : {}),
        })
        response.pipe(res)
      })
      proxy.on('timeout', () => proxy.destroy())
      proxy.on('error', () => {
        if (!res.headersSent) reply(res, 503, 'Camera bridge is unavailable. Retry shortly.')
        else res.destroy()
      })
      res.on('close', () => { if (!res.writableEnded) proxy.destroy() })
      proxy.end(body)
    } catch {
      if (!res.headersSent) reply(res, 503, 'Camera authorization service is temporarily unavailable.')
    }
  }
}

export function cameraPlugin() {
  const setup = (server) => {
    server.middlewares.use(createCameraGateway())
    registerCameraAutostart(server)
  }
  return { name: 'agri-camera-gateway', configureServer: setup, configurePreviewServer: setup }
}
