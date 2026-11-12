import fs from 'node:fs'
import path from 'node:path'

export function agentOptions(root, env = process.env) {
  const directory = path.join(root, 'llm', 'agentv2')
  const state = env.AGRI_AGENT_STATE_DIR || path.join(directory, 'runtime', 'intelligent-workbench')
  fs.mkdirSync(state, { recursive: true })
  const registry = path.join(state, 'registry.json')
  if (!fs.existsSync(registry)) fs.copyFileSync(path.join(directory, 'examples', 'empty-registry.json'), registry)
  const args = [
    '-listen', env.AGRI_AGENT_LISTEN || '127.0.0.1:9026',
    '-platform-url', env.AGRI_ATOM_URL || 'http://127.0.0.1:8080',
    '-mode', env.AGRI_UI_MODEL ? 'ollama' : 'reader-demo',
    '-reader', env.AGRI_READER_URL || 'http://127.0.0.1:9011',
    '-alarms-url', env.AGRI_ALARMS_URL || 'http://127.0.0.1:8050',
    '-time-unit', 'nanoseconds', '-registry', registry,
    '-state-dir', path.join(state, 'history'),
    '-workbench-model', env.AGRI_WORKBENCH_MODEL || 'qwen3.5:9b',
  ]
  for (const [variable, flag] of [
    ['AGRI_UI_MODEL', '-model'], ['AGRI_OLLAMA_URL', '-ollama-url'],
    ['AGRI_WORKBENCH_DATA', '-workbench-data'],
    ['AGRI_WORKBENCH_PHOTOS', '-workbench-photo-sources'],
    ['AGRI_WORKBENCH_CAMERAS', '-workbench-camera-sources'],
    ['AGRI_CAMERA_GATEWAY_URL', '-camera-gateway-url'],
  ]) if (env[variable]) args.push(flag, env[variable])
  if (env.AGRI_WORKBENCH_PREVIEW === '1') args.push('-workbench-preview')
  return { directory, args }
}
