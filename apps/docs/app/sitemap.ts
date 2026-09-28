import type { MetadataRoute } from "next"
import { absoluteDocsUrl } from "@/lib/site"
import { source } from "@/lib/source"

export default function sitemap(): MetadataRoute.Sitemap {
  return source.getPages().map(page => ({ url: absoluteDocsUrl(page.url) }))
}
