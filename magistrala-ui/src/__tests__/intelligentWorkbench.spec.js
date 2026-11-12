import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
const mocks = vi.hoisted(() => ({ request: vi.fn(), listAll: vi.fn() }))
vi.mock('../api/platform', () => ({ ...mocks, errorText: (e) => e.message }))
import AgentWorkbench from '../components/AgentWorkbench.vue'

async function setup() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/domain/:id/agent', component: AgentWorkbench }],
  })
  await router.push('/domain/w/agent')
  const wrapper = mount({ template: '<router-view />' }, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}
beforeEach(() => {
  localStorage.setItem('agri-language', 'zh')
  mocks.listAll.mockReset().mockResolvedValue([])
  const threads = new Map()
  let turnCount = 0
  mocks.request.mockReset().mockImplementation(async (path, options = {}) => {
    if (path.endsWith('/status'))
      return {
        model: 'qwen3.5:9b',
        inference_enabled: true,
        history_available: true,
        first_at: '2026-10-26T17:44:17+08:00',
        last_at: '2026-10-27T22:08:34+08:00',
      }
    if (path.endsWith('/conversations'))
      return [...threads.values()]
        .map((turns) => ({
          id: turns[0].id,
          title: turns[0].question,
          updated_at: turns.at(-1).created_at,
        }))
        .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
    if (path.includes('/conversations/')) return threads.get(path.split('/').at(-1))
    if (path.endsWith('/automation')) return []
    if (path.endsWith('/chat')) {
      const run = {
        id: 'r' + ++turnCount,
        created_at: new Date(1700000000000 + turnCount * 1000).toISOString(),
        parent_id: options.body.parent_id,
        task: 'qa',
        question: options.body.question,
        status: 'completed',
        claims: [{ text: '需要核实单位。', kind: 'knowledge', citations: ['k'] }],
        knowledge: [
          {
            id: 'k',
            title: '单位说明',
            text: '气压单位暂定。',
            source: 'project.md',
            version: 'v1',
          },
        ],
        trace: [{ role: 'reviewer', detail: '已核对引用。' }],
        limitations: [],
      }
      if (run.question === '平均气压是多少？') {
        run.status = 'needs_clarification'
        run.clarification = '请提供明确的查询时间范围。'
        run.claims = []
        run.knowledge = []
      }
      if (run.question === '昨天下午') {
        run.query_results = [
          {
            id: 'data-1',
            title: '气压 · A 站',
            text: '平均值 101.00 kPa。',
            answer: '昨天下午的平均气压为 101 kPa。',
            source: '平台 Reader',
          },
        ]
        run.claims = []
        run.knowledge = []
      }
      if (run.question === '10月27日最高气温是多少？') {
        run.query_results = [
          {
            id: 'data-temperature',
            title: '空气温度 · 40366226',
            text: '2026-10-27 00:00:00 +08:00 至 2026-10-28 00:00:00 +08:00，有效样本数 2881，最大值 36.7 Cel。',
            answer: '10月27日的最高气温为 36.7°C。',
            source: '平台 Reader',
          },
        ]
        run.claims = []
        run.knowledge = []
        run.limitations = ['查询依据为平台消息记录时间，采样时间与现场覆盖范围尚待核实。']
      }
      if (run.question === '10月27日整体天气怎么样？') {
        run.synthesized = true
        run.query_results = [
          {
            id: 'data-temperature',
            title: '气温观测',
            text: '有效样本数2881，最大值36.7 Cel。',
            answer: '原始气温摘要。',
            source: '平台 Reader',
          },
          {
            id: 'data-pressure',
            title: '气压观测',
            text: '平均值101.1 kPa。',
            answer: '气压的样本均值为101.1 kPa。',
            source: '平台 Reader',
          },
        ]
        run.claims = [
          {
            text: '这一天气温最高为36.7°C。',
            kind: 'observation',
            citations: ['data-temperature'],
          },
        ]
        run.knowledge = []
        run.answer_notes = ['没有降水、云量观测，无法确定当天晴雨。']
      }
      if (run.parent_id) {
        const turns = [...threads.values()].find((rows) =>
          rows.some((row) => row.id === run.parent_id),
        )
        turns.push(run)
      } else {
        threads.set(run.id, [run])
      }
      if (run.claims.length) options.onRecord({ type: 'claim', claim: run.claims[0] })
      options.onRecord({ type: 'result', run })
      return
    }
    if (path.endsWith('/plans'))
      return {
        id: 'p',
        task: 'execution',
        question: '模拟草案',
        status: 'waiting_confirmation',
        plan: {
          target: 'simulation-only-valve',
          action: 'open',
          duration_seconds: 30,
          stop_action: 'close',
          revision: 'exact',
          expires_at: '2026-11-03T22:00:00+08:00',
        },
        claims: [],
        limitations: [],
        trace: [],
      }
    if (path.endsWith('/simulate'))
      return {
        id: 'p',
        task: 'execution',
        question: '模拟草案',
        status: 'simulated_completed',
        receipt: '没有调用真实设备。',
        claims: [],
        limitations: [],
        trace: [],
      }
    return {}
  })
})
describe('intelligent workbench', () => {
  it('shows reviewed synthesis with query citations and retains uncovered results', async () => {
    const { wrapper } = await setup()
    await wrapper.get('textarea').setValue('10月27日整体天气怎么样？')
    await wrapper.get('form.chat-composer').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.query-answer').text()).toBe('气压的样本均值为101.1 kPa。')
    expect(wrapper.findAll('.query-answer')).toHaveLength(1)
    expect(wrapper.find('.chat-answer').text()).toContain('这一天气温最高为36.7°C。')
    expect(wrapper.find('.chat-answer').text()).toContain('无法确定当天晴雨')
    const citation = wrapper.get('.answer-claim a')
    expect(citation.text()).toBe('气温观测')
    expect(citation.attributes('href')).toContain('data-temperature')
    expect(wrapper.get('.chat-sources').element.open).toBe(false)
    wrapper.unmount()
  })

  it('shows automatic query clarification and resumes it without a manual data window', async () => {
    const { wrapper } = await setup()
    const send = async (text) => {
      await wrapper.get('textarea').setValue(text)
      await wrapper.get('form.chat-composer').trigger('submit')
      await flushPromises()
    }
    await send('平均气压是多少？')
    expect(wrapper.text()).toContain('请提供明确的查询时间范围。')
    await send('昨天下午')
    expect(wrapper.text()).toContain('平均值 101.00 kPa。')
    expect(wrapper.text()).toContain('平台观测记录')
    expect(wrapper.get('.query-answer').text()).toBe('昨天下午的平均气压为 101 kPa。')
    const chat = mocks.request.mock.calls.filter(([url]) => url.endsWith('/chat'))
    expect(chat[0][1].body).toEqual({ task: 'qa', question: '平均气压是多少？' })
    expect(chat[1][1].body).toEqual({ task: 'qa', question: '昨天下午', parent_id: 'r1' })
    wrapper.unmount()
  })

  it('answers naturally and folds the query metadata into sources', async () => {
    const { wrapper } = await setup()
    await wrapper.get('textarea').setValue('10月27日最高气温是多少？')
    await wrapper.get('form.chat-composer').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.query-answer').text()).toBe('10月27日的最高气温为 36.7°C。')
    const sources = wrapper.get('.chat-sources')
    expect(sources.element.open).toBe(false)
    expect(sources.get('summary').text()).toBe('查看依据')
    expect(sources.text()).toContain('2881')
    expect(sources.text()).toContain('平台观测记录')
    expect(sources.text()).not.toContain('Cel')
    expect(wrapper.find('.chat-answer > ul').exists()).toBe(false)
    await sources.get('summary').trigger('click')
    wrapper.unmount()
  })

  it('asks a sourced question and renders source references', async () => {
    const { wrapper } = await setup()
    expect(wrapper.text()).toContain('qwen3.5:9b')
    await wrapper.get('textarea').setValue('气压单位为什么暂定？')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('需要核实单位。')
    expect(wrapper.get('.chat-question').text()).toBe('气压单位为什么暂定？')
    expect(wrapper.get('.chat-answer').text()).not.toContain('农业助手')
    expect(wrapper.get('.chat-answer').text()).not.toMatch(/资料依据|一般常识|处理过程/)
    expect(wrapper.text()).not.toContain('智能体如何协作')
    expect(wrapper.find('a[href="#chat-source-0-k"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('project.md')
    const call = mocks.request.mock.calls.find(([path]) => path.endsWith('/chat'))
    expect(call[1].headers['X-Workspace-ID']).toBe('w')
    expect(call[1].body.task).toBe('qa')
    await wrapper.get('textarea').setValue('还要核实什么？')
    await wrapper.get('textarea').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(wrapper.findAll('.chat-turn')).toHaveLength(2)
    expect(
      mocks.request.mock.calls.filter(([path]) => path.endsWith('/chat')).at(-1)[1].body.parent_id,
    ).toBe('r1')
    expect(wrapper.findAll('.conversation-list button')).toHaveLength(1)
    expect(wrapper.get('.conversation-list button strong').text()).toBe('气压单位为什么暂定？')
    wrapper.unmount()
  })
  it('opens a complete saved chat and appends another turn', async () => {
    const { wrapper } = await setup()
    async function ask(text) {
      await wrapper.get('textarea').setValue(text)
      await wrapper.get('form').trigger('submit')
      await flushPromises()
    }
    await ask('第一段对话')
    await ask('第一段里的追问')
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('新对话'))
      .trigger('click')
    expect(wrapper.findAll('.conversation-list button')).toHaveLength(1)
    await ask('第二段对话')
    const titles = () => wrapper.findAll('.conversation-list strong').map((title) => title.text())
    expect(titles()).toEqual(['第二段对话', '第一段对话'])
    await wrapper.findAll('.conversation-list button')[1].trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.chat-question').map((question) => question.text())).toEqual([
      '第一段对话',
      '第一段里的追问',
    ])
    await ask('重新打开后继续问')
    expect(titles()).toEqual(['第一段对话', '第二段对话'])
    expect(
      mocks.request.mock.calls.filter(([path]) => path.endsWith('/chat')).at(-1)[1].body.parent_id,
    ).toBe('r2')
    wrapper.unmount()
  })
  it('submits one goal and shows both automatically run branches under one task', async () => {
    const { wrapper } = await setup()
    await wrapper.findAll('.workbench-tabs button')[1].trigger('click')
    await flushPromises()
    const text = '概述环境并检查这张叶片'
    mocks.request.mockResolvedValueOnce({
      task: {
        id: 'automation-1',
        question: text,
        status: 'completed',
        automation: {
          updated_at: '2026-11-04T09:00:00Z',
          steps: [
            { role: 'planner', status: 'completed' },
            { role: 'environment', status: 'completed' },
            { role: 'image', status: 'completed' },
            { role: 'review', status: 'completed' },
            { role: 'report', status: 'completed' },
          ],
        },
      },
      reports: [
        { id: 'environment-result', task: 'environment', claims: [{ text: '气温为 30 °C。' }] },
        { id: 'image-result', task: 'image', claims: [{ text: '叶片边缘有褐色区域。' }] },
      ],
    })
    await wrapper.get('textarea').setValue(text)
    expect(wrapper.get('textarea').attributes('readonly')).toBeUndefined()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const writes = mocks.request.mock.calls.filter(([, options]) => options?.method === 'POST')
    expect(writes.map(([path]) => path)).toEqual(['/api/agent/v2/workbench/automation'])
    expect(writes[0][1].body).toEqual({ goal: text, time_zone: expect.any(String) })
    expect(wrapper.get('.steps .branches').text()).toMatch(/环境观测.*图片分析/)
    expect(wrapper.get('.result-summary').text()).toContain('叶片边缘有褐色区域。')
    expect(wrapper.findAll('.task-row')).toHaveLength(1)
    expect(wrapper.find('.conversation-list').exists()).toBe(false)
    wrapper.unmount()
  })
  it('requires confirmation of the automation task draft before simulation', async () => {
    const { wrapper } = await setup()
    await wrapper.findAll('.workbench-tabs button')[1].trigger('click')
    await flushPromises()
    const task = {
      id: 'automation-2',
      question: '模拟开启阀门 5 秒',
      status: 'waiting_confirmation',
      automation: { steps: [{ role: 'simulation', status: 'waiting_confirmation' }] },
      plan: {
        revision: 'exact',
        target: 'simulation-only-valve',
        action: 'open',
        duration_seconds: 5,
      },
    }
    mocks.request.mockResolvedValueOnce({ task, reports: [] })
    await wrapper.get('textarea').setValue(task.question)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const confirm = wrapper.findAll('button').find((b) => b.text() === '确认并模拟执行')
    expect(confirm.attributes('disabled')).toBeDefined()
    await wrapper.get('.result-summary input[type="checkbox"]').setValue(true)
    mocks.request.mockResolvedValueOnce({}).mockResolvedValueOnce({
      task: { ...task, status: 'simulated_completed', receipt: '已读回关闭。' },
      reports: [],
    })
    await confirm.trigger('click')
    await flushPromises()
    expect(mocks.request.mock.calls.find(([path]) => path.endsWith('/simulate'))[1].body).toEqual({
      id: task.id,
      revision: 'exact',
      confirm: true,
    })
    expect(wrapper.text()).toContain('已读回关闭。')
    expect(wrapper.findAll('.task-row')).toHaveLength(1)
    wrapper.unmount()
  })
  it('explains the real-control gap and switches the same task to an unconfirmed simulation', async () => {
    const { wrapper } = await setup()
    await wrapper.findAll('.workbench-tabs button')[1].trigger('click')
    await flushPromises()
    const task = {
      id: 'natural-device-task',
      question: '开启闸门10秒',
      status: 'needs_input',
      automation: {
        steps: [
          { role: 'planner', status: 'completed' },
          { role: 'simulation', status: 'needs_input' },
        ],
        needs: ['execution_mode'],
        input_needs: [
          { field: 'execution_mode', message: '已理解请求，当前尚未接通真实控制和回读接口。' },
        ],
        goal_plan: {
          summary: '开启闸门十秒后关闭',
          execution: { target: '闸门', action: 'open', mode: 'real', duration_seconds: 10 },
        },
      },
    }
    mocks.request.mockResolvedValueOnce({ task, reports: [] })
    await wrapper.get('textarea').setValue(task.question)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.understood-task').text()).toContain('闸门')
    expect(wrapper.get('.needs').text()).toContain('真实控制和回读接口')
    expect(wrapper.get('.understood-task').text()).toContain('真实操作')
    mocks.request.mockResolvedValueOnce({
      task: {
        ...task,
        status: 'waiting_confirmation',
        automation: { ...task.automation, needs: [], execution_mode: 'simulation' },
        plan: {
          target: 'simulation-only-valve',
          action: 'open',
          duration_seconds: 10,
          revision: 'exact',
        },
      },
      reports: [],
    })
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '改为模拟')
      .trigger('click')
    await flushPromises()
    expect(mocks.request.mock.calls.at(-1)[0]).toBe(
      '/api/agent/v2/workbench/automation/natural-device-task/resume',
    )
    expect(mocks.request.mock.calls.at(-1)[1].body).toEqual({ execution_mode: 'simulation' })
    expect(wrapper.findAll('.task-row')).toHaveLength(1)
    expect(
      wrapper
        .findAll('button')
        .find((b) => b.text() === '确认并模拟执行')
        .attributes('disabled'),
    ).toBeDefined()
    expect(mocks.request.mock.calls.some(([path]) => path.endsWith('/simulate'))).toBe(false)
    wrapper.unmount()
  })
  it('accepts a natural-language clarification in the same task', async () => {
    const { wrapper } = await setup()
    await wrapper.findAll('.workbench-tabs button')[1].trigger('click')
    await flushPromises()
    const task = {
      id: 'missing-window',
      question: '看看这段时间的气温',
      status: 'needs_input',
      automation: {
        steps: [],
        needs: ['window'],
        input_needs: [{ field: 'window', message: '尚未确定具体时间。' }],
      },
    }
    mocks.request.mockResolvedValueOnce({ task, reports: [] })
    await wrapper.get('textarea').setValue(task.question)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '补充资料或继续任务')
      .trigger('click')
    await wrapper.findAll('textarea')[1].setValue('就用昨天一整天的')
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(false)
    mocks.request.mockResolvedValueOnce({
      task: { ...task, status: 'completed', automation: { steps: [] } },
      reports: [],
    })
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.request.mock.calls.at(-1)[0]).toBe(
      '/api/agent/v2/workbench/automation/missing-window/resume',
    )
    expect(mocks.request.mock.calls.at(-1)[1].body.clarification).toBe('就用昨天一整天的')
    wrapper.unmount()
  })
  it('clears prior workspace answers and ignores late responses', async () => {
    const { wrapper, router } = await setup()
    let resolve
    mocks.request.mockImplementation(async (path, options) =>
      path.endsWith('/chat')
        ? new Promise((r) => {
            resolve = (run) => {
              options.onRecord({ type: 'result', run })
              r()
            }
          })
        : path.endsWith('/conversations')
          ? []
          : { model: 'qwen3.5:9b', inference_enabled: true },
    )
    await wrapper.get('textarea').setValue('旧工作区问题')
    await wrapper.get('form').trigger('submit')
    await router.push('/domain/other/agent')
    await flushPromises()
    resolve({
      task: 'qa',
      status: 'completed',
      question: 'private old data',
      claims: [],
      knowledge: [],
      trace: [],
    })
    await flushPromises()
    expect(wrapper.text()).not.toContain('private old data')
    expect(wrapper.get('textarea').element.value).toBe('')
    wrapper.unmount()
  })
  it('shows backend errors without a fabricated answer', async () => {
    const { wrapper } = await setup()
    mocks.request.mockRejectedValue(new Error('model unavailable'))
    await wrapper.get('textarea').setValue('测试问题')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.chat-turn [role="alert"]').text()).toBe('暂时无法生成回答，请稍后重试。')
    expect(wrapper.get('.chat-turn details').text()).toContain('model unavailable')
    expect(wrapper.find('.answer-claim').exists()).toBe(false)
    wrapper.unmount()
  })
  it('shows a fixed reply for a completed stream with no reliable answer', async () => {
    const { wrapper } = await setup()
    mocks.request.mockImplementationOnce(async (_, { onRecord }) => {
      onRecord({
        type: 'result',
        run: { id: 'empty', task: 'qa', status: 'no_answer', claims: [] },
      })
    })
    await wrapper.get('textarea').setValue('这个未接入设备的当前读数是多少？')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.chat-answer [role="status"]').text()).toBe(
      '暂时无法可靠回答这个问题。可以补充相关信息，或换一种问法。',
    )
    wrapper.unmount()
  })
  it('shows the time component failure directly in the answer', async () => {
    const { wrapper } = await setup()
    mocks.request.mockImplementationOnce(async (_, { onRecord }) => {
      onRecord({
        type: 'result',
        run: {
          id: 'time-unavailable',
          task: 'qa',
          status: 'time_recognizer_unavailable',
          claims: [],
          limitations: ['时间解析组件不可用，请稍后重试。'],
        },
      })
    })
    await wrapper.get('textarea').setValue('昨天最高温度是多少？')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.chat-answer [role="status"]').text()).toBe(
      '时间解析组件不可用，请稍后重试。',
    )
    expect(wrapper.find('.answer-claim').exists()).toBe(false)
    wrapper.unmount()
  })
  it('streams collapsible thinking separately from claims and cancels the open request', async () => {
    const { wrapper } = await setup()
    let stream
    mocks.request.mockImplementationOnce(async (_, options) => {
      stream = options
      options.onRecord({ type: 'thinking', delta: '查阅' })
      await new Promise((_, reject) =>
        stream.signal.addEventListener('abort', () => reject(new Error('cancelled')), {
          once: true,
        }),
      )
    })
    await wrapper.get('textarea').setValue('气压单位')
    await wrapper.get('form').trigger('submit')
    const thinking = wrapper.get('.chat-thinking')
    expect(thinking.element.open).toBe(false)
    thinking.element.open = true
    stream.onRecord({ type: 'thinking', delta: '资料' })
    stream.onRecord({
      type: 'claim',
      claim: { text: '实时片段', kind: 'knowledge', citations: [] },
    })
    await flushPromises()
    expect(thinking.element.open).toBe(true)
    expect(thinking.get('p').text()).toBe('查阅资料')
    expect(wrapper.get('.answer-claim').text()).toBe('实时片段')
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '停止生成')
      .trigger('click')
    await flushPromises()
    expect(stream.signal.aborted).toBe(true)
    expect(wrapper.get('.chat-turn').text()).toContain('已停止生成')
    expect(wrapper.find('.answer-claim').exists()).toBe(false)
    wrapper.unmount()
  })
})
