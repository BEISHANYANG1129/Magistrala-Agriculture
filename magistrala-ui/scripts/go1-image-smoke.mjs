import { chromium } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs/promises'
const root = 'D:/personal/magistrala'
const resources = JSON.parse(await fs.readFile(`${root}/output/go1-channel-pilot/resources.json`, 'utf8'))
const env = JSON.parse(execFileSync('docker', ['inspect', 'magistrala-atom'], { encoding: 'utf8' }))[0].Config.Env
const secret = env.find(x => x.startsWith('ADMIN_SECRET=')).slice('ADMIN_SECRET='.length)
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const context = await browser.newContext({ viewport: { width: 1400, height: 950 } })
const errors = []
try {
  const login = await context.request.post('http://127.0.0.1:5173/api/session/login', {
    headers: { Origin: 'http://127.0.0.1:5173', 'X-Agri-Session': '1' },
    data: { identifier: 'admin', secret },
  })
  if (!login.ok()) throw new Error(`Login failed ${login.status()}`)
  const page = await context.newPage()
  page.on('pageerror', e => errors.push(e.message))
  await page.goto(resources.preview_url)
  await page.locator('.message-image-grid img').first().waitFor({ state: 'visible', timeout: 20000 })
  const image = page.locator('.message-image-grid img').first()
  await image.evaluate(async el => { await el.decode() })
  const size = await image.evaluate(el => ({ width: el.naturalWidth, height: el.naturalHeight }))
  if (size.width !== 464 || size.height !== 400) throw new Error('Unexpected image dimensions')
  await page.screenshot({ path: `${root}/output/go1-channel-pilot/platform-image-preview.png`, fullPage: true })
  await page.getByRole('button', { name: '放大图片', exact: true }).first().click()
  await page.getByRole('dialog').waitFor()
  await page.getByRole('dialog').locator('img').evaluate(async el => { await el.decode() })
  await page.getByRole('button', { name: '关闭', exact: true }).click()
  await page.getByRole('button', { name: '表格', exact: true }).click()
  await page.locator('.table-wrap img').first().waitFor({ state: 'visible' })
  await page.getByRole('button', { name: '原始记录', exact: true }).click()
  if (!(await page.locator('pre.panel').innerText()).includes('data_value')) throw new Error('Missing Reader bytes')
  if (errors.length) throw new Error(errors.join('\n'))
  console.log(JSON.stringify({ status: 'PASS', imageSize: size, checks: ['real Reader image', 'enlarge modal', 'table image', 'raw binary record'], url: resources.preview_url }))
} finally {
  await context.request.post('http://127.0.0.1:5173/api/session/logout', { headers: { Origin: 'http://127.0.0.1:5173', 'X-Agri-Session': '1' } }).catch(() => {})
  await browser.close()
}
