export const comparisonOperators = ['==', '~=', '>', '<', '>=', '<=']
const path = /^[A-Za-z_][A-Za-z0-9_]*(?:(?:\.[A-Za-z_][A-Za-z0-9_]*)|(?:\[\d+\]))*$/
export function comparisonScript(blocks) {
  if (!blocks.length) throw new Error('Add a comparison / 请添加比较条件')
  const clauses = blocks
    .map((b, i) => {
      if (!path.test(b.input) || !comparisonOperators.includes(b.operator))
        throw new Error(
          'Use a field path such as message.payload[1].v / 请输入字段路径，例如 message.payload[1].v',
        )
      let value
      if (b.type === 'number') {
        if (!String(b.value).trim() || !Number.isFinite(Number(b.value)))
          throw new Error('Invalid comparison number / 比较数值无效')
        value = String(Number(b.value))
      } else if (b.type === 'boolean') value = String(b.value) === 'true' ? 'true' : 'false'
      else
        value =
          '"' +
          String(b.value)
            .replace(/\\/g, '\\\\')
            .replace(/"/g, '\\"')
            .replace(/\n/g, '\\n')
            .replace(/\r/g, '\\r') +
          '"'
      return (
        (i ? (b.join === 'or' ? ' or ' : ' and ') : '') + b.input + ' ' + b.operator + ' ' + value
      )
    })
    .join('')
  return `function logicFunction()\n  if ${clauses} then\n    return true\n  else\n    return false\n  end\nend\n\nreturn logicFunction()`
}
export function parseComparison(script) {
  const condition = script.match(
    /^\s*function\s+logicFunction\(\)\s+if\s+([\s\S]*?)\s+then\s+return true\s+else\s+return false\s+end\s+end\s+return logicFunction\(\)\s*$/,
  )?.[1]
  if (!condition) return []
  const re =
    /(?:^|\s+(and|or)\s+)([A-Za-z_][\w.\[\]]*)\s*(==|~=|>=|<=|>|<)\s*("(?:\\.|[^"\\])*"|true|false|-?\d+(?:\.\d+)?(?:e[+-]?\d+)?)/gi
  const result = []
  let match,
    end = 0
  while ((match = re.exec(condition))) {
    if (condition.slice(end, match.index).trim()) return []
    end = re.lastIndex
    let value = match[4],
      type = value.startsWith('"')
        ? 'string'
        : ['true', 'false'].includes(value)
          ? 'boolean'
          : 'number'
    if (type === 'string') {
      try {
        value = JSON.parse(value)
      } catch {
        return []
      }
    }
    result.push({ input: match[2], operator: match[3], type, value, join: match[1] || 'and' })
  }
  return condition.slice(end).trim() ? [] : result
}
