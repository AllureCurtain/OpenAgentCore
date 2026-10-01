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

The landing page lives in `.vitepress/theme/`. `landing-content.ts` holds its English and Chinese copy; every claim there must be backed by a page in `docs/` or `contracts/`, and its harness protocols follow [Model execution](../contracts/agents-api/model-execution.md).

## Publish

`make check-website` builds and tests the site. `core-check` selects the reusable `.github/workflows/website.yml` for website, published documentation and dependency changes. That workflow builds and tests once; on main pushes it deploys the same artifact to GitHub Pages. The [CI planner](../docs/maintainers.md#continuous-integration) owns the input map. A repository administrator enables Pages once: **Settings → Pages → Source: GitHub Actions**. The build reads the Pages base path, so the site works both at `https://<owner>.github.io/<repository>/` and on a custom domain set under **Settings → Pages → Custom domain**.
