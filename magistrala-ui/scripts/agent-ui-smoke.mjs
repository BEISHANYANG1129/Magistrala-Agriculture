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
const base = 'http://127.0.0.1:5186',
  workspace = 'f7764dd9-f5e4-4902-8737-cf0a8146e7c5',
  regionId = 'ui-verification-' + Date.now()
const browser = await chromium.launch({ headless: true, channel: 'msedge' }),
  page = await browser.newPage({ viewport: { width: 1440, height: 1000 } }),
  checks = []
let created = false
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(env.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page.getByLabel('界面语言').selectOption('en')
  await page.goto(`${base}/domain/${workspace}/agent`)
  await page.getByRole('button', { name: '+ New region', exact: true }).click()
  const modal = page.locator('.modal-card')
  await modal.getByLabel('Name', { exact: true }).fill('UI verification')
  await modal.getByLabel('Region ID', { exact: true }).fill(regionId)
  await modal.locator('select').nth(0).selectOption('04ab7c3e-36bf-401d-8a66-740bf63f945d')
  await modal.locator('select').nth(1).selectOption('76b77410-50eb-452d-94da-29ef799e26c4')
  await modal.getByRole('button', { name: 'Save', exact: true }).click()
  await page.locator('.modal-card').waitFor({ state: 'hidden' })
  created = true
  checks.push({ name: 'scoped native region create', ok: true })
  await page.getByRole('button', { name: 'Analyze this region', exact: true }).click()
  await page.getByRole('heading', { name: 'Analysis result', exact: true }).waitFor()
  await page.getByText('Missing or stale data', { exact: true }).first().waitFor()
  checks.push({ name: 'real Reader missing data stops analysis without GPU', ok: true })
  await page.reload()
  await page.getByRole('heading', { name: 'Field regions', exact: true }).waitFor()
  await page.getByRole('button', { name: /UI verification/ }).click()
  await page.getByRole('heading', { name: 'Region analysis history', exact: true }).waitFor()
  await page.getByRole('button', { name: /Missing or stale data/ }).waitFor()
  checks.push({ name: 'history restores in integrated UI', ok: true })
  await page.screenshot({
    path: 'D:/personal/magistrala/output/agri-ui-upgrade/agent-workflow-en.png',
    fullPage: true,
  })
} catch (e) {
  checks.push({ name: 'agent workflow', ok: false, error: e.message })
  process.exitCode = 1
  console.error(e.message)
} finally {
  if (created) {
    try {
      await page.evaluate(
        async ({ workspace, regionId }) => {
          const { request } = await import('/src/api/platform.js')
          const headers = { 'X-Workspace-ID': workspace }
          const r = await request('/api/agent/v1/regions', { headers })
          await request('/api/agent/v1/regions', {
            method: 'POST',
            headers,
            body: {
              expected_revision: r.revision,
              registry: { version: r.version, regions: r.regions.filter((x) => x.id !== regionId) },
            },
          })
        },
        { workspace, regionId },
      )
      checks.push({ name: 'test region removed', ok: true })
    } catch (e) {
      checks.push({ name: 'cleanup', ok: false, error: e.message })
      process.exitCode = 1
    }
  }
  await fs.writeFile(
    'D:/personal/magistrala/output/agri-ui-upgrade/agent-ui-smoke.json',
    JSON.stringify({ checks }, null, 2),
  )
  console.log(JSON.stringify({ checks }, null, 2))
  await browser.close()
}
