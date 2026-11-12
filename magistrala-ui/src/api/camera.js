import { request } from './platform'

export function cameraConfig(record) {
  const attributes = record?.attributes || {}
  const camera = attributes.metadata?.camera || attributes.camera
  return camera?.provider === 'jooan-tutk' && camera.bindingId === 'jooan-g4s4' ? camera : null
}

export function cameraBase(workspace, deviceId) {
  const uuid = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/
  if (!uuid.test(workspace || '') || !uuid.test(deviceId || '')) throw new Error('请选择摄像头设备。')
  return `/api/cameras/${workspace}/${deviceId}/`
}

export function cameraRequest(workspace, deviceId, tail, method = 'GET') {
  return request(cameraBase(workspace, deviceId) + tail, { method })
}

export function cameraPtz(workspace, deviceId, action, command) {
  if (!['move', 'stop'].includes(action)) throw new Error('云台动作无效。')
  return request(cameraBase(workspace, deviceId) + `ptz/${action}`, { method: 'POST', body: command, timeout: 15000 })
}

export function cameraMediaUrl(workspace, deviceId, playlist) {
  if (!/^sessions\/[a-f0-9]{32}\/media\/g-[a-f0-9]{32}\/[012]\/index\.m3u8$/.test(playlist || ''))
    throw new Error('视频播放地址无效。')
  return cameraBase(workspace, deviceId) + playlist
}
