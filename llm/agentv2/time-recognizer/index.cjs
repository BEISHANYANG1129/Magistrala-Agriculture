// The recognizer is local. Only question text and reference times enter this process.
const { recognizeDateTime } = require('@microsoft/recognizers-text-date-time')

let input = ''
process.stdin.setEncoding('utf8')
process.stdin.on('data', (chunk) => { input += chunk })
process.stdin.on('end', () => {
  try {
    const request = JSON.parse(input)
    const reference = new Date(request.reference)
    let text = request.text
    // A day-only correction inherits the selected month from the query slots.
    if (!text.includes('月') && !/\d{4}[-/]\d{1,2}[-/]\d{1,2}/.test(text)) {
      text = text.replace(/(^|[^\d年月])(\d{1,2})[号日]/g, (_, prefix, day) =>
        prefix + reference.getFullYear() + '年' + (reference.getMonth() + 1) + '月' + day + '日')
    }
    const entities = recognizeDateTime(text, 'zh-cn', 0, new Date(request.now), false)
    process.stdout.write(JSON.stringify(entities))
  } catch (error) {
    process.stderr.write(error.message)
    process.exitCode = 1
  }
})
