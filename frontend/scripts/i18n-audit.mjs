import fs from 'node:fs'

const source = fs.readFileSync(new URL('../src/i18n.tsx', import.meta.url), 'utf8')
const en = source.match(/'en-US':\s*\{([\s\S]*?)\n\s*\},\n\s*'zh-CN'/)?.[1] || ''
const zh = source.match(/'zh-CN':\s*\{([\s\S]*?)\n\s*\},\n\s*\}/)?.[1] || ''
const keys = (block) => new Set([...block.matchAll(/\b([A-Za-z][A-Za-z0-9_]*):\s*['`]/g)].map((match) => match[1]))
const enKeys = keys(en)
const zhKeys = keys(zh)
const missing = [...enKeys].filter((key) => !zhKeys.has(key))

if (missing.length) {
  console.error(`i18n audit failed: zh-CN is missing ${missing.length} keys: ${missing.join(', ')}`)
  process.exit(1)
}
console.log(`i18n audit passed: ${enKeys.size} en-US keys and ${zhKeys.size} zh-CN keys`)
