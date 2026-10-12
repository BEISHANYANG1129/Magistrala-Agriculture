export interface BrowserSession {
  token: string
  expiresAt?: string
  entityId?: string
  sessionId?: string
}
export function ensureSession(force?: boolean): Promise<BrowserSession | null>
export function signIn(identifier: string, secret: string): Promise<BrowserSession>
export function signOut(): Promise<void>
export function watchSession(): () => void
