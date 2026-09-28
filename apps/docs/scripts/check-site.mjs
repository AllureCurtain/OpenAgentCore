#!/usr/bin/env node
// Check every English documentation route against a running built site.
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const here = path.dirname(fileURLToPath(import.meta.url))
const contentRoot = path.resolve(here, "../content/docs")
const origin = (process.argv[2] ?? "http://127.0.0.1:4001").replace(/\/+$/, "")

function walk(dir) {
  const out = []
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) out.push(...walk(full))
    else if (entry.name.endsWith(".mdx")) out.push(full)
  }
  return out
}

// content/docs/index.mdx            -> /
// content/docs/foo.mdx              -> /foo
// content/docs/a/index.mdx          -> /a
// content/docs/a/b.mdx              -> /a/b
function slugFor(file) {
  let rel = path.relative(contentRoot, file).split(path.sep).join("/")
  rel = rel.replace(/\.mdx$/, "").replace(/(^|\/)index$/, "")
  return rel.replace(/\/$/, "")
}

const slugs = [...new Set(walk(contentRoot).map(slugFor))].sort()
const routes = []
for (const slug of slugs) {
  routes.push(slug ? `/${slug}` : "/")
}

const failures = []
let checked = 0
const concurrency = 8

async function check(route) {
  try {
    const response = await fetch(`${origin}${route}`, { redirect: "manual" })
    checked += 1
    if (response.status !== 200) failures.push(`${response.status}  ${route}`)
  } catch (error) {
    checked += 1
    failures.push(`ERR   ${route}  (${error.message})`)
  }
}

const queue = [...routes]
await Promise.all(
  Array.from({ length: concurrency }, async () => {
    while (queue.length > 0) await check(queue.shift())
  }),
)

if (failures.length > 0) {
  console.error(`\n${failures.length} of ${checked} routes did not return 200:`)
  for (const line of failures.sort()) console.error(`  ${line}`)
  process.exit(1)
}
console.log(`${checked} routes returned 200 at ${origin} (${slugs.length} pages).`)