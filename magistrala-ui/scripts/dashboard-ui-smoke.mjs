// The current edition rejects dashboard creation. This test uses an explicit
// in-memory dashboard fixture; telemetry and login still use the local platform.
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
  workspace = 'cccd00c1-a12c-4d6e-9aa9-d2c52906f2e5'
const dashboard = {
  id: 'ui-fixture-dashboard',
  name: 'UI fixture',
  metadata: { preserve: 'existing-metadata', widget: {} },
  layout: {},
}
const browser = await chromium.launch({ headless: true, channel: 'msedge' }),
  page = await browser.newPage({ viewport: { width: 1440, height: 1000 } }),
  checks = [],
  errors = []
let saved
page.on('pageerror', (e) => errors.push(e.message))
await page.route(new RegExp(`/api/platform/ui/${workspace}/dashboards`), async (route) => {
  const request = route.request()
  if (request.method() === 'PATCH') {
    saved = request.postDataJSON()
    await route.fulfill({ json: saved })
    return
  }
  await route.fulfill({
    json: request.url().includes('/ui-fixture-dashboard')
      ? dashboard
      : { dashboards: [dashboard], total: 1, limit: 20, offset: 0 },
  })
})
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(env.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page.getByLabel('界面语言').selectOption('en')
  await page.goto(`${base}/domain/${workspace}/dashboards`)
  await page.getByRole('button', { name: 'Details', exact: true }).click()
  await page.getByRole('button', { name: '+ Add widget', exact: true }).click()
  const modal = page.locator('.modal-card')
  await modal.getByLabel('Title', { exact: true }).fill('Soil moisture')
  await modal.locator('select').nth(1).selectOption('3e692d54-5ccc-4a83-9cfd-ac0080263327')
  await modal.getByRole('button', { name: 'Apply to canvas', exact: true }).click()
  await page.getByRole('heading', { name: 'Soil moisture', exact: true }).waitFor()
  await page.locator('canvas').waitFor()
  await page.getByRole('button', { name: 'Save dashboard', exact: true }).click()
  await page.getByText('Dashboard saved.', { exact: true }).waitFor()
  checks.push({
    name: 'dashboard widget editor and chart rendering',
    ok:
      !!saved &&
      Object.keys(saved.metadata.widget).length === 1 &&
      saved.metadata.preserve === 'existing-metadata',
    backend: 'mocked; current edition blocks creation',
  })
  await page.screenshot({
    path: 'D:/personal/magistrala/output/agri-ui-upgrade/dashboard-fixture-en.png',
    fullPage: true,
  })
  await page.goto(`${base}/domain/${workspace}/field-map`)
  await page.locator('.toolbar select').first().selectOption('3e692d54-5ccc-4a83-9cfd-ac0080263327')
  await page.locator('svg.map-canvas').waitFor()
  checks.push({
    name: 'native existing field map loads',
    ok: (await page.locator('.error').count()) === 0,
  })
} catch (e) {
  checks.push({ name: 'dashboard/map', ok: false, error: e.message })
  process.exitCode = 1
  console.error(e.message, { saved: !!saved, errors: await page.locator('.error').allTextContents() })
  await page.screenshot({path:'D:/personal/magistrala/output/agri-ui-upgrade/dashboard-failure.png',fullPage:true})
} finally {
  await fs.writeFile(
    'D:/personal/magistrala/output/agri-ui-upgrade/dashboard-ui-smoke.json',
    JSON.stringify({ checks, pageErrors: errors }, null, 2),
  )
  console.log(JSON.stringify({ checks, pageErrors: errors }, null, 2))
  await browser.close()
}
