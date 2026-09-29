// Set the published origin before building crawler metadata.
const siteOrigin = (process.env.DOCS_SITE_ORIGIN ?? "http://127.0.0.1:4001").replace(/\/+$/, "")

export function absoluteDocsUrl(relative: string): string {
  return `${siteOrigin}${relative === "/" ? "" : relative}`
}
