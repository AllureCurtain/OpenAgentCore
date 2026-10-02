# OpenAgentCore website

The website is the landing page plus the published documentation, built with [VitePress](https://vitepress.dev) and deployed to GitHub Pages. It reads the repository's existing `docs/`, `contracts/` and `docs.json` in place: no page is copied, moved or rewritten.

## Run it

```sh
pnpm --dir website install --frozen-lockfile
pnpm --dir website dev       # http://127.0.0.1:4180
pnpm --dir website build     # website/.vitepress/dist
pnpm --dir website test      # after build: navigation and output checks
pnpm --dir website preview   # serve the build at http://127.0.0.1:4181
```

## How documentation reaches the site

- The content root is the repository root. `docs/**` and `contracts/**` keep their paths as URLs, so `docs/architecture.md` is `/docs/architecture`. `website/index.md` and `website/zh/index.md` are mapped onto `/` and `/zh/`.
- `docs.json` owns the page list and order. The sidebar, the landing page's documentation index and `/llms.txt` are generated from it at build time; each page's frontmatter `title` is its label. Every Markdown page under `docs/` and `contracts/` is built; listing it in `docs.json` adds it to the sidebar, the landing page and `llms.txt`.
- Each page's source Markdown is published next to its HTML (`/docs/architecture.md`), and the `README` paths declared in `docs.json` redirect to their section index.
- Relative links from a page to a file that is not a published page, such as `CONTRIBUTING.md` or `openapi.yaml`, are rewritten to GitHub at build time, so the Markdown works unchanged on GitHub and on the site.

The visual system combines warm paper surfaces, green accents, ruled sections and ASCII artwork. Landing headings use self-hosted Space Grotesk; body text uses Inter and code uses Geist Mono. Chinese text uses the system Chinese sans-serif stack. Both light and dark themes share the same hierarchy.

Site appearance and metadata are configured in `.vitepress/config.mts` and `.vitepress/theme/`.

The landing page lives in `.vitepress/theme/`. `landing-content.ts` holds its English and Chinese copy; every claim there must be backed by a page in `docs/` or `contracts/`, and its harness protocols follow [Model execution](../contracts/agents-api/model-execution.md).

## Publish

`make check-website` builds and tests the site. core-check runs it for pull requests that change the website, published documentation or relevant Node dependencies; `.github/workflows/website.yml` builds and deploys `main` to GitHub Pages. A repository administrator enables Pages once: **Settings → Pages → Source: GitHub Actions**.

The publishing step reads `html_url` directly from the GitHub Pages API and passes it to the build as `WEBSITE_URL`. VitePress derives the base path from that URL, supporting both `https://<owner>.github.io/<repository>/` and a custom domain set under **Settings → Pages → Custom domain**. An empty or invalid URL stops the build. Local builds omit `WEBSITE_URL` to serve from `/`.

Pull request checks build under the published `/OpenAgentCore/` path. Output tests verify that generated navigation, assets, redirects and `llms.txt` links use the configured path and that local HTML links resolve to generated files. To check a deployment path locally, run `WEBSITE_URL=https://example.com/OpenAgentCore/ make check-website`.
