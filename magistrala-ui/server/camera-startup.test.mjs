import test from 'node:test'
import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import { mkdtemp, mkdir, writeFile, rm, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'
import { startCameraBridge, registerCameraAutostart } from './camera-startup.mjs'

async function fixture(run) {
  const root = await mkdtemp(join(tmpdir(), 'camera-startup-test-'))
  await mkdir(join(root, '.venv/Scripts'), { recursive: true })
  await writeFile(join(root, '.venv/Scripts/python.exe'), '')
  await writeFile(join(root, 'start_camera_service.py'), '')
  await writeFile(join(root, 'playback-state.json'), '{"enabled":false}')
  try { await run(root) } finally {
    assert.equal(dirname(resolve(root)), resolve(tmpdir()))
    assert.ok(basename(root).startsWith('camera-startup-test-'))
    await rm(root, { recursive: true, force: true })
  }
}
const quiet = { info() {}, warn() {} }

test('autostart invokes the existing launcher without a shell, a visible window or playback arguments', () => fixture(async (root) => {
  let launch
  const result = await startCameraBridge({ bridgeRoot: root, platform: 'win32', logger: quiet,
    spawnProcess(exe, args, options) {
      launch = { exe, args, options }
      const child = new EventEmitter()
      queueMicrotask(() => child.emit('exit', 0))
      return child
    } })
  assert.equal(result.status, 'ready')
  assert.equal(launch.exe, join(root, '.venv/Scripts/python.exe'))
  assert.deepEqual(launch.args, [join(root, 'start_camera_service.py')])
  assert.equal(launch.options.windowsHide, true)
  assert.equal(launch.options.shell, undefined)
  assert.equal(await readFile(join(root, 'playback-state.json'), 'utf8'), '{"enabled":false}')
}))

test('a failed bridge startup reports a warning without rejecting platform startup', () => fixture(async (root) => {
  let warning = ''
  const result = await startCameraBridge({ bridgeRoot: root, platform: 'win32', logger: { ...quiet, warn(text) { warning = text } },
    spawnProcess() { const child = new EventEmitter(); queueMicrotask(() => child.emit('exit', 1)); return child } })
  assert.equal(result.status, 'failed')
  assert.match(warning, /自动启动失败/)
}))

test('a hung launcher is bounded and terminated without blocking the platform', () => fixture(async (root) => {
  let killed = false
  const result = await startCameraBridge({ bridgeRoot: root, platform: 'win32', logger: quiet, timeoutMs: 10,
    spawnProcess() { const child = new EventEmitter(); child.kill = () => { killed = true }; return child } })
  assert.equal(result.status, 'failed')
  assert.equal(killed, true)
}))

test('missing local camera configuration does not start a process', async () => {
  let launches = 0
  const result = await startCameraBridge({ bridgeRoot: join(tmpdir(), 'camera-nonexistent-' + Date.now()), platform: 'win32', logger: quiet,
    spawnProcess() { launches++ } })
  assert.equal(result.reason, 'missing_files')
  assert.equal(launches, 0)
})

test('externally managed ports and other operating systems do not launch the Windows bridge', async () => {
  assert.equal((await startCameraBridge({ platform: 'linux' })).reason, 'platform')
  assert.equal((await startCameraBridge({ platform: 'win32', bridgePort: 9999 })).reason, 'external_port')
})

test('registration waits until the platform HTTP server is listening', () => {
  const httpServer = new EventEmitter()
  registerCameraAutostart({ httpServer })
  assert.equal(httpServer.listenerCount('listening'), 1)
  assert.doesNotThrow(() => registerCameraAutostart({}))
})
