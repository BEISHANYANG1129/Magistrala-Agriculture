// Authenticated local end-to-end verification. Credentials remain in memory.
// Physical checks use an explicitly bounded, once-confirmed plan.
import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
const root = 'D:/personal/magistrala'
const output = root + '/output/multi-agent-rebuild/implementation'
const workspace = 'a14a9eeb-da40-4f82-babe-aa3a5204ecbe'
const base = 'http://127.0.0.1:5173'
const mode = process.argv[2] || 'capture'
const env = Object.fromEntries(
  (await fs.readFile(root + '/magistrala/docker/.env', 'utf8'))
    .split(/\r?\n/)
    .filter((line) => line.includes('=') && !line.trim().startsWith('#'))
    .map((line) => {
      const at = line.indexOf('=')
      return [
        line.slice(0, at),
        line
          .slice(at + 1)
          .trim()
          .replace(/^['"]|['"]$/g, ''),
      ]
    }),
)
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const page = await browser.newPage({ viewport: { width: 1600, height: 1100 } })
const errors = []
page.on('pageerror', (error) => errors.push(error.message))
async function api(path, body) {
  return page.evaluate(
    async ({ path, body, workspace }) => {
      const { request } = await import('/src/api/platform.js')
      return request('/api/agent/v2/workbench' + path, {
        method: body === undefined ? 'GET' : 'POST',
        body,
        headers: { 'X-Workspace-ID': workspace },
        timeout: 45000,
      })
    },
    { path, body, workspace },
  )
}
async function settle(id) {
  const until = Date.now() + 600000
  let log = 0
  while (Date.now() < until) {
    const detail = await api('/automation/' + id)
    if (!['running', 'camera_running'].includes(detail.task.status)) return detail
    if (Date.now() - log > 30000) {
      console.log('waiting', detail.task.status, detail.task.automation.steps.at(-1)?.role)
      log = Date.now()
    }
    await page.waitForTimeout(1000)
  }
  throw new Error('Task timeout: ' + id)
}
function assert(ok, message) {
  if (!ok) throw new Error(message)
}
let detail
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(env.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page.goto(`${base}/domain/${workspace}/agent`)
  await page.getByText('服务已连接', { exact: true }).waitFor({ timeout: 30000 })
  await page.getByRole('button', { name: '自动化任务', exact: true }).click()
  const goals = {
    capture: '请用乔安 G4S4 摄像头现在截一张画面1的照片，保存即可，不要分析，也不要转动。',
    motion:
      '请让乔安 G4S4 摄像头向左转动100毫秒，截取画面1的新照片，再向右转动100毫秒，然后再截一张画面1的新照片。保存两张截图即可，不需要分析。',
    scene: '请用乔安 G4S4 摄像头现在截取画面1，并简要说明画面里有什么。不要转动摄像头。',
    cancel: '请让乔安 G4S4 摄像头先等待10秒，再截取画面1，保存即可，不需要转动或分析。',
    unsupported: '请把乔安 G4S4 摄像头精确向左转90度，然后截图。',
    other_camera: '请用Go1摄像头现在拍一张新照片保存，不要用乔安摄像头代替。',
    simulation: '请模拟让乔安 G4S4 摄像头向左微调，不要真实转动。',
  }
  assert(goals[mode], 'Unknown test mode')
  await page.getByLabel('任务目标', { exact: true }).fill(goals[mode])
  let created
  const existing = process.argv[3]
  if (existing) {
    assert(/^[a-f0-9]{32}$/.test(existing), 'Invalid task ID')
    const saved = await api('/automation/' + existing)
    created = ['planner_failed', 'interrupted', 'needs_input', 'camera_unknown'].includes(
      saved.task.status,
    )
      ? await api('/automation/' + existing + '/resume', {})
      : saved
    await page.reload()
    await page.getByRole('button', { name: '自动化任务', exact: true }).click()
  } else {
    const createdResponse = page.waitForResponse(
      (r) => r.url().endsWith('/automation') && r.request().method() === 'POST',
    )
    await page.getByRole('button', { name: '运行任务', exact: true }).click()
    created = await (await createdResponse).json()
  }
  console.log('created', created.task.id)
  detail = await settle(created.task.id)
  await fs.writeFile(output + `/camera-${mode}-planning.json`, JSON.stringify(detail, null, 2))
  if (['unsupported', 'other_camera', 'simulation'].includes(mode)) {
    assert(
      detail.task.status === 'needs_input',
      'Unsupported request did not pause: ' + detail.task.status,
    )
    assert(
      !detail.task.automation.camera?.started_at &&
        !detail.task.automation.camera?.receipts?.length &&
        !detail.task.automation.camera?.captures?.length,
      'Unsupported task dispatched camera operations',
    )
    assert(detail.task.automation.input_needs?.length > 0, 'Capability gap was not explained')
    await fs.writeFile(
      output + `/camera-${mode}-result.json`,
      JSON.stringify(
        { checked_at: new Date().toISOString(), detail, page_errors: errors },
        null,
        2,
      ),
    )
    console.log('passed', mode, created.task.id, 'no device operations')
  } else {
    assert(
      detail.task.status === 'waiting_camera_confirmation',
      'Planning did not produce a confirmation: ' +
        detail.task.status +
        ' ' +
        JSON.stringify(detail.task.automation.input_needs || detail.task.automation.steps),
    )
    const camera = detail.task.automation.camera
    const snapshot = { action: 'snapshot', direction: '', duration_ms: 0, view: 1, wait_seconds: 0 }
    const expected =
      mode === 'motion'
        ? [
            { action: 'move', direction: 'left', duration_ms: 100, view: 1, wait_seconds: 0 },
            snapshot,
            { action: 'move', direction: 'right', duration_ms: 100, view: 1, wait_seconds: 0 },
            snapshot,
          ]
        : mode === 'cancel'
          ? [{ action: 'wait', direction: '', duration_ms: 0, view: 1, wait_seconds: 10 }, snapshot]
          : [snapshot]
    assert(
      JSON.stringify(camera.request.operations) === JSON.stringify(expected),
      'Plan differed from the bounded test sequence; it was not confirmed',
    )
    assert(
      camera.request.operations.filter((op) => op.action === 'move').length ===
        (mode === 'motion' ? 2 : 0),
      'Unexpected movement',
    )
    if (mode === 'motion')
      assert(
        camera.request.operations
          .filter((op) => op.action === 'move')
          .every((op) => op.duration_ms === 100),
        'Movement duration changed',
      )
    assert(camera.request.analyze === (mode === 'scene'), 'Unexpected analysis request')
    assert(
      camera.receipts.length === 0 && camera.captures.length === 0,
      'Device acted before confirmation',
    )
    const confirm = page.getByRole('button', { name: '确认整条计划并执行', exact: true })
    await confirm.waitFor({ timeout: 10000 })
    assert(!(await confirm.isEnabled()), 'Confirmation checkbox bypassed')
    await page.screenshot({ path: output + `/camera-${mode}-confirmation.png`, fullPage: true })
    await page.locator('.camera-task input[type=checkbox]').check()
    const confirmationResponse = page.waitForResponse(
      (r) => r.url().endsWith('/camera/confirm') && r.request().method() === 'POST',
    )
    await confirm.click()
    const confirmed = await confirmationResponse
    assert(confirmed.ok(), 'Confirmation failed: ' + (await confirmed.text()))
    if (mode === 'cancel') {
      const until = Date.now() + 6000
      let waiting = false
      while (Date.now() < until) {
        detail = await api('/automation/' + created.task.id)
        waiting = detail.task.automation.camera?.receipts?.[0]?.status === 'dispatching'
        if (waiting) break
        await page.waitForTimeout(150)
      }
      assert(waiting, 'Wait step did not start before stop check')
      const stoppedResponse = page.waitForResponse(
        (r) => r.url().endsWith('/camera/stop') && r.request().method() === 'POST',
      )
      await page.getByRole('button', { name: '停止转动并中止任务', exact: true }).click()
      assert((await stoppedResponse).ok(), 'Emergency stop failed')
    }
    detail = await settle(created.task.id)
    await fs.writeFile(
      output + `/camera-${mode}-result.json`,
      JSON.stringify(
        { checked_at: new Date().toISOString(), detail, page_errors: errors },
        null,
        2,
      ),
    )
    assert(
      detail.task.status === (mode === 'cancel' ? 'camera_stopped' : 'completed'),
      'Camera task failed: ' + JSON.stringify(detail.task.automation.steps),
    )
    assert(
      detail.task.automation.camera.captures.length ===
        (mode === 'motion' ? 2 : mode === 'cancel' ? 0 : 1),
      'Unexpected capture count',
    )
    if (mode === 'scene')
      assert(
        detail.reports.length === 1 && detail.reports[0].claims?.length > 0,
        'Scene report missing',
      )
    else assert(detail.reports.length === 0, 'Screenshot-only task ran analysis')
    if (mode !== 'cancel') {
      const saved = detail.task.automation.camera.captures[0]
      const image = await api('/automation/' + created.task.id + '/camera/captures/' + saved.id)
      assert(image.image?.length > 1000, 'Capture download failed')
      await page.getByRole('button', { name: '查看截图', exact: true }).first().click()
      await page.locator('.camera-task img').first().waitFor()
    }
    await page.screenshot({ path: output + `/camera-${mode}-completed.png`, fullPage: true })
    assert(errors.length === 0, 'Browser errors: ' + errors.join('; '))
    console.log(
      'passed',
      mode,
      created.task.id,
      detail.task.automation.camera.captures.length,
      detail.reports.length,
    )
  }
} catch (error) {
  await fs.writeFile(
    output + `/camera-${mode}-failure.json`,
    JSON.stringify({ error: error.message, detail, page_errors: errors }, null, 2),
  )
  console.error(error.message)
  process.exitCode = 1
} finally {
  await browser.close()
}
