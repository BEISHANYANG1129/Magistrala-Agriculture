import { beforeEach, afterEach, describe, it, expect, vi } from 'vitest'

beforeEach(() => {
  vi.resetModules()
  localStorage.clear()
  sessionStorage.clear()
  vi.stubGlobal('fetch', vi.fn())
})
afterEach(() => vi.unstubAllGlobals())
const session = { token: 'access-only', entityId: 'user', sessionId: 'session' }

describe('remembered browser session', () => {
  it('coalesces concurrent checks and stores only the short-lived access token', async () => {
    const { ensureSession } = await import('../api/session')
    fetch.mockResolvedValue(Response.json(session))
    await Promise.all(Array.from({ length: 10 }, () => ensureSession(true)))
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch.mock.calls[0][1].credentials).toBe('same-origin')
    expect(localStorage.getItem('token')).toBe('access-only')
    expect(localStorage.getItem('refreshToken')).toBeNull()
  })
  it('preserves session on temporary failure and clears it only for confirmed revocation', async () => {
    const { ensureSession } = await import('../api/session')
    localStorage.setItem('token', 'existing')
    fetch.mockResolvedValueOnce(Response.json({ message: 'offline' }, { status: 503 }))
    await expect(ensureSession(true)).rejects.toMatchObject({ status: 503 })
    expect(localStorage.getItem('token')).toBe('existing')
    fetch.mockResolvedValueOnce(Response.json({ message: 'revoked' }, { status: 401 }))
    expect(await ensureSession(true)).toBeNull()
    expect(localStorage.getItem('token')).toBeNull()
  })
  it('does not repopulate a session when a pending check finishes during logout', async () => {
    const { ensureSession, signOut } = await import('../api/session')
    let finish
    fetch.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve
        }),
    )
    const check = ensureSession(true)
    fetch.mockResolvedValueOnce(Response.json({ signedOut: true }))
    const logout = signOut()
    finish(Response.json(session))
    await Promise.all([check, logout])
    expect(localStorage.getItem('token')).toBeNull()
  })
})
