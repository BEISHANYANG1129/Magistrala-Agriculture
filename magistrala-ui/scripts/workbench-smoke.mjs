// Local integration checks. Never dispatches a physical device command.
import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
import path from 'node:path'
const root = 'D:/personal/magistrala'
const output = path.join(root, 'output/multi-agent-rebuild/implementation')
await fs.mkdir(output, { recursive: true })
const env = Object.fromEntries(
  (await fs.readFile(path.join(root, 'magistrala/docker/.env'), 'utf8'))
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
const workspace = 'a14a9eeb-da40-4f82-babe-aa3a5204ecbe'
const base = 'http://127.0.0.1:5173'
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const page = await browser.newPage({ viewport: { width: 1500, height: 1050 } })
const checks = [],
  errors = []
page.on('pageerror', (e) => errors.push(e.message))
async function record(name, detail = {}) {
  checks.push({ name, ...detail })
  await fs.writeFile(
    path.join(output, 'smoke.json'),
    JSON.stringify({ checks, pageErrors: errors }, null, 2),
  )
  console.log(name, JSON.stringify(detail))
}
async function api(suffix, body) {
  return page.evaluate(
    async ({ suffix, body, workspace }) => {
      const { request } = await import('/src/api/platform.js')
      return request('/api/agent/v2/workbench' + suffix, {
        method: body === undefined ? 'GET' : 'POST',
        body,
        headers: { 'X-Workspace-ID': workspace },
        timeout: 620000,
      })
    },
    { suffix, body, workspace },
  )
}
try {
  await page.goto(base + '/login')
  await page.getByLabel('邮箱或用户名').fill(env.ATOM_ADMIN_USERNAME)
  await page.getByLabel('密码', { exact: true }).fill(env.ATOM_ADMIN_SECRET)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/domains')
  await page.goto(`${base}/domain/${workspace}/agent`)
  await page.getByText('服务已连接', { exact: true }).waitFor({ timeout: 30000 })
  if (
    await page
      .locator('.sidebar a[href$="/field-map"], .sidebar a[href$="/agri-integration"]')
      .count()
  )
    throw new Error('retired pages still in navigation')
  await record('native login, workbench and simplified navigation', { ok: true })
  await page.screenshot({ path: path.join(output, 'workbench-desktop.png'), fullPage: true })
  const replay = await api('/query', {
    mode: 'history',
    start: '2026-10-26T00:00:00+08:00',
    end: '2026-10-28T00:00:00+08:00',
  })
  if (
    replay.rows !== 3996 ||
    replay.metrics.length !== 14 ||
    replay.metrics.find((m) => m.key === 'co2').below_range !== 3136
  )
    throw new Error('replay does not match audited data')
  await record('real historical replay and quality flags', {
    ok: true,
    rows: replay.rows,
    metrics: replay.metrics.length,
  })
  const live = await api('/query', {
    mode: 'platform',
    channel_id: '78dc745a-4f32-4ecd-b92d-90bf247f48a1',
    publisher_id: 'dc96f494-a001-4b40-880e-e338a40771c3',
    start: '2026-10-27T21:55:00+08:00',
    end: '2026-10-27T22:09:00+08:00',
  })
  if (!live.rows || live.mode !== 'platform')
    throw new Error('native platform evidence unavailable')
  await record('authenticated native Reader query', {
    ok: true,
    rows: live.rows,
    metrics: live.metrics.length,
  })
  const automationTab = page.getByRole('button', { name: '自动化任务', exact: true })
  await automationTab.click()
  await page.getByLabel('任务目标', { exact: true }).fill('模拟开启阀门 5 秒')
  await page.getByRole('button', { name: '运行任务', exact: true }).click()
  const confirm = page.getByRole('button', { name: '确认并模拟执行', exact: true })
  await confirm.waitFor()
  if (await confirm.isEnabled()) throw new Error('confirmation checkbox bypassed')
  await page.locator('.result-summary input[type="checkbox"]').check()
  await confirm.click()
  await page.getByText('模拟完成', { exact: true }).first().waitFor()
  await page.getByText('模拟适配器回读', { exact: true }).waitFor()
  await record('exact human confirmation and simulated receipt', { ok: true })
  await page.screenshot({ path: path.join(output, 'workbench-execution.png'), fullPage: true })
  if (process.argv.includes('--automation')) {
    const before = await api('/automation')
    const goal = '概述这段时间的环境，并检查这张叶片，列出需要核实的问题。'
    await page.getByLabel('任务目标', { exact: true }).fill(goal)
    await page.getByText('观测时段', { exact: true }).click()
    await page.getByLabel('附上观测时段', { exact: true }).check()
    await page.getByText('叶片照片', { exact: true }).click()
    await page
      .locator('.automation input[type=file]')
      .setInputFiles(
        path.join(
          root,
          'output/go1-channel-pilot/captures/f14dc9b7-b2d1-4645-9024-062b33082b4c.jpg',
        ),
      )
    const writes = []
    const track = (r) => {
      if (r.method() === 'POST' && r.url().includes('/api/agent/v2/workbench'))
        writes.push(new URL(r.url()).pathname)
    }
    page.on('request', track)
    const accepted = page.waitForResponse(
      (r) =>
        r.url().endsWith('/workbench/automation') &&
        r.request().method() === 'POST' &&
        r.status() === 202,
    )
    await page.getByRole('button', { name: '运行任务', exact: true }).click()
    await accepted
    await page.reload()
    await page.getByText('服务已连接', { exact: true }).waitFor()
    await automationTab.click()
    await page
      .getByRole('button', { name: '补充资料或继续任务', exact: true })
      .waitFor({ timeout: 900000 })
    page.off('request', track)
    const after = await api('/automation')
    const row = after.find((task) => !before.some((old) => old.id === task.id))
    const detail = await api('/automation/' + row?.id)
    if (
      writes.join(',') !== '/api/agent/v2/workbench/automation' ||
      after.length !== before.length + 1 ||
      detail.task.question !== goal ||
      detail.task.status !== 'needs_input' ||
      detail.task.automation.needs.join(',') !== 'image' ||
      !detail.reports.some((r) => r.task === 'environment' && r.claims?.length) ||
      !detail.reports.some((r) => r.task === 'image' && r.workflow?.vision?.subject === 'non_leaf')
    )
      throw new Error('The combined task did not run both branches automatically')
    await page.getByText('查看完整报告', { exact: true }).click()
    await page.getByText('质量诊断', { exact: true }).first().waitFor()
    if ((await page.getByText('质量诊断', { exact: true }).count()) !== 2)
      throw new Error('Both branch quality reports should be visible')
    await page.getByText('视觉工具初步观察', { exact: true }).waitFor()
    await page.screenshot({ path: path.join(output, 'workbench-automation.png'), fullPage: true })
    await fs.writeFile(path.join(output, 'automation-result.json'), JSON.stringify(detail, null, 2))
    await record(
      'one submission ran both analyses and requested a replacement image in the same task',
      { ok: true, id: row.id },
    )
  }
  if (process.argv.includes('--model')) {
    await page.getByRole('button', { name: '问答助手', exact: true }).click()
    const previousIds = new Set((await api('/history')).map((item) => item.id))
    const question = '气压单位是什么？原始值如何换算？'
    await page.getByLabel('你的问题', { exact: true }).fill(question)
    const response = page.waitForResponse(
      (r) => r.url().endsWith('/v2/workbench/chat') && r.request().method() === 'POST',
      { timeout: 620000 },
    )
    await page.getByRole('button', { name: '发送', exact: true }).click()
    const http = await response
    if (!http.ok() || !http.headers()['content-type']?.includes('application/x-ndjson'))
      throw new Error('QA did not return a stream')
    await page
      .getByRole('button', { name: '停止生成', exact: true })
      .waitFor({ state: 'hidden', timeout: 620000 })
    const run = (await api('/history')).find(
      (item) => item.question === question && !previousIds.has(item.id),
    )
    if (!run) throw new Error('QA stream did not save a final result')
    await fs.writeFile(path.join(output, 'qwen-qa-result.json'), JSON.stringify(run, null, 2))
    if (!http.ok() || run.status !== 'completed' || !run.claims?.length)
      throw new Error('Qwen QA did not pass: ' + JSON.stringify(run.limitations))
    await record('real Qwen3.5-9B QA, RAG and source verification', {
      ok: true,
      id: run.id,
      claims: run.claims.length,
    })
    await page.locator('.answer-claim').first().waitFor()
    await page.screenshot({ path: path.join(output, 'workbench-qa.png'), fullPage: true })
  }
  if (process.argv.includes('--image')) {
    const image = await fs.readFile(
      path.join(root, 'output/go1-channel-pilot/captures/f14dc9b7-b2d1-4645-9024-062b33082b4c.jpg'),
    )
    const run = await api('/runs', {
      task: 'image',
      question: '这张图片能用于叶片病虫害判断吗？请只说明可见内容和适用性。',
      crop: '',
      image: image.toString('base64'),
      image_source: 'user_upload',
    })
    await fs.writeFile(path.join(output, 'qwen-image-result.json'), JSON.stringify(run, null, 2))
    if (
      run.status !== 'needs_input' ||
      !run.image_hash ||
      run.workflow?.vision?.subject !== 'non_leaf'
    )
      throw new Error('Qwen image pipeline did not pass: ' + JSON.stringify(run.limitations))
    await record('real Qwen image pipeline with existing indoor Go1 image', {
      ok: true,
      agriculturalAccuracyValidated: false,
      id: run.id,
    })
  }
  if (process.argv.includes('--common')) {
    const run = await api('/runs', { task: 'qa', question: '光合作用的基本过程是什么？' })
    await fs.writeFile(path.join(output, 'qwen-common-result.json'), JSON.stringify(run, null, 2))
    if (
      run.status !== 'completed' ||
      run.retrieval !== 'no_hit' ||
      !run.claims?.every((c) => c.kind === 'common' && c.citations.length === 0)
    )
      throw new Error('General-knowledge QA failed: ' + JSON.stringify(run.limitations))
    await record('general-knowledge QA without fabricated citations', { ok: true, id: run.id })
  }
  if (process.argv.includes('--environment')) {
    const run = await api('/runs', {
      task: 'environment',
      question: '简要说明这段时间的状况，比较两个时段的空气温度和数据质量。',
      query: {
        mode: 'history',
        start: '2026-10-26T00:00:00+08:00',
        end: '2026-10-27T00:00:00+08:00',
      },
      compare: {
        start: '2026-10-27T00:00:00+08:00',
        end: '2026-10-28T00:00:00+08:00',
      },
    })
    await fs.writeFile(
      path.join(output, 'qwen-environment-result.json'),
      JSON.stringify(run, null, 2),
    )
    if (
      !['completed', 'evidence_summary'].includes(run.status) ||
      !run.claims?.some((c) => c.kind === 'observation') ||
      !run.comparison?.rows ||
      !run.workflow?.quality?.length ||
      !run.workflow?.snapshot
    )
      throw new Error('Environmental explanation failed: ' + JSON.stringify(run.limitations))
    for (const window of [run.evidence, run.comparison]) {
      const temperature = window.metrics.find((metric) => metric.key === 'air_temperature')
      if (!run.claims.some((claim) => claim.citations.includes(temperature?.id)))
        throw new Error('Comparison missed an explicitly requested temperature statistic')
    }
    await record('real Qwen environmental explanation with computed evidence', {
      ok: true,
      id: run.id,
    })
    const plan = await api('/plans', { action: 'open', duration_seconds: 5, basis_run_id: run.id })
    const receipt = await api('/simulate', {
      id: plan.id,
      revision: plan.plan.revision,
      confirm: true,
    })
    if (
      plan.plan.basis_snapshot !== run.workflow.snapshot ||
      receipt.status !== 'simulated_completed' ||
      receipt.approval?.revision !== plan.plan.revision ||
      receipt.execution_feedback?.state !== 'closed' ||
      receipt.execution_feedback?.transitions.at(-1)?.offset_seconds !== 5
    )
      throw new Error('Analysis-to-simulator handoff failed')
    await record('analysis snapshot linked to confirmed simulation and closed readback', {
      ok: true,
    })
  }
  await page.setViewportSize({ width: 390, height: 844 })
  await automationTab.click()
  await page.screenshot({ path: path.join(output, 'workbench-mobile.png'), fullPage: true })
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2)
  if (overflow) throw new Error('mobile horizontal overflow')
  await record('mobile workbench layout', { ok: true })
  if (errors.length) throw new Error('browser runtime errors: ' + errors.join('; '))
} catch (e) {
  await record('failure', { ok: false, message: e.message })
  await page
    .screenshot({ path: path.join(output, 'workbench-failure.png'), fullPage: true })
    .catch(() => {})
  process.exitCode = 1
} finally {
  await browser.close()
}
