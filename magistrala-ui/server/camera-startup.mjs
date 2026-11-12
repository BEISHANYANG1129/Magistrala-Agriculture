import { spawn } from 'node:child_process'
import { access } from 'node:fs/promises'
import { resolve } from 'node:path'

export async function startCameraBridge({
  uiRoot,
  bridgeRoot = process.env.AGRI_CAMERA_BRIDGE_ROOT,
  bridgePort = Number(process.env.AGRI_CAMERA_BRIDGE_PORT || 9030),
  platform = process.platform,
  logger = console,
  spawnProcess = spawn,
  timeoutMs = 20000,
} = {}) {
  if (platform !== 'win32') return { status: 'skipped', reason: 'platform' }
  if (bridgePort !== 9030) return { status: 'skipped', reason: 'external_port' }
  const directory = bridgeRoot ? resolve(bridgeRoot) : resolve(uiRoot, '../../../output/camera-integration')
  const python = resolve(directory, '.venv/Scripts/python.exe')
  const launcher = resolve(directory, 'start_camera_service.py')
  try {
    await Promise.all([access(python), access(launcher)])
  } catch {
    logger.warn('[camera] 本机视频桥接服务未配置，平台其他功能可正常使用。')
    return { status: 'skipped', reason: 'missing_files' }
  }
  return new Promise((done) => {
    let child, timer, settled = false
    const finish = (status) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      if (status === 'ready') logger.info('[camera] 视频桥接服务已就绪，沿用已保存的播放开关状态。')
      else logger.warn('[camera] 视频桥接服务自动启动失败，请检查本机桥接服务配置和日志。')
      done({ status })
    }
    try {
      // The existing launcher checks health before starting a hidden, independent
      // bridge process. It never changes the saved playback setting.
      child = spawnProcess(python, [launcher], { cwd: directory, windowsHide: true, stdio: 'ignore' })
      child.once('error', () => finish('failed'))
      child.once('exit', (code) => finish(code === 0 ? 'ready' : 'failed'))
      timer = setTimeout(() => { child.kill(); finish('failed') }, timeoutMs)
    } catch {
      finish('failed')
    }
  })
}

export function registerCameraAutostart(server) {
  if (!server.httpServer) return
  const start = () => {
    void startCameraBridge({ uiRoot: server.config.root, logger: server.config.logger })
      .catch(() => server.config.logger.warn('[camera] 视频桥接服务自动启动失败。'))
  }
  if (server.httpServer.listening) start()
  else server.httpServer.once('listening', start)
}
