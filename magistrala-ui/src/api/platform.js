import contracts from './contracts.json'
import { tr } from '../i18n'
import { ensureSession } from './session'

export class ApiError extends Error {
  constructor(message, status = 0) {
    super(message)
    this.status = status
  }
}
export async function request(
  path,
  {
    method = 'GET',
    body,
    headers = {},
    auth = true,
    signal,
    timeout = 25000,
    download = false,
    sessionRetry = false,
  } = {},
) {
  const controller = new AbortController()
  const abort = () => controller.abort()
  signal?.addEventListener('abort', abort, { once: true })
  const timer = setTimeout(abort, timeout)
  try {
    if (auth && !headers.Authorization && localStorage.getItem('agri-remembered'))
      await ensureSession(sessionRetry)
    const token = localStorage.getItem('token')
    const h = { ...headers }
    if (auth && token && !h.Authorization) h.Authorization = `Bearer ${token}`
    if (body !== undefined && !(body instanceof FormData) && !h['Content-Type'])
      h['Content-Type'] = 'application/json'
    const response = await fetch(path, {
      method,
      headers: h,
      body:
        body === undefined
          ? undefined
          : body instanceof FormData || typeof body === 'string'
            ? body
            : JSON.stringify(body),
      signal: controller.signal,
      cache: 'no-store',
    })
    if (download && response.ok) {
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download =
        response.headers.get('Content-Disposition')?.match(/filename="?([^";]+)/)?.[1] ||
        (blob.type.includes('pdf') ? 'report.pdf' : 'report.csv')
      a.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
      return { downloaded: true }
    }
    const raw = await response.text()
    let data
    try {
      data = raw ? JSON.parse(raw) : null
    } catch {
      data = raw
    }
    if (!response.ok || data?.errors?.length) {
      const message =
        data?.errors?.[0]?.message ||
        data?.error ||
        data?.message ||
        (typeof data === 'string' ? data : response.statusText)
      if (
        (response.status === 401 ||
          /ExpiredSignature|invalid token|token expired/i.test(String(message))) &&
        auth &&
        !headers.Authorization
      ) {
        if (!sessionRetry && localStorage.getItem('agri-remembered') && (await ensureSession(true)))
          return request(path, {
            method,
            body,
            headers,
            auth,
            signal,
            timeout,
            download,
            sessionRetry: true,
          })
        // Only the session proxy can confirm a remembered session was revoked.
        // A single resource's 401 must not discard an otherwise valid login.
        if (!localStorage.getItem('agri-remembered')) {
          localStorage.removeItem('token')
          window.dispatchEvent(new Event('agri-session-expired'))
        }
      }
      throw new ApiError(
        typeof message === 'string' ? message : JSON.stringify(message),
        response.status,
      )
    }
    return data
  } catch (e) {
    if (e.name === 'AbortError')
      throw new ApiError(
        tr(
          '请求已取消或超时，请检查服务状态后重试。',
          'Request cancelled or timed out. Check the service before retrying.',
        ),
      )
    if (e instanceof TypeError)
      throw new ApiError(
        tr(
          '无法连接服务，请检查后端是否启动。',
          'Cannot connect. Check that the backend is running.',
        ),
      )
    throw e
  } finally {
    clearTimeout(timer)
    signal?.removeEventListener('abort', abort)
  }
}
export async function gql(query, variables = {}, options = {}) {
  return (await request('/graphql', { method: 'POST', body: { query, variables }, ...options }))
    .data
}
export const bare = (type) => type.replace(/[\[\]!]/g, '')
export function selection(type, depth = 0) {
  const t = contracts.types[bare(type)]
  if (!t || t.kind !== 'type') return ''
  const fields = t.fields
    .filter((f) => !f.args.length)
    .flatMap((f) => {
      const nested = contracts.types[bare(f.type)]
      if (!nested || nested.kind === 'enum') return [f.name]
      if (depth < 2) return [`${f.name} ${selection(f.type, depth + 1)}`]
      return []
    })
  return `{ ${fields.join(' ') || '__typename'} }`
}
export function operation(name) {
  for (const kind of ['query', 'mutation']) {
    const field = contracts.types[kind === 'query' ? 'QueryRoot' : 'MutationRoot'].fields.find(
      (x) => x.name === name,
    )
    if (field) return { ...field, kind }
  }
  throw new Error(`Unknown operation: ${name}`)
}
export async function runOperation(name, values = {}, options = {}) {
  const op = operation(name)
  const args = op.args.filter((a) => values[a.name] !== undefined)
  const vars = Object.fromEntries(args.map((a) => [a.name, values[a.name]]))
  const decl = args.length ? `(${args.map((a) => `$${a.name}: ${a.type}`).join(',')})` : ''
  const call = args.length ? `(${args.map((a) => `${a.name}: $${a.name}`).join(',')})` : ''
  const data = await gql(`${op.kind}${decl}{${name}${call}${selection(op.type)}}`, vars, options)
  return data[name]
}
export function inputSchema(type) {
  const required = type.endsWith('!')
  const inner = type.replace(/!$/, '')
  if (inner.startsWith('['))
    return { type: 'array', items: inputSchema(inner.slice(1, -1)), required }
  const t = contracts.types[inner]
  if (t?.kind === 'enum') return { type: 'string', enum: t.values, required }
  if (t?.kind === 'input')
    return {
      type: 'object',
      properties: Object.fromEntries(t.fields.map((f) => [f.name, inputSchema(f.type)])),
      required,
    }
  return {
    type:
      inner === 'Boolean'
        ? 'boolean'
        : ['Int', 'Float'].includes(inner)
          ? 'number'
          : inner === 'JSON'
            ? 'json'
            : 'string',
    required,
  }
}
export function operationSchema(name) {
  return {
    type: 'object',
    properties: Object.fromEntries(operation(name).args.map((a) => [a.name, inputSchema(a.type)])),
  }
}
export function errorText(error) {
  const m = error?.message || String(error)
  if (m.includes('rule limit'))
    return tr(
      '当前平台版本的规则数量已达上限。可以编辑已有规则；创建更多规则需要平台版本支持。',
      'This edition has reached its rule limit. Edit an existing rule or use an edition supporting more rules.',
    )
  if (m.includes('limit for this edition'))
    return (
      tr(
        '当前平台版本的功能额度已达上限，后端拒绝创建。已有记录仍可查看；增加额度需要平台版本支持。',
        'The backend rejected creation because this edition has reached its feature limit. Existing records remain available; additional capacity requires edition support.',
      ) +
      ' ' +
      m
    )
  if (error?.status === 403)
    return (
      tr(
        '当前账号没有执行此操作的权限。',
        'Your account does not have permission to perform this operation.',
      ) +
      ' ' +
      m
    )
  if (error?.status === 401)
    return tr(
      '认证失效，请重新登录或核对设备密钥。',
      'Authentication failed. Sign in again or check the device key.',
    )
  return m
}
export function items(data) {
  if (Array.isArray(data)) return data
  return data?.items || []
}
export async function listAll(name, values = {}) {
  const out = []
  for (let offset = 0; offset < 10000; offset += 100) {
    const d = await runOperation(name, { ...values, offset, limit: 100 })
    const page = items(d)
    out.push(...page)
    if (!page.length || out.length >= (d.total ?? out.length)) return out
  }
  throw new ApiError(
    tr(
      '记录超过选择器上限，请使用筛选列表。',
      'Too many records for the selector; use a filtered list.',
    ),
  )
}
export { contracts }
