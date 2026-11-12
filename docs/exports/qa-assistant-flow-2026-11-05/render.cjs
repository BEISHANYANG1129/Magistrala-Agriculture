const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require(path.join(__dirname, '../../../magistrala-ui/node_modules/@playwright/test'))

async function main() {
  const source = fs.readFileSync(path.join(__dirname, 'qa-assistant-flow.mmd'), 'utf8')
  const renderer = fs.readFileSync(process.argv[2], 'utf8')
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  try {
    const page = await browser.newPage({ viewport: { width: 1600, height: 1100 }, deviceScaleFactor: 1 })
    await page.setContent('<!doctype html><meta charset="utf-8"><body style="margin:0;background:#ffffff"><main id="diagram"></main></body>')
    await page.addScriptTag({ content: renderer })
    const result = await page.evaluate(async source => {
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: 'strict',
        htmlLabels: false,
        theme: 'base',
        fontFamily: 'Microsoft YaHei, Noto Sans SC, sans-serif',
        themeVariables: { fontSize: '20px', lineColor: '#6c7c8c', textColor: '#24374a', edgeLabelBackground: '#ffffff', background: '#ffffff' },
        flowchart: { htmlLabels: false, useMaxWidth: false, curve: 'basis', nodeSpacing: 32, rankSpacing: 40, padding: 16, wrappingWidth: 380 },
      })
      const { svg } = await mermaid.render('qaAssistantFlow', source)
      document.getElementById('diagram').innerHTML = svg
      const element = document.querySelector('svg')
      element.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
      element.style.background = '#ffffff'
      const box = element.viewBox.baseVal
      element.setAttribute('width', Math.ceil(box.width))
      element.setAttribute('height', Math.ceil(box.height))
      await document.fonts.ready
      const bad = [...element.querySelectorAll('text')].some(el => !Number.isFinite(el.getBBox().width))
      if (bad) throw new Error('Invalid text layout')
      return { svg: new XMLSerializer().serializeToString(element), width: Math.ceil(box.width), height: Math.ceil(box.height), nodes: element.querySelectorAll('.node').length }
    }, source)
    fs.writeFileSync(path.join(__dirname, 'qa-assistant-flow.svg'), '<?xml version="1.0" encoding="UTF-8"?>\n' + result.svg)
    await page.setViewportSize({ width: result.width, height: result.height })
    await page.locator('svg').screenshot({ path: path.join(__dirname, 'qa-assistant-flow.png') })
    console.log(JSON.stringify({ width: result.width, height: result.height, nodes: result.nodes, svg: path.join(__dirname, 'qa-assistant-flow.svg'), png: path.join(__dirname, 'qa-assistant-flow.png') }))
  } finally {
    await browser.close()
  }
}
main().catch(error => { console.error(error.message); process.exitCode = 1 })
