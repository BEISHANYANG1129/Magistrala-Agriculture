import { runOperation } from './platform'
import { resourceConfig, readResource, setResourceStatus } from './resourceWorkflow'

// CSV cells can contain commas, escaped quotes, CRLF and embedded newlines.
export function parseCsv(text) {
  const rows = []
  let row = [],
    cell = '',
    quoted = false,
    closed = false
  text = text.replace(/^\uFEFF/, '')
  for (let i = 0; i < text.length; i++) {
    const c = text[i]
    if (quoted) {
      if (c === '"') {
        if (text[i + 1] === '"') {
          cell += '"'
          i++
        } else {
          quoted = false
          closed = true
        }
      } else cell += c
      continue
    }
    if (c === '"') {
      if (cell || closed) throw new Error('Invalid CSV quote')
      quoted = true
    } else if (c === ',') {
      row.push(cell)
      cell = ''
      closed = false
    } else if (c === '\n' || c === '\r') {
      if (c === '\r' && text[i + 1] === '\n') i++
      row.push(cell)
      if (row.some((v) => v.trim())) rows.push(row)
      row = []
      cell = ''
      closed = false
    } else {
      if (closed && !/\s/.test(c)) throw new Error('Unexpected text after CSV quote')
      if (!closed) cell += c
    }
  }
  if (quoted) throw new Error('Unclosed CSV quote')
  row.push(cell)
  if (row.some((v) => v.trim())) rows.push(row)
  return rows
}
export function csvJobs(text, section) {
  const [header, ...rows] = parseCsv(text)
  if (!header || !rows.length) throw new Error('CSV must contain a header and data rows')
  const keys = header.map((v) => v.trim().toLowerCase().replace(/ /g, '_'))
  const allowed =
    section === 'channels'
      ? ['name', 'route', 'tags', 'metadata']
      : ['name', 'key', 'serial', 'tags', 'metadata']
  if (
    !keys.includes('name') ||
    new Set(keys).size !== keys.length ||
    keys.some((k) => !allowed.includes(k))
  )
    throw new Error('CSV headers: ' + allowed.join(', '))
  if (rows.length > 1000) throw new Error('Maximum 1000 rows per import')
  return rows.map((cells, i) => {
    const values = Object.fromEntries(keys.map((k, j) => [k, cells[j] ?? '']))
    let error = ''
    try {
      if (cells.length !== keys.length) throw new Error('Column count mismatch')
      if (!values.name.trim()) throw new Error('Name is required')
      values.name = values.name.trim()
      values.metadata = values.metadata ? JSON.parse(values.metadata) : {}
      if (!values.metadata || Array.isArray(values.metadata) || typeof values.metadata !== 'object')
        throw new Error('Metadata must be an object')
      values.tags = values.tags
        ? values.tags.trim().startsWith('[')
          ? JSON.parse(values.tags)
          : values.tags
              .split(';')
              .map((v) => v.trim())
              .filter(Boolean)
        : []
      if (!Array.isArray(values.tags) || values.tags.some((v) => typeof v !== 'string'))
        throw new Error('Tags must be strings')
      if (values.route && !/^[A-Za-z0-9_-]+$/.test(values.route)) throw new Error('Invalid route')
    } catch (e) {
      error = e.message
    }
    return {
      row: i + 2,
      values,
      status: error ? 'invalid' : 'pending',
      error,
      created: null,
      keyDone: false,
      groupDone: false,
    }
  })
}
export async function importJob(job, section, workspace, parentGroup) {
  const c = resourceConfig[section],
    v = job.values
  if (!job.created) {
    const input = {
      tenantId: workspace,
      kind: c.kind,
      name: v.name,
      attributes: { tags: v.tags, metadata: v.metadata },
    }
    if (section === 'channels') {
      input.attributes.status = 'active'
      if (v.route) {
        input.alias = v.route
        input.attributes.route = v.route
      }
    } else {
      input.attributes.is_gateway = section === 'gateways'
      input.attributes.profile = 'client'
      if (v.serial) input.externalId = v.serial
    }
    job.created = await runOperation('create' + c.type, { input })
  }
  if (c.type === 'Entity' && !job.keyDone) {
    await runOperation('createSharedKey', {
      entityId: job.created.id,
      input: v.key ? { key: v.key } : {},
    })
    job.keyDone = true
    delete v.key
  }
  if (parentGroup && !job.groupDone) {
    await runOperation(`add${c.type}ToObjectGroup`, {
      [c.type === 'Entity' ? 'entityId' : 'resourceId']: job.created.id,
      objectGroupId: parentGroup,
    })
    job.groupDone = true
  }
  return job.created
}
export async function batchResource(action, section, workspace, record) {
  const latest = await readResource(section, record.id, workspace)
  if (latest.managedBy === 'config') throw new Error('Managed by platform configuration')
  if (action === 'enable' || action === 'disable')
    return setResourceStatus(section, latest, action === 'enable', workspace)
  if (!['delete', 'restore'].includes(action)) throw new Error('Unsupported batch action')
  return runOperation(action + resourceConfig[section].type, { id: record.id })
}
