let pending,
  checkedAt = 0,
  generation = 0

function remember(data) {
  localStorage.setItem('token', data.token)
  localStorage.setItem('agri-remembered', '1')
  sessionStorage.setItem('entityId', data.entityId)
  sessionStorage.setItem('sessionId', data.sessionId)
  checkedAt = Date.now()
  return data
}
function clear() {
  checkedAt = 0
  localStorage.removeItem('token')
  localStorage.removeItem('agri-remembered')
  sessionStorage.removeItem('entityId')
  sessionStorage.removeItem('sessionId')
}
async function call(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    cache: 'no-store',
    credentials: 'same-origin',
    signal: AbortSignal.timeout(20000),
    headers: { 'X-Agri-Session': '1', 'Content-Type': 'application/json' },
  })
  const data = await response.json()
  if (!response.ok)
    throw Object.assign(new Error(data.message || 'Session service unavailable'), {
      status: response.status,
    })
  return data
}
export async function signIn(identifier, secret) {
  generation++
  return remember(
    await call('/api/session/login', {
      method: 'POST',
      body: JSON.stringify({ identifier, secret }),
    }),
  )
}
export async function ensureSession(force = false) {
  if (!force && localStorage.getItem('token') && Date.now() - checkedAt < 55000)
    return { token: localStorage.getItem('token') }
  if (pending) return pending
  const current = generation
  pending = call('/api/session')
    .then((data) => (current === generation ? remember(data) : data))
    .catch((error) => {
      if (error.status === 401 && current === generation) {
        clear()
        window.dispatchEvent(new Event('agri-session-expired'))
        return null
      }
      // Network/5xx never erase the remembered session or the current access token.
      throw error
    })
    .finally(() => {
      pending = null
    })
  return pending
}
export async function signOut() {
  generation++
  await pending?.catch(() => {})
  await call('/api/session/logout', { method: 'POST' })
  clear()
}
export function watchSession() {
  const check = () => {
    if (localStorage.getItem('agri-remembered') && !document.hidden)
      ensureSession(true).catch(() => {})
  }
  const timer = setInterval(check, 60000)
  window.addEventListener('focus', check)
  window.addEventListener('online', check)
  document.addEventListener('visibilitychange', check)
  return () => {
    clearInterval(timer)
    window.removeEventListener('focus', check)
    window.removeEventListener('online', check)
    document.removeEventListener('visibilitychange', check)
  }
}
