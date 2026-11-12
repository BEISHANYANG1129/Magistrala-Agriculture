// Local model + authenticated platform checks. Physical control is unavailable.
import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
import path from 'node:path'

const root = 'D:/personal/magistrala'
const output = path.join(root, 'output/multi-agent-rebuild/implementation')
const env = Object.fromEntries(
  (await fs.readFile(path.join(root, 'magistrala/docker/.env'), 'utf8'))
    .split(/\r?\n/)
    .filter((line) => line.includes('=') && !line.trim().startsWith('#'))
    .map((line) => {
      const i = line.indexOf('=')
      return [
        line.slice(0, i),
        line
          .slice(i + 1)
          .trim()
          .replace(/^['"]|['"]$/g, ''),
      ]
    }),
)
const weather = 'a14a9eeb-da40-4f82-babe-aa3a5204ecbe'
const camera = 'cccd00c1-a12c-4d6e-9aa9-d2c52906f2e5'
const base = 'http://127.0.0.1:5173'
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const page = await browser.newPage({ viewport: { width: 1500, height: 1100 } })
const checks = [],
  errors = [],
  failures = []
let critical = true
page.on('pageerror', (e) => errors.push(e.message))
let workspace = weather
async function persist(extra = {}) {
  await fs.mkdir(output, { recursive: true })
  await fs.writeFile(
    path.join(output, 'semantic-planner-smoke.json'),
    JSON.stringify(
      { checked_at: new Date().toISOString(), checks, failures, page_errors: errors, ...extra },
      null,
      2,
    ),
  )
}
async function api(suffix, body) {
  return page.evaluate(
    async ({ suffix, body, workspace }) => {
      const { request } = await import('/src/api/platform.js')
      return request('/api/agent/v2/workbench' + suffix, {
        method: body === undefined ? 'GET' : 'POST',
        body,
        headers: { 'X-Workspace-ID': workspace },
        timeout: 20000,
      })
    },
    { suffix, body, workspace },
  )
}
async function settle(id) {
  const until = Date.now() + 600000
  while (Date.now() < until) {
    const detail = await api('/automation/' + id)
    if (detail.task.status !== 'running') return detail
    await page.waitForTimeout(1000)
  }
  throw new Error('Task did not settle: ' + id)
}
function assert(ok, message, detail) {
  if (!ok) {
    failures.push({ message, detail })
    if (critical) throw new Error(message + '\n' + JSON.stringify(detail))
    console.log('FAILED', message, detail.task?.id)
  }
}
async function record(name, detail) {
  checks.push({
    name,
    id: detail.task.id,
    goal: detail.task.question,
    status: detail.task.status,
    plan: detail.task.automation.goal_plan,
    needs: detail.task.automation.input_needs,
    evidence: detail.reports.map((r) => ({
      task: r.task,
      status: r.status,
      rows: r.evidence?.rows,
      vision: r.workflow?.vision?.subject,
    })),
  })
  await persist()
  console.log(name, detail.task.status, detail.task.id)
}
async function submit(goal) {
  const created = await api('/automation', { goal, time_zone: 'Asia/Shanghai' })
  return settle(created.task.id)
}
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(env.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page.goto(`${base}/domain/${workspace}/agent`)
  await page.getByText('服务已连接', { exact: true }).waitFor({ timeout: 30000 })
  await page.getByRole('button', { name: '自动化任务', exact: true }).click()
  await page.getByLabel('任务目标', { exact: true }).fill('开启闸门10秒')
  const createdResponse = page.waitForResponse(
    (r) => r.url().endsWith('/automation') && r.request().method() === 'POST',
  )
  await page.getByRole('button', { name: '运行任务', exact: true }).click()
  const created = await (await createdResponse).json()
  let detail = await settle(created.task.id)
  assert(
    detail.task.status === 'needs_input' &&
      detail.task.automation.needs.includes('execution_mode') &&
      detail.task.automation.goal_plan.execution.duration_seconds === 10 &&
      !detail.task.plan,
    'Real gate request was not understood correctly',
    detail,
  )
  await page.getByRole('button', { name: '改为模拟', exact: true }).waitFor({ timeout: 10000 })
  await page.screenshot({
    path: path.join(output, 'semantic-planner-real-gap.png'),
    fullPage: true,
  })
  await record('gate request understood, real capability gap explained', detail)
  await page.reload()
  await page.getByRole('button', { name: '自动化任务', exact: true }).click()
  await page.getByRole('button', { name: '改为模拟', exact: true }).waitFor({ timeout: 10000 })
  await page.getByRole('button', { name: '改为模拟', exact: true }).click()
  detail = await settle(created.task.id)
  assert(
    detail.task.status === 'waiting_confirmation' &&
      detail.task.plan.duration_seconds === 10 &&
      detail.task.automation.execution_mode === 'simulation' &&
      !detail.task.execution_feedback,
    'Explicit mode choice did not preserve task',
    detail,
  )
  const confirm = page.getByRole('button', { name: '确认并模拟执行', exact: true })
  await confirm.waitFor({ timeout: 10000 })
  assert(!(await confirm.isEnabled()), 'Simulation confirmation was bypassed', detail)
  await page.locator('.result-summary input[type="checkbox"]').check()
  await confirm.click()
  await page.getByText('模拟适配器回读', { exact: true }).waitFor({ timeout: 10000 })
  detail = await api('/automation/' + created.task.id)
  assert(
    detail.task.status === 'simulated_completed' &&
      detail.task.execution_feedback.state === 'closed',
    'Simulation readback incomplete',
    detail,
  )
  await record('reload, explicit switch, exact confirmation and readback in same task', detail)
  await page.screenshot({
    path: path.join(output, 'semantic-planner-simulation.png'),
    fullPage: true,
  })
  critical = false

  for (const [goal, seconds] of [
    ['请帮我把闸门打开十秒钟，到点关上，先做个演练。', 10],
    ['演练一下，阀门通水半分钟，然后停止。', 30],
  ]) {
    detail = await submit(goal)
    assert(
      detail.task.status === 'waiting_confirmation' &&
        detail.task.plan.duration_seconds === seconds,
      'Natural action/unit interpretation failed',
      detail,
    )
    await record('natural simulation and duration conversion', detail)
  }
  detail = await submit('不要开闸门，先看看这段时间的温度。')
  assert(
    detail.task.automation.goal_plan?.environment &&
      !detail.task.automation.goal_plan.execution &&
      !detail.task.plan &&
      detail.task.automation.needs?.includes('window'),
    'Negation or unanchored time was lost',
    detail,
  )
  await record('negated action omitted; asks only for missing time', detail)

  detail = await submit('如果温度超过35度，就开启阀门十秒。')
  assert(
    detail.task.status === 'needs_input' &&
      detail.task.automation.goal_plan.execution?.timing === 'conditional' &&
      !detail.task.plan,
    'Conditional request was simplified to immediate action',
    detail,
  )
  await record('conditional request retained, unsupported trigger explained', detail)

  detail = await submit('怎样才能开启闸门十秒？')
  assert(
    detail.task.status === 'needs_input' &&
      !detail.task.automation.goal_plan.execution &&
      !detail.task.plan,
    'Knowledge question became an action',
    detail,
  )
  await record('instructional question does not become an action', detail)

  detail = await submit('把水泵启动半分钟。')
  assert(
    detail.task.status === 'needs_input' &&
      detail.task.automation.execution_match?.capability_id === '' &&
      detail.task.automation.needs.includes('execution') &&
      !detail.task.plan,
    'Pump was mapped to virtual valve',
    detail,
  )
  await record('unsupported device has a specific capability gap', detail)

  detail = await submit('读一下原平台十月二十七日晚上九点五十五到十点零九的空气温度。')
  assert(
    detail.task.status === 'completed' &&
      detail.task.automation.query?.mode === 'platform' &&
      detail.reports[0]?.evidence?.rows === 210,
    'Natural observation time/source interpretation failed',
    detail,
  )
  await record('Chinese spoken time resolves to real Reader observations', detail)

  workspace = camera
  await page.goto(`${base}/domain/${workspace}/agent`)
  await page.getByText('服务已连接', { exact: true }).waitFor({ timeout: 30000 })
  detail = await submit('帮我瞅瞅Go1左侧摄像头最近保存的画面，看看有没有叶片。')
  assert(
    detail.task.status === 'needs_input' &&
      detail.task.automation.photo_source?.origin === 'platform' &&
      detail.reports[0]?.workflow?.vision?.subject === 'non_leaf',
    'Natural photo request did not resolve to scoped Reader photo',
    detail,
  )
  await record('colloquial photo request uses registered camera and checks actual content', detail)
  assert(errors.length === 0, 'Browser errors', errors)
  if (failures.length) throw new Error(`${failures.length} semantic cases failed; see the report.`)
  await persist({ passed: true })
} catch (error) {
  await persist({ passed: false, failure: error.message })
  throw error
} finally {
  await browser.close()
}
