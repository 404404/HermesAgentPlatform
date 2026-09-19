import fs from 'node:fs'

const source = fs.readFileSync(new URL('../src/i18n.tsx', import.meta.url), 'utf8')
const completion = fs.readFileSync(new URL('../src/i18nCompletion.ts', import.meta.url), 'utf8')
const en = source.match(/'en-US':\s*\{([\s\S]*?)\n\s*\},\n\s*'zh-CN'/)?.[1] || ''
const zh = source.match(/'zh-CN':\s*\{([\s\S]*?)\n\s*\},\n\s*\}/)?.[1] || ''
const keys = (block) => new Set([...block.matchAll(/\b([A-Za-z][A-Za-z0-9_]*):\s*['`]/g)].map((match) => match[1]))
const enKeys = keys(en)
const zhKeys = keys(zh)
const missing = [...enKeys].filter((key) => !zhKeys.has(key))
const completionBlock = (name) => completion.match(new RegExp("export const " + name + ": Record<string, string> = \\{([\\s\\S]*?)\\n\\}", "m"))?.[1] || ""
const enCompletionKeys = keys(completionBlock("enUSCompletion"))
const zhCompletionKeys = keys(completionBlock("zhCNCompletion"))
const missingCompletion = [...enCompletionKeys].filter((key) => !zhCompletionKeys.has(key))

if (missing.length || missingCompletion.length) {
  console.error("i18n audit failed: core missing " + missing.length + " keys (" + missing.join(", ") + "); completion missing " + missingCompletion.length + " keys (" + missingCompletion.join(", ") + ")")
  process.exit(1)
}
console.log("i18n audit passed: " + enKeys.size + " core keys and " + enCompletionKeys.size + " completion keys are present in both en-US and zh-CN")
