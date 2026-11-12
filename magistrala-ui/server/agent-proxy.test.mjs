import test from 'node:test'
import assert from 'node:assert/strict'
import http from 'node:http'
import { createServer } from 'vite'
import { createAgentProxy } from './agent-proxy.mjs'

test('Vite agent proxy supports LAN access while retaining origin and authorization checks', async (t) => {
  const requests = []
  const upstream = http.createServer(async (req, res) => {
    const body = []
    for await (const chunk of req) body.push(chunk)
    requests.push({ url: req.url, headers: req.headers, body: Buffer.concat(body).toString() })
    res.setHeader('Content-Type', 'application/json')
    const host = new URL(`http://${req.headers.host}`).hostname
    if (!['127.0.0.1', 'localhost', '[::1]'].includes(host)) {
      res.writeHead(403).end(JSON.stringify({ error: 'loopback host required' }))
    } else if (req.headers.origin && req.headers.origin !== `http://${req.headers.host}`) {
      res.writeHead(403).end(JSON.stringify({ error: 'cross-origin requests are disabled' }))
    } else if (req.url !== '/health' && !req.headers.authorization) {
      res.writeHead(401).end(JSON.stringify({ error: 'login required' }))
    } else {
      res.end(JSON.stringify({ status: 'ok' }))
    }
  })
  await new Promise((resolve) => upstream.listen(0, '127.0.0.1', resolve))
  const target = `http://127.0.0.1:${upstream.address().port}`
  let vite, frontend
  try {
    vite = await createServer({
      configFile: false,
      cacheDir: 'node_modules/.vite-agent-proxy-tests',
      optimizeDeps: { noDiscovery: true, include: [] },
      logLevel: 'silent',
      appType: 'custom',
      server: {
        middlewareMode: true,
        watch: null,
        proxy: { '/api/agent': createAgentProxy(target) },
      },
    })
    frontend = http.createServer(vite.middlewares)
    await new Promise((resolve) => frontend.listen(0, '127.0.0.1', resolve))
    const request = (host, { path = '/health', origin, method = 'GET', body, headers = {} } = {}) =>
      new Promise((resolve, reject) => {
        const req = http.request(
          {
            hostname: '127.0.0.1',
            port: frontend.address().port,
            path: '/api/agent' + path,
            method,
            headers: {
              Host: host,
              ...(origin === undefined ? {} : { Origin: origin }),
              ...headers,
            },
          },
          async (res) => {
            const chunks = []
            for await (const chunk of res) chunks.push(chunk)
            resolve({ status: res.statusCode, data: JSON.parse(Buffer.concat(chunks).toString()) })
          },
        )
        req.on('error', reject)
        req.end(body)
      })

    await t.test(
      'local and LAN health requests pass with or without a same-origin header',
      async () => {
        for (const host of [
          '127.0.0.1:5173',
          'localhost:5173',
          '172.25.76.186:5173',
          '[::1]:5173',
        ]) {
          for (const origin of [undefined, `http://${host}`]) {
            assert.equal((await request(host, { origin })).status, 200)
            assert.equal(requests.at(-1).headers.host, new URL(target).host)
            assert.equal(requests.at(-1).headers.origin, origin === undefined ? undefined : target)
          }
        }
      },
    )

    await t.test('LAN POST retains JSON, token, workspace, path and query', async () => {
      const body = JSON.stringify({ question: '当前有哪些观测指标？' })
      assert.equal(
        (
          await request('172.25.76.186:5173', {
            path: '/v2/workbench/query?limit=2',
            origin: 'http://172.25.76.186:5173',
            method: 'POST',
            body,
            headers: {
              'Content-Type': 'application/json',
              Authorization: 'Bearer test-token',
              'X-Workspace-ID': 'test-workspace',
            },
          })
        ).status,
        200,
      )
      const forwarded = requests.at(-1)
      assert.equal(forwarded.url, '/v2/workbench/query?limit=2')
      assert.equal(forwarded.body, body)
      assert.equal(forwarded.headers.authorization, 'Bearer test-token')
      assert.equal(forwarded.headers['x-workspace-id'], 'test-workspace')
      assert.equal(forwarded.headers['content-type'], 'application/json')
    })

    await t.test('foreign, null, malformed and mismatched origins remain forbidden', async () => {
      for (const origin of [
        'https://evil.example',
        target,
        'null',
        'not-an-origin',
        'http://localhost:5173',
        'https://172.25.76.186:5173',
        'http://172.25.76.186:5174',
      ]) {
        const result = await request('172.25.76.186:5173', {
          origin,
          method: 'POST',
          headers: { Authorization: 'Bearer test-token' },
          body: '{}',
        })
        assert.equal(result.status, 403, origin)
        assert.equal(result.data.error, 'cross-origin requests are disabled')
      }
    })

    await t.test('LAN requests without a token still require login', async () => {
      const result = await request('172.25.76.186:5173', {
        path: '/v2/workbench/status',
        origin: 'http://172.25.76.186:5173',
      })
      assert.equal(result.status, 401)
      assert.equal(result.data.error, 'login required')
    })
  } finally {
    if (frontend) await new Promise((resolve) => frontend.close(resolve))
    if (vite) await vite.close()
    await new Promise((resolve) => upstream.close(resolve))
  }
})
