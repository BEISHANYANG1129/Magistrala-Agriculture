// Uses a separate seeded-admin test session. Never logs passwords, cookies or tokens.
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import crypto from 'node:crypto'
import http from 'node:http'
import { createSessionHandler } from './session-server.mjs'

const inspected = JSON.parse(
  execFileSync('docker', ['inspect', 'magistrala-atom'], { encoding: 'utf8' }),
)[0]
const env = Object.fromEntries(
  inspected.Config.Env.map((v) => {
    const i = v.indexOf('=')
    return [v.slice(0, i), v.slice(i + 1)]
  }),
)
assert.ok(env.ADMIN_SECRET, 'Configured test login unavailable')
const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'agri-session-live-'))
let rotations = 0
const options = {
  directory,
  fetch: (url, opts) => {
    if (opts.body?.includes('RememberedDeviceRefresh')) rotations++
    return fetch(url, opts)
  },
}
let handler = await createSessionHandler(options)
const server = http.createServer((req, res) => handler(req, res))
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const base = `http://127.0.0.1:${server.address().port}`
let cookie = ''
async function request(route = '', opts = {}) {
  const response = await fetch(base + '/api/session' + route, {
    ...opts,
    headers: {
      'X-Agri-Session': '1',
      'Content-Type': 'application/json',
      Origin: base,
      Cookie: cookie,
    },
  })
  if (response.headers.get('set-cookie')) cookie = response.headers.get('set-cookie').split(';')[0]
  return response
}
try {
  const login = await request('/login', {
    method: 'POST',
    body: JSON.stringify({ identifier: 'admin', secret: env.ADMIN_SECRET }),
  })
  assert.equal(login.status, 200, 'Test login failed')
  const session = await login.json()
  assert.equal(session.refreshToken, undefined)
  assert.match(login.headers.get('set-cookie'), /HttpOnly; SameSite=Strict; Max-Age=34560000/)
  console.log('PASS: real Atom login with persistent HttpOnly cookie and no browser refresh token')
  const id = cookie.split('=')[1].split('.')[0]
  assert.match(id, /^[a-f0-9]{64}$/)
  const secret = await fs.readFile(path.join(directory, 'encryption.key'), 'utf8')
  const file = path.join(directory, id + '.json')
  const key = crypto
    .createHash('sha256')
    .update('magistrala-remembered-device-v1\0' + secret)
    .digest()
  const raw = JSON.parse(await fs.readFile(file, 'utf8'))
  const decipher = crypto.createDecipheriv('aes-256-gcm', key, Buffer.from(raw.iv, 'base64'))
  decipher.setAAD(Buffer.from(id))
  decipher.setAuthTag(Buffer.from(raw.tag, 'base64'))
  const state = JSON.parse(
    Buffer.concat([decipher.update(Buffer.from(raw.data, 'base64')), decipher.final()]).toString(),
  )
  // Expire only this script's own isolated session, never a user's browser session.
  state.accessTokenExpiry = Date.now() - 1000
  state.accessToken = 'expired-test-access'
  const iv = crypto.randomBytes(12),
    cipher = crypto.createCipheriv('aes-256-gcm', key, iv)
  cipher.setAAD(Buffer.from(id))
  const data = Buffer.concat([cipher.update(JSON.stringify(state)), cipher.final()])
  await fs.writeFile(
    file,
    JSON.stringify({
      v: 1,
      iv: iv.toString('base64'),
      tag: cipher.getAuthTag().toString('base64'),
      data: data.toString('base64'),
    }),
  )
  const results = await Promise.all(
    Array.from({ length: 16 }, async () => {
      const response = await request()
      assert.equal(response.status, 200)
      return response.json()
    }),
  )
  assert.equal(rotations, 1)
  assert.ok(results.every((s) => s.sessionId === session.sessionId && s.token && !s.refreshToken))
  console.log('PASS: expired access restored through real refresh rotation, 16 callers rotate once')
  handler = await createSessionHandler(options)
  assert.equal((await request()).status, 200)
  assert.equal(rotations, 1)
  console.log('PASS: new proxy instance restores encrypted persisted session')
  const copiedCookie = cookie
  assert.equal((await request('/logout', { method: 'POST' })).status, 200)
  cookie = copiedCookie
  assert.equal((await request()).status, 401)
  console.log('PASS: real logout revokes the session and rejects the copied cookie')
} finally {
  await request('/logout', { method: 'POST' }).catch(() => {})
  await new Promise((resolve) => server.close(resolve))
  assert.equal(path.dirname(directory), os.tmpdir())
  await fs.rm(directory, { recursive: true, force: true })
}
