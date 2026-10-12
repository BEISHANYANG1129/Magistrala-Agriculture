import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
import path from 'node:path'
// Read existing local credentials only in memory. Never include them in reports.
const envFile = process.env.AGRI_TEST_ENV || 'D:/personal/magistrala/magistrala/docker/.env'
const settings = Object.fromEntries(
  (await fs.readFile(envFile, 'utf8'))
    .split(/\r?\n/)
    .filter((l) => l.includes('=') && !l.trim().startsWith('#'))
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
const base = process.env.AGRI_UI_TEST_URL || 'http://127.0.0.1:5186'
const output = 'D:/personal/magistrala/output/agri-ui-upgrade'
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
const errors = []
page.on('pageerror', (e) => errors.push(e.message))
const checks = []
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(settings.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(settings.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page
    .locator('.workspace-card')
    .filter({ has: page.getByRole('heading', { name: 'test', exact: true }) })
    .click()
  await page.getByRole('heading', { name: '从数据到农田决策' }).waitFor()
  const workspace = page.url().match(/domain\/([^/]+)/)[1]
  await page.screenshot({ path: path.join(output, 'overview-zh.png'), fullPage: true })
  checks.push({ name: 'login and workspace', ok: true })
  await page.getByLabel('界面语言').selectOption('en')
  await page.getByRole('heading', { name: 'From telemetry to field decisions' }).waitFor()
  await page.reload()
  await page.getByRole('heading', { name: 'From telemetry to field decisions' }).waitFor()
  checks.push({ name: 'language switch and persistence', ok: true })
  for (const section of [
    'devices',
    'channels',
    'groups',
    'gateways',
    'device-types',
    'users',
    'rules',
    'alarms',
    'reports',
    'bootstraps',
    'dashboards',
    'custom-nodes',
    'role-management',
    'invitations',
    'account',
    'platform',
  ]) {
    await page.goto(`${base}/domain/${workspace}/${section}`)
    await page.waitForFunction(() => !document.body.innerText.includes('Loading platform data…'))
    await page.locator('table').first().waitFor()
    const messages = await page.locator('[role="alert"]').allTextContents()
    checks.push({ name: section, ok: messages.length === 0, errors: messages })
  }
  await page.goto(`${base}/domain/${workspace}/devices`)
  await page.getByRole('button', { name: '+ Create', exact: true }).click()
  await page.locator('.modal-card input').first().waitFor()
  checks.push({
    name: 'native device form',
    ok: (await page.locator('.modal-card .field').count()) > 4,
  })
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.goto(
    `${base}/domain/${workspace}/messages?channel=3e692d54-5ccc-4a83-9cfd-ac0080263327`,
  )
  await page.waitForResponse((r) => r.url().includes('/messages?') && r.status() === 200)
  checks.push({ name: 'reader messages', ok: (await page.locator('[role="alert"]').count()) === 0 })
  await page.goto(`${base}/domain/${workspace}/agent`)
  await page.getByRole('button', { name: 'New region', exact: false }).waitFor()
  await page.waitForFunction(
    () =>
      document.body.innerText.includes('Field regions') ||
      document.body.innerText.includes('Connect the regional analysis service'),
  )
  checks.push({
    name: 'agent connection',
    ok: (await page.getByRole('heading', { name: 'Field regions', exact: true }).count()) === 1,
    errors: await page.locator('[role="alert"]').allTextContents(),
  })
  await page.screenshot({ path: path.join(output, 'agent-en.png'), fullPage: true })
  const report = { checks, pageErrors: errors }
  await fs.writeFile(path.join(output, 'ui-smoke.json'), JSON.stringify(report, null, 2))
  console.log(JSON.stringify(report, null, 2))
  if (checks.some((c) => !c.ok) || errors.length) process.exitCode = 1
} finally {
  await browser.close()
}
