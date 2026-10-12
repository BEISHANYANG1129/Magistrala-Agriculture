import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
const env = Object.fromEntries(
  (await fs.readFile('D:/personal/magistrala/magistrala/docker/.env', 'utf8'))
    .split(/\r?\n/)
    .filter((l) => l.includes('=') && !l.startsWith('#'))
    .map((l) => {
      const i = l.indexOf('=')
      return [
        l.slice(0, i),
        l
          .slice(i + 1)
          .trim()
          .replace(/^['"]|['"]$/g, ''),
      ]
    }),
)
const base = process.env.AGRI_UI_TEST_URL || 'http://127.0.0.1:5186',
  workspace = 'f7764dd9-f5e4-4902-8737-cf0a8146e7c5'
const browser = await chromium.launch({ headless: true, channel: 'msedge' }),
  page = await browser.newPage({ viewport: { width: 1400, height: 1000 } })
const created = [],
  checks = [],
  suffix = Date.now(),
  errors = []
page.on('pageerror', (e) => errors.push(e.message))
async function api(name, values) {
  return page.evaluate(
    async ({ name, values }) => {
      const api = await import('/src/api/platform.js')
      return api.runOperation(name, values)
    },
    { name, values },
  )
}
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(env.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page.getByLabel('界面语言').selectOption('en')
  for (const section of ['channels', 'devices']) {
    const name = `agri-ui-check-${section}-${suffix}`
    await page.goto(`${base}/domain/${workspace}/${section}`)
    await page.getByRole('button', { name: '+ Create', exact: true }).click()
    const modal = page.locator('.modal-card')
    await modal.getByLabel(/^Name/).fill(name)
    await modal.getByRole('button', { name: 'Submit', exact: true }).click()
    await modal.locator('.success').waitFor()
    const result = JSON.parse(await modal.locator('.success pre').innerText())
    created.push({ section, id: result.id })
    if (!result.id) throw new Error('Create response missing ID')
    await modal.getByRole('button', { name: 'Close', exact: true }).click()
    const row = page.locator('tr').filter({ hasText: name })
    await row.getByRole('button', { name: 'Edit', exact: true }).click()
    await modal.getByLabel(/^Name/).fill(name + '-edited')
    await modal.getByRole('button', { name: 'Submit', exact: true }).click()
    await modal.locator('.success').waitFor()
    await modal.getByRole('button', { name: 'Close', exact: true }).click()
    const read = await api(section === 'channels' ? 'resource' : 'entity', { id: result.id })
    if (read.name !== name + '-edited') throw new Error('Edit did not persist')
    checks.push({ name: `${section} native create/edit/read`, ok: true })
  }
  const ch = created.find((x) => x.section === 'channels').id,
    dev = created.find((x) => x.section === 'devices').id
  await page.goto(`${base}/domain/${workspace}/channels`)
  await page.getByLabel('Device', { exact: true }).selectOption(dev)
  await page.getByLabel('Channel', { exact: true }).selectOption(ch)
  await page.getByRole('button', { name: 'Connect', exact: true }).click()
  await page.locator('.success').filter({ hasText: 'Connection created.' }).waitFor()
  const policies = await api('directPolicies', {
    tenantId: workspace,
    objectId: ch,
    objectKind: 'resource',
    limit: 100,
  })
  for (const p of policies.items) {
    created.push({ section: 'policy', id: p.id })
    created.push({ section: 'permission', id: p.permissionBlockId })
  }
  checks.push({
    name: 'native device/channel permission connection',
    ok: policies.items.length === 1,
  })
  await page.goto(`${base}/domain/${workspace}/dashboards`)
  await page.getByRole('button', { name: '+ Create', exact: true }).click()
  const modal = page.locator('.modal-card')
  await modal.getByLabel(/^Name/).fill('agri-ui-dashboard-' + suffix)
  await modal.getByRole('button', { name: 'Submit', exact: true }).click()
  await modal.locator('.success').waitFor()
  await modal.getByRole('button', { name: 'Close', exact: true }).click()
  const dashboard = await page.evaluate(
    async ({ workspace, suffix }) => {
      const { request } = await import('/src/api/platform.js')
      const d = await request(
        `/api/platform/ui/${workspace}/dashboards?name=agri-ui-dashboard-${suffix}`,
      )
      return d.dashboards.find((x) => x.name === `agri-ui-dashboard-${suffix}`)
    },
    { workspace, suffix },
  )
  if (!dashboard?.id) throw new Error('Dashboard was not saved')
  created.push({ section: 'dashboard', id: dashboard.id })
  checks.push({ name: 'native dashboard creation', ok: true })
  console.log(JSON.stringify({ checks, pageErrors: errors }, null, 2))
} catch (e) {
  checks.push({ name: 'write flow', ok: false, error: e.message })
  process.exitCode = 1
  console.error(e.message)
  await page.screenshot({
    path: 'D:/personal/magistrala/output/agri-ui-upgrade/write-failure.png',
    fullPage: true,
  })
} finally {
  for (const item of [...created].sort(
    (a, b) =>
      ['policy', 'permission', 'dashboard', 'devices', 'channels'].indexOf(a.section) -
      ['policy', 'permission', 'dashboard', 'devices', 'channels'].indexOf(b.section),
  )) {
    try {
      if (item.section === 'dashboard')
        await page.evaluate(
          async ({ workspace, id }) => {
            const { request } = await import('/src/api/platform.js')
            await request(`/api/platform/ui/${workspace}/dashboards/${id}`, { method: 'DELETE' })
          },
          { workspace, id: item.id },
        )
      else
        await api(
          {
            policy: 'deleteDirectPolicy',
            permission: 'deletePermissionBlock',
            devices: 'deleteEntity',
            channels: 'deleteResource',
          }[item.section],
          { id: item.id },
        )
      item.cleaned = true
    } catch (e) {
      if (item.section === 'permission' && e.message.includes('not found')) {
        item.cleaned = true
        item.note = 'Automatically removed with the direct policy'
      } else {
        item.cleaned = false
        item.error = e.message
        process.exitCode = 1
      }
    }
  }
  await fs.writeFile(
    'D:/personal/magistrala/output/agri-ui-upgrade/ui-write-smoke.json',
    JSON.stringify({ checks, pageErrors: errors, created }, null, 2),
  )
  await browser.close()
}
