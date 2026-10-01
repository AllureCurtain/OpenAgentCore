// Checks a finished build: run `pnpm build` first. Every docs.json page has
// an HTML page and a raw Markdown copy, legacy paths redirect, both landing
// pages exist and llms.txt lists every page.
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { legacyRedirects, readDocsJson } from '../.vitepress/docs-nav.mts'

const dist = fileURLToPath(new URL('../.vitepress/dist/', import.meta.url))
const pages = readDocsJson().navigation.groups.flatMap((g) => g.pages)

function html(page) {
  return resolve(dist, `${page}.html`)
}

test('the build output exists', () => {
  assert.ok(existsSync(resolve(dist, 'index.html')), 'run `pnpm build` before `pnpm test`')
})

test('both landing pages are built', () => {
  for (const [file, marker] of [['index.html', 'lang="en"'], ['zh/index.html', 'lang="zh-CN"']]) {
    const source = readFileSync(resolve(dist, file), 'utf8')
    assert.match(source, /class="landing"/, `${file} renders the landing page`)
    assert.ok(source.includes(marker), `${file} has ${marker}`)
  }
})

test('every documentation page has HTML and a Markdown copy', () => {
  for (const page of pages) {
    assert.ok(existsSync(html(page)), `${page}.html`)
    assert.ok(existsSync(resolve(dist, `${page}.md`)), `${page}.md`)
  }
})

test('legacy paths redirect', () => {
  for (const { from, to } of legacyRedirects()) {
    const source = readFileSync(resolve(dist, `${from}.html`), 'utf8')
    assert.ok(source.includes(`url=`) && source.includes(to.replace(/^\//, '')), `${from} → ${to}`)
  }
})

test('llms.txt lists every page', () => {
  const llms = readFileSync(resolve(dist, 'llms.txt'), 'utf8')
  for (const page of pages) assert.ok(llms.includes(`${page}.md)`), `${page} in llms.txt`)
})
