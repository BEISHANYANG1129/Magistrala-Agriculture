const outputTypes = {
  channels: 'channel',
  email: 'email',
  save_remote_pg: 'postgres',
  save_senml: 'senMl',
  save_json: 'json',
  alarms: 'alarm',
  slack: 'slack',
}
const logicTypes = ['editor', 'comparison', 'custom']
const clone = (v) => JSON.parse(JSON.stringify(v))
export function removeFlowNode(draft, key) {
  const flow = decodeRuleFlow(draft.metadata)
  const binding = flowBindings(draft, flow).find((b) => b.key === key)
  if (!flow || !binding) return
  flow.nodes = flow.nodes.filter((n) => n.id !== binding.node.id)
  flow.edges = flow.edges.filter(
    (e) => e.source !== binding.node.id && e.target !== binding.node.id,
  )
  draft.metadata = { ...draft.metadata, flow: encodeRuleFlow(flow) }
}
export function changeOutputFlowType(draft, index, type) {
  const flow = decodeRuleFlow(draft.metadata),
    output = draft.outputs[index]
  if (!flow || !output) return
  const binding = flowBindings(draft, flow).find((b) => b.key === 'output-' + (output.id || index))
  if (binding?.existing) binding.node.type = outputTypes[type] || type
  draft.metadata = { ...draft.metadata, flow: encodeRuleFlow(flow) }
}
export function decodeRuleFlow(metadata = {}) {
  if (!metadata.flow) return null
  try {
    const value = JSON.parse(
      new TextDecoder('utf-8', { fatal: true }).decode(
        Uint8Array.from(atob(metadata.flow), (c) => c.charCodeAt(0)),
      ),
    )
    if (!Array.isArray(value.nodes) || !Array.isArray(value.edges)) throw new Error('shape')
    return value
  } catch {
    throw new Error(
      'The saved rule canvas cannot be decoded. Keep the original metadata and repair it before editing the canvas. / 已保存的规则画布无法解析，请保留原元数据并修复后再编辑。',
    )
  }
}
export function encodeRuleFlow(flow) {
  return btoa(
    Array.from(new TextEncoder().encode(JSON.stringify(flow)), (b) => String.fromCharCode(b)).join(
      '',
    ),
  )
}
export function flowBindings(draft, flow = decodeRuleFlow(draft.metadata)) {
  const available = [...(flow?.nodes || [])],
    used = new Set()
  const take = (key, types) => {
    const found = available.find((n) => types.includes(n.type) && !used.has(n.id))
    if (found) used.add(found.id)
    return {
      key,
      node: found || { id: key, type: types[0], position: { x: 0, y: 0 } },
      existing: !!found,
    }
  }
  const bindings = []
  if (draft.input_channel) bindings.push(take('input', ['input']))
  if (draft.logic?.value) bindings.push(take('logic', logicTypes))
  for (const [i, o] of (draft.outputs || []).entries())
    bindings.push(take('output-' + (o.id || i), [outputTypes[o.type] || o.type]))
  return bindings
}
export function ruleCanvasState(draft) {
  const flow = decodeRuleFlow(draft.metadata),
    bindings = flowBindings(draft, flow),
    legacy = draft.metadata?._agri_ui?.rule_canvas || {}
  const positions = {}
  for (const b of bindings) {
    const p = b.existing ? b.node.position : legacy[b.key]
    if (Number.isFinite(p?.x) && Number.isFinite(p?.y)) positions[b.key] = { x: p.x, y: p.y }
  }
  const keys = new Map(bindings.map((b) => [b.node.id, b.key]))
  let edges
  if (flow)
    edges = flow.edges
      .filter((e) => keys.has(e.source) && keys.has(e.target))
      .map((e) => ({ ...e, source: keys.get(e.source), target: keys.get(e.target) }))
  else {
    edges = []
    if (draft.input_channel && draft.logic?.value)
      edges.push({ id: 'input-logic', source: 'input', target: 'logic' })
    if (draft.logic?.value)
      for (const b of bindings.filter((b) => b.key.startsWith('output-')))
        edges.push({ id: 'logic-' + b.key, source: 'logic', target: b.key })
  }
  return { positions, edges }
}
export function writeRuleFlow(draft, positions, edges, channels = []) {
  const previous = decodeRuleFlow(draft.metadata),
    bindings = flowBindings(draft, previous),
    state = ruleCanvasState(draft)
  positions = positions || state.positions
  edges = edges || state.edges
  const knownTypes = ['input', ...logicTypes, ...Object.values(outputTypes)]
  const unknown = (previous?.nodes || []).filter((n) => !knownTypes.includes(n.type))
  const nodes = bindings.map((b, i) => ({
    ...clone(b.node),
    position: positions[b.key] || {
      x: b.key === 'input' ? 20 : b.key === 'logic' ? 360 : 700,
      y: b.key.startsWith('output-')
        ? Math.max(0, i - (draft.input_channel ? 1 : 0) - (draft.logic?.value ? 1 : 0)) * 230 + 20
        : 20,
    },
  }))
  const ids = new Map(bindings.map((b) => [b.key, b.node.id])),
    unknownIds = new Set(unknown.map((n) => n.id)),
    allIds = new Set([...nodes, ...unknown].map((n) => n.id))
  const preservedEdges = (previous?.edges || []).filter(
    (e) =>
      (unknownIds.has(e.source) || unknownIds.has(e.target)) &&
      allIds.has(e.source) &&
      allIds.has(e.target),
  )
  const flow = {
    ...previous,
    outputMetadata: {
      ...previous?.outputMetadata,
      inputChannel:
        channels.find((c) => c.id === draft.input_channel)?.name ||
        previous?.outputMetadata?.inputChannel ||
        '',
      outputChannel:
        channels.find(
          (c) => c.id === (draft.outputs || []).find((o) => o.type === 'channels')?.channel,
        )?.name ||
        previous?.outputMetadata?.outputChannel ||
        '',
    },
    nodes: [...nodes, ...unknown],
    edges: [
      ...edges
        .filter((e) => ids.has(e.source) && ids.has(e.target))
        .map((e) => ({ ...e, source: ids.get(e.source), target: ids.get(e.target) })),
      ...preservedEdges,
    ],
  }
  return { ...draft.metadata, flow: encodeRuleFlow(flow) }
}
