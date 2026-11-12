import type { Plugin } from 'vite'
import type { IncomingMessage, ServerResponse } from 'node:http'
export function cameraPlugin(): Plugin
export function createCameraGateway(options?: {
  atomUrl?: string
  bridgePort?: number
  fetch?: typeof globalThis.fetch
}): (req: IncomingMessage, res: ServerResponse, next?: () => void) => Promise<void>
