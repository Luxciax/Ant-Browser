import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const root = path.resolve(import.meta.dirname, '..')
const resolve = (...parts) => path.join(root, ...parts)
const read = (file) => fs.readFileSync(resolve(file), 'utf8')
const json = (file) => JSON.parse(read(file))
const exists = (file) => fs.existsSync(resolve(file))
const fail = (message) => {
  console.error(`repository consistency: ${message}`)
  process.exitCode = 1
}

const wails = json('wails.json')
const pkg = json('frontend/package.json')
const lock = json('frontend/package-lock.json')
const version = String(wails.info?.productVersion || '').trim()
const versions = new Map([
  ['wails.json', version],
  ['frontend/package.json', String(pkg.version || '').trim()],
  ['frontend/package-lock.json', String(lock.version || '').trim()],
  ['frontend/package-lock.json packages[""]', String(lock.packages?.['']?.version || '').trim()],
])

if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(version)) {
  fail(`invalid product version: ${version || '<empty>'}`)
}
for (const [source, value] of versions) {
  if (value !== version) fail(`${source} version ${value || '<empty>'} != ${version}`)
}

const releaseNotes = `publish/release-notes-${version}.md`
if (!exists(releaseNotes)) fail(`missing ${releaseNotes}`)

const readme = read('README.md')
if (!readme.includes(`当前源码版本：\`${version}\``)) fail('README current version is stale')
if (readme.includes('black-ant/Ant-Browser')) fail('README still points to the old repository')
for (const doc of ['docs/architecture.md', 'docs/security-and-evidence.md']) {
  if (!exists(doc)) fail(`missing canonical document: ${doc}`)
  if (!readme.includes(`(${doc})`)) fail(`README does not link canonical document: ${doc}`)
}

const agents = read('AGENTS.md')
if (/\b[A-Za-z]:\\/.test(agents)) fail('AGENTS.md contains a Windows absolute path')

const goMod = read('go.mod')
const wailsMatch = goMod.match(/^\s*github\.com\/wailsapp\/wails\/v2\s+(v\S+)/m)
if (!wailsMatch) fail('go.mod does not declare github.com/wailsapp/wails/v2')
const wailsVersion = wailsMatch?.[1] || ''

for (const workflow of [
  '.github/workflows/publish-windows.yml',
  '.github/workflows/publish-linux.yml',
  '.github/workflows/publish-macos.yml',
]) {
  const text = read(workflow)
  const hardcoded = text.match(/wailsapp\/wails\/v2\/cmd\/wails@(v\d+\.\d+\.\d+)/)
  if (hardcoded) fail(`${workflow} hardcodes Wails ${hardcoded[1]}; derive it from go.mod (${wailsVersion})`)
}

const windowsWorkflow = read('.github/workflows/publish-windows.yml')
for (const expected of [
  `branches: [release/${version}]`,
  `RELEASE_VERSION: ${version}`,
  `publish/release-notes-${version}.md`,
]) {
  if (!windowsWorkflow.includes(expected)) fail(`publish-windows.yml is stale: missing ${expected}`)
}

const stalePhaseDocs = fs.readdirSync(resolve('docs'), { withFileTypes: true })
  .filter((entry) => entry.isFile() && /^phase\d.*\.md$/i.test(entry.name))
  .map((entry) => entry.name)
if (stalePhaseDocs.length) fail(`phase snapshot docs remain in docs/: ${stalePhaseDocs.join(', ')}`)
if (exists('docs/plan')) {
  const planFiles = fs.readdirSync(resolve('docs/plan')).filter((name) => !name.startsWith('.'))
  if (planFiles.length) fail(`stale plan files remain in docs/plan: ${planFiles.join(', ')}`)
}

const skillDirs = [
  'skills/ant-chrome-openclaw',
  'skills/ant-chrome-openclaw-automation-plan',
]
for (const dir of skillDirs) {
  const skillFile = `${dir}/SKILL.md`
  const metadataFile = `${dir}/agents/openai.yaml`
  if (!exists(skillFile)) {
    fail(`missing skill entrypoint: ${skillFile}`)
    continue
  }
  if (!exists(metadataFile)) fail(`missing skill UI metadata: ${metadataFile}`)
  const source = read(skillFile)
  const match = source.match(/^---\r?\n([\s\S]*?)\r?\n---/)
  if (!match) {
    fail(`invalid skill frontmatter: ${skillFile}`)
    continue
  }
  const keys = match[1]
    .split(/\r?\n/)
    .map((line) => line.match(/^([A-Za-z0-9_-]+):/)?.[1])
    .filter(Boolean)
    .sort()
  if (keys.join(',') !== 'description,name') {
    fail(`${skillFile} frontmatter keys must be exactly name,description; got ${keys.join(',')}`)
  }
}

const currentContextFiles = [
  'README.md',
  'AGENTS.md',
  '.github/workflows/publish-windows.yml',
  '.github/workflows/publish-linux.yml',
  '.github/workflows/publish-macos.yml',
  'bat/README.md',
  'publish/linux/README.md',
  'publish/mac/README.md',
  'docs/architecture.md',
  'docs/security-and-evidence.md',
  'docs/evidence-provenance-v1.md',
  ...skillDirs.flatMap((dir) => {
    const out = []
    const walk = (relative) => {
      for (const entry of fs.readdirSync(resolve(relative), { withFileTypes: true })) {
        const child = path.posix.join(relative.replaceAll('\\', '/'), entry.name)
        if (entry.isDirectory()) walk(child)
        else if (/\.(md|yaml|yml|json)$/i.test(entry.name)) out.push(child)
      }
    }
    walk(dir)
    return out
  }),
]
const stalePatterns = [
  ['old repository', /black-ant\/Ant-Browser/],
  ['old Wails pin', /v2\.12\.0/],
  ['deleted delivery status', /delivery-status\.md/],
  ['stale phase date', /2026-04-09/],
  ['machine-specific path', /\b[A-Za-z]:\\(?:Users|AI|code|projects)\\/i],
]
for (const file of currentContextFiles) {
  const source = read(file)
  for (const [label, pattern] of stalePatterns) {
    if (pattern.test(source)) fail(`${file} contains ${label}`)
  }
}

if (exists('frontend/src/ui-v2/UiV2Shell.tsx')) {
  fail('stale UI V2 surface remains: frontend/src/ui-v2/UiV2Shell.tsx')
}
for (const staleUiDir of ['frontend/src/ui-v2/pages', 'frontend/src/ui-v2/components']) {
  if (exists(staleUiDir) && fs.readdirSync(resolve(staleUiDir)).length > 0) {
    fail(`stale UI V2 surface remains: ${staleUiDir}`)
  }
}

if (process.exitCode) process.exit(process.exitCode)
console.log(`repository consistency OK: version=${version}, wails=${wailsVersion}`)
