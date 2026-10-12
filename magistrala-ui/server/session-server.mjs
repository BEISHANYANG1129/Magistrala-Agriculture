import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import crypto from 'node:crypto'
import http from 'node:http'
import { pathToFileURL } from 'node:url'
import remembered from './remembered-session.mjs'

const fields =
  'accessToken accessTokenExpiresAt refreshToken refreshTokenExpiresAt entityId sessionId'
export async function createSessionHandler({
  directory = process.env.AGRI_SESSION_DIR ||
    path.join(
      process.env.LOCALAPPDATA || path.join(os.homedir(), '.local', 'share'),
      'Magistrala',
      'agri-sessions',
    ),
  baseUrl = process.env.AGRI_ATOM_URL || 'http://127.0.0.1:80',
  fetch: fetcher = globalThis.fetch,
  now = Date.now,
} = {}) {
  await fs.mkdir(directory, { recursive: true, mode: 0o700 })
  const keyFile = path.join(directory, 'encryption.key')
  try {
    await fs.writeFile(keyFile, crypto.randomBytes(32).toString('hex'), { flag: 'wx', mode: 0o600 })
  } catch (error) {
    if (error.code !== 'EEXIST') throw error
  }
  const secret = await fs.readFile(keyFile, 'utf8')
  const manager = remembered.createManager({ directory, secret, baseUrl, fetch: fetcher, now })
  const cookieName = 'agri_session'
  const signature = (id) => crypto.createHmac('sha256', secret).update(id).digest('hex')
  function cookieId(req) {
    const value = (req.headers.cookie || '')
      .split(';')
      .map((s) => s.trim())
      .find((s) => s.startsWith(cookieName + '='))
      ?.slice(cookieName.length + 1)
    const [id, mac] = (value || '').split('.')
    return /^[a-f0-9]{64}$/.test(id || '') &&
      /^[a-f0-9]{64}$/.test(mac || '') &&
      crypto.timingSafeEqual(Buffer.from(mac), Buffer.from(signature(id)))
      ? id
      : null
  }
  function setCookie(req, res, id) {
    const secure =
      req.socket.encrypted || process.env.AGRI_COOKIE_SECURE === 'true' ? '; Secure' : ''
    res.setHeader(
      'Set-Cookie',
      `${cookieName}=${id ? id + '.' + signature(id) : ''}; Path=/api/session; HttpOnly; SameSite=Strict; Max-Age=${id ? remembered.COOKIE_SECONDS : 0}${secure}`,
    )
  }
  function reply(res, status, data) {
    res.writeHead(status, {
      'Content-Type': 'application/json; charset=utf-8',
      'Cache-Control': 'no-store',
    })
    res.end(JSON.stringify(data))
  }
  function publicSession(token) {
    return {
      token: token.accessToken,
      expiresAt: new Date(token.accessTokenExpiry).toISOString(),
      entityId: token.user.id,
      sessionId: token.sessionId,
    }
  }
  async function body(req) {
    let text = ''
    for await (const chunk of req) {
      text += chunk
      if (Buffer.byteLength(text) > 16384)
        throw Object.assign(new Error('Request too large'), { status: 413 })
    }
    try {
      return JSON.parse(text)
    } catch {
      throw Object.assign(new Error('Invalid JSON'), { status: 400 })
    }
  }
  const handler = async (req, res, next) => {
    const route = (req.url || '').split('?')[0]
    if (!['/api/session', '/api/session/login', '/api/session/logout'].includes(route)) {
      if (next) return next()
      return reply(res, 404, { message: 'Not found' })
    }
    // Custom header prevents simple cross-site requests; never enable cross-origin access.
    const origin = req.headers.origin
    if (
      req.headers['x-agri-session'] !== '1' ||
      (origin &&
        origin !== `http://${req.headers.host}` &&
        origin !== `https://${req.headers.host}`)
    )
      return reply(res, 403, { message: 'Invalid session request origin' })
    const id = cookieId(req)
    try {
      if (route === '/api/session/login' && req.method === 'POST') {
        const input = await body(req)
        if (
          typeof input.identifier !== 'string' ||
          typeof input.secret !== 'string' ||
          !input.identifier ||
          !input.secret
        )
          return reply(res, 400, { message: 'Enter your identifier and password' })
        const response = await fetcher(baseUrl + '/graphql', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          signal: AbortSignal.timeout(15000),
          body: JSON.stringify({
            query: `mutation($input:LoginInput!){login(input:$input){${fields}}}`,
            variables: {
              input: { identifier: input.identifier, secret: input.secret, kind: 'password' },
            },
          }),
        })
        const result = await response.json()
        if (!response.ok || result.errors?.length)
          return reply(res, response.status >= 500 ? 503 : 401, {
            message:
              response.status >= 500
                ? 'Login service temporarily unavailable'
                : 'Login failed. Check your credentials.',
          })
        const p = result.data.login
        const token = await manager.jwt({
          trigger: 'signIn',
          token: {},
          account: {},
          user: {
            ...p,
            accessTokenExpiry: Date.parse(p.accessTokenExpiresAt),
            refreshTokenExpiry: Date.parse(p.refreshTokenExpiresAt),
            user: { id: p.entityId },
          },
        })
        if (id) await manager.signOut({ token: { rememberId: id } })
        setCookie(req, res, token.rememberId)
        return reply(res, 200, publicSession(token))
      }
      if (route === '/api/session/logout' && req.method === 'POST') {
        if (id) await manager.signOut({ token: { rememberId: id } })
        setCookie(req, res, null)
        return reply(res, 200, { signedOut: true })
      }
      if (route === '/api/session' && req.method === 'GET') {
        if (!id) return reply(res, 401, { message: 'Sign in required' })
        const token = await manager.jwt({ token: { rememberId: id } })
        if (token.error) {
          setCookie(req, res, null)
          return reply(res, 401, { message: 'Session revoked or expired' })
        }
        if (token.sessionUnavailable)
          return reply(res, 503, { message: 'Session service temporarily unavailable' })
        setCookie(req, res, id)
        return reply(res, 200, publicSession(token))
      }
      return reply(res, 405, { message: 'Method not allowed' })
    } catch (error) {
      reply(res, error.status || 503, {
        message: error.status ? error.message : 'Session service temporarily unavailable',
      })
    }
  }
  handler.retryPendingLogouts = () => manager.retryPendingLogouts()
  return handler
}

export function sessionPlugin() {
  async function setup(server) {
    const handler = await createSessionHandler()
    server.middlewares.use(handler)
    const timer = setInterval(() => handler.retryPendingLogouts().catch(() => {}), 60000)
    timer.unref()
    server.httpServer?.once('close', () => clearInterval(timer))
    handler.retryPendingLogouts().catch(() => {})
  }
  return { name: 'agri-persistent-session', configureServer: setup, configurePreviewServer: setup }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const handler = await createSessionHandler()
  const server = http.createServer(handler)
  const timer = setInterval(() => handler.retryPendingLogouts().catch(() => {}), 60000)
  timer.unref()
  server.listen(Number(process.env.AGRI_SESSION_PORT || 5174), '127.0.0.1', () =>
    console.log('Agriculture session proxy listening on loopback'),
  )
}
