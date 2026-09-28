#!/usr/bin/env node
// Require static output for every guide and keep API pages read-only.
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const here = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(here, "..")
const contentRoot = path.join(root, "content/docs")
const appRoot = path.join(root, ".next/server/app")

if (!fs.existsSync(appRoot)) {
  console.error(`No build output at ${path.relative(root, appRoot)}. Run \`pnpm build\` first.`)
  process.exit(1)
}

function walk(dir) {
  const out = []
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) out.push(...walk(full))
    else if (entry.name.endsWith(".mdx")) out.push(full)
  }
  return out
}

function slugFor(file) {
  let rel = path.relative(contentRoot, file).split(path.sep).join("/")
  rel = rel.replace(/\.mdx$/, "").replace(/(^|\/)index$/, "")
  return rel.replace(/\/$/, "")
}

const slugs = [...new Set(walk(contentRoot).map(slugFor))].sort()

const missing = []
for (const slug of slugs) {
    const file = slug
      ? path.join(appRoot, `${slug}.html`)
      : path.join(appRoot, "index.html")
    if (!fs.existsSync(file)) missing.push(path.relative(root, file))
    else if (slug.startsWith("api-reference/")) {
      const html = fs.readFileSync(file, "utf8")
      if (/<form(?:\s|>)/i.test(html) || /<input(?:\s|>)/i.test(html)) {
        throw new Error("API reference must not collect credentials or send requests: " + file)
      }
    }
}

const expected = slugs.length
if (missing.length > 0) {
  console.error(
    `\n${missing.length} of ${expected} routes were not prerendered. ` +
      `The docs route is rendering per request:`,
  )
  for (const file of missing.slice(0, 12)) console.error(`  ${file}`)
  if (missing.length > 12) console.error(`  … and ${missing.length - 12} more`)
  console.error(
    "\nLook for a request-scoped API (headers, cookies, searchParams, " +
      "`dynamic = \"force-dynamic\"`) in app/**.",
  )
  process.exit(1)
}
console.log(`${expected} documentation routes prerendered.`)
