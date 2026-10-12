import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import http from 'node:http'
import { createSessionHandler } from './session-server.mjs'

async function fixture(t) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'agri-session-test-'))
  let time = Date.now(),
    rotations = 0,
    offline = false,
    active = true,
    logouts = 0
  const pair = () => ({
    accessToken: 'access-' + rotations,
    refreshToken: 'atom_rt_secret-' + rotations,
    accessTokenExpiresAt: new Date(time + 3600000).toISOString(),
    refreshTokenExpiresAt: new Date(time + 3153600000000).toISOString(),
    entityId: 'user-1',
    sessionId: 'session-1',
  })
  const fakeFetch = async (url, options) => {
    if (offline) throw new Error('offline')
    if (url.endsWith('/auth/introspect')) return Response.json({ active })
    const body = JSON.parse(options.body)
    if (body.query.includes('RememberedDeviceLogout')) {
      logouts++
      active = false
      return Response.json({ data: { logout: true } })
    }
    if (body.query.includes('RememberedDeviceRefresh')) {
      if (!active) return Response.json({ errors: [{ message: 'invalid refresh token' }] })
      rotations++
      return Response.json({ data: { refreshToken: pair() } })
    }
    return Response.json({ data: { login: pair() } })
  }
  const options = { directory, fetch: fakeFetch, now: () => time }
  let handler = await createSessionHandler(options)
  const server = http.createServer((req, res) => handler(req, res))
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  const base = `http://127.0.0.1:${server.address().port}`
  t.after(async () => {
    await new Promise((resolve) => server.close(resolve))
    // Only the fresh, explicitly created fixture directory is removed.
    assert.equal(path.dirname(directory), os.tmpdir())
    await fs.rm(directory, { recursive: true, force: true })
  })
  let cookie = ''
  const request = async (route = '', options = {}) => {
    const response = await fetch(base + '/api/session' + route, {
      ...options,
      headers: {
        'X-Agri-Session': '1',
        'Content-Type': 'application/json',
        Origin: base,
        Cookie: cookie,
        ...options.headers,
      },
    })
    if (response.headers.get('set-cookie'))
      cookie = response.headers.get('set-cookie').split(';')[0]
    return response
  }
  const login = () =>
    request('/login', {
      method: 'POST',
      body: JSON.stringify({ identifier: 'test', secret: 'only-test' }),
    })
  return {
    directory,
    request,
    login,
    advance: (n) => {
      time += n
    },
    offline: (v) => {
      offline = v
    },
    revoke: () => {
      active = false
    },
    rotations: () => rotations,
    logouts: () => logouts,
    restart: async () => {
      handler = await createSessionHandler(options)
    },
    retry: () => handler.retryPendingLogouts(),
    cookie: () => cookie,
  }
}

test('persistent HttpOnly cookie; encrypted storage; expired access restored exactly once across 16 requests and restart', async (t) => {
  const f = await fixture(t)
  const response = await f.login()
  assert.equal(response.status, 200)
  assert.match(response.headers.get('set-cookie'), /HttpOnly; SameSite=Strict; Max-Age=34560000/)
  const data = await response.json()
  assert.equal(data.refreshToken, undefined)
  assert.equal(data.entityId, 'user-1')
  for (const file of (await fs.readdir(f.directory)).filter((s) => s.endsWith('.json'))) {
    const saved = await fs.readFile(path.join(f.directory, file), 'utf8')
    assert.ok(!saved.includes('atom_rt_secret') && !saved.includes('access-0'))
  }
  f.advance(7200000)
  const restored = await Promise.all(
    Array.from({ length: 16 }, async () => (await f.request()).json()),
  )
  assert.equal(f.rotations(), 1)
  assert.ok(restored.every((s) => s.token === 'access-1' && !s.refreshToken))
  await f.restart()
  assert.equal((await (await f.request()).json()).token, 'access-1')
  assert.equal(f.rotations(), 1)
})

test('offline and temporary errors preserve cookie; recovered network renews; revoked session is rejected', async (t) => {
  const f = await fixture(t)
  await f.login()
  const cookie = f.cookie()
  f.advance(7200000)
  f.offline(true)
  assert.equal((await f.request()).status, 503)
  assert.equal(f.cookie(), cookie)
  f.offline(false)
  assert.equal((await f.request()).status, 200)
  f.advance(61000)
  f.revoke()
  assert.equal((await f.request()).status, 401)
})

test('offline logout is durable across restart, rejects copied cookies and completes revocation after reconnect', async (t) => {
  const f = await fixture(t)
  await f.login()
  const cookie = f.cookie()
  f.advance(7200000)
  f.offline(true)
  assert.equal((await f.request('/logout', { method: 'POST' })).status, 200)
  await f.restart()
  assert.equal((await f.request('', { headers: { Cookie: cookie } })).status, 401)
  f.offline(false)
  await f.retry()
  assert.equal(f.logouts(), 1)
})

test('cross-origin, missing custom header, forged cookies, and malformed login are rejected', async (t) => {
  const f = await fixture(t)
  await f.login()
  assert.equal(
    (
      await f.request('/logout', {
        method: 'POST',
        headers: { Origin: 'https://untrusted.example' },
      })
    ).status,
    403,
  )
  assert.equal((await f.request('', { headers: { 'X-Agri-Session': '' } })).status, 403)
  assert.equal(
    (
      await f.request('', {
        headers: { Cookie: 'agri_session=' + 'a'.repeat(64) + '.' + 'b'.repeat(64) },
      })
    ).status,
    401,
  )
  assert.equal((await f.request('/login', { method: 'POST', body: '{}' })).status, 400)
})
