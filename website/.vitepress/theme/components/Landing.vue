<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { withBase } from 'vitepress'
import AsciiCanvas from './AsciiCanvas.vue'
import ComposeLab from './ComposeLab.vue'
import LogoWall from './LogoWall.vue'
import TypeTerminal, { type TermLine } from './TypeTerminal.vue'
import { copy, installCommand, repoUrl, type Lang } from '../landing-content'
import { data as docsGroups } from '../docs.data.mts'
import architectureImg from '../../../../docs/assets/architecture.png'
import componentsImg from '../../../../docs/assets/development-architecture.png'
import surfacesImg from '../../../../docs/assets/architecture-api-surfaces.png'
import overviewEn from '../../../../docs/assets/console-overview-en.webp'
import overviewZh from '../../../../docs/assets/console-overview-zh.webp'
import metricsEn from '../../../../docs/assets/console-agent-metrics-en.webp'
import metricsZh from '../../../../docs/assets/console-agent-metrics-zh.webp'

const props = withDefaults(defineProps<{ lang?: Lang }>(), { lang: 'en' })
const t = computed(() => copy[props.lang])

const archImages: Record<string, string> = { ecosystem: architectureImg, protocols: componentsImg, surfaces: surfacesImg }
const consoleImages = computed(() =>
  props.lang === 'zh' ? { overview: overviewZh, metrics: metricsZh } : { overview: overviewEn, metrics: metricsEn },
)
const archTab = ref('ecosystem')
const consoleTab = ref<'overview' | 'metrics'>('overview')
const archAlt = computed(() => t.value.architecture.tabs.find((tab) => tab.id === archTab.value)!.alt)
const consoleAlt = computed(() => t.value.observe.tabs.find((tab) => tab.id === consoleTab.value)!.alt)

const localizedDocs = computed(() => docsGroups.map(group => ({ ...group, group: props.lang === 'zh' ? group.zhGroup : group.group, pages: group.pages.map(page => props.lang === 'zh' ? page.zh : page) })))
function localLink(path: string) { return withBase(props.lang === 'zh' ? `/zh${path}` : path) }

const pageCount = docsGroups.reduce((n, g) => n + g.pages.length, 0)

const copied = ref(false)
async function copyInstall() {
  try {
    await navigator.clipboard.writeText(installCommand)
    copied.value = true
    setTimeout(() => (copied.value = false), 1600)
  } catch {
    copied.value = false
  }
}

const termLines = computed<TermLine[]>(() => [
  { kind: 'comment', text: props.lang === 'zh' ? '# 1. 安装 Core 和 Web' : '# 1. install Core and Web' },
  { kind: 'cmd', text: installCommand },
  { kind: 'comment', text: props.lang === 'zh' ? '# 2. 在 Web 中设置默认模型，创建 Project API key' : '# 2. set a default model in Web, issue a Project API key' },
  { kind: 'cmd', text: 'export OPENAI_BASE_URL=https://core.example/v1' },
  { kind: 'cmd', text: 'read -rs OPENAI_API_KEY && export OPENAI_API_KEY' },
  { kind: 'comment', text: props.lang === 'zh' ? '# 3. 用官方 SDK 运行第一个 Session' : '# 3. run your first Session with the official SDK' },
  { kind: 'cmd', text: 'pip install openai==3.13.0 && python session.py' },
])

// Scrambles the hero title in from random glyphs.
const GLYPHS = '01<>/\\{}[]#%&@$=+*'
const titleLines = ref<string[]>([...t.value.hero.title])
let scrambleTimer: ReturnType<typeof setInterval> | undefined
function scramble() {
  const finals = t.value.hero.title
  let tick = 0
  scrambleTimer = setInterval(() => {
    tick++
    titleLines.value = finals.map((line, li) =>
      line
        .split('')
        .map((ch, i) => (ch === ' ' || tick > i * 1.4 + li * 6 + 4 ? ch : GLYPHS[(Math.random() * GLYPHS.length) | 0]))
        .join(''),
    )
    if (tick > finals.join('').length * 1.4 + 16) {
      clearInterval(scrambleTimer)
      titleLines.value = [...finals]
    }
  }, 40)
}

let revealObserver: IntersectionObserver | undefined
const root = ref<HTMLElement>()
onMounted(() => {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (!reduced) scramble()
  if (reduced) return
  // Content stays visible without JavaScript; hiding for the reveal starts here,
  // after anything already on screen is marked as shown.
  const targets = root.value!.querySelectorAll('[data-reveal]')
  targets.forEach((el) => {
    if (el.getBoundingClientRect().top < window.innerHeight) el.classList.add('in')
  })
  root.value!.classList.add('animate')
  revealObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue
        entry.target.classList.add('in')
        revealObserver?.unobserve(entry.target)
      }
    },
    { rootMargin: '0px 0px -8% 0px' },
  )
  targets.forEach((el) => {
    if (!el.classList.contains('in')) revealObserver!.observe(el)
  })
})
onBeforeUnmount(() => {
  clearInterval(scrambleTimer)
  revealObserver?.disconnect()
})
</script>

<template>
  <div ref="root" class="landing" :lang="lang === 'zh' ? 'zh-CN' : 'en'">
    <div class="bg-grid" aria-hidden="true" />

    <!-- Hero -->
    <section class="hero">
      <div class="wrap hero-grid">
        <div class="hero-copy">
          <p class="eyebrow"><span class="led" aria-hidden="true" />{{ t.hero.eyebrow }}</p>
          <h1 class="hero-title" :aria-label="t.hero.title.join(' ')">
            <span aria-hidden="true">{{ titleLines[0] }}</span>
            <span aria-hidden="true" class="accent">{{ titleLines[1] }}<span class="cursor">_</span></span>
          </h1>
          <p class="lede">{{ t.hero.lede }}</p>
          <div class="ctas">
            <a class="btn primary" :href="localLink('/docs/getting-started/quickstart')">{{ t.hero.primary }} <span aria-hidden="true">→</span></a>
            <a class="btn ghost" :href="localLink('/docs/getting-started/')">{{ t.hero.secondary }}</a>
            <a class="btn ghost icon" :href="repoUrl" target="_blank" rel="noreferrer" aria-label="GitHub">
              <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path fill="currentColor" d="M12 .5a11.5 11.5 0 0 0-3.64 22.41c.58.1.79-.25.79-.56v-2c-3.2.7-3.88-1.37-3.88-1.37-.53-1.33-1.28-1.69-1.28-1.69-1.05-.71.08-.7.08-.7 1.16.08 1.77 1.19 1.77 1.19 1.03 1.77 2.71 1.26 3.37.96.1-.75.4-1.26.73-1.55-2.55-.29-5.24-1.28-5.24-5.68 0-1.25.45-2.28 1.19-3.08-.12-.29-.52-1.46.11-3.05 0 0 .97-.31 3.17 1.18a11 11 0 0 1 5.77 0c2.2-1.49 3.17-1.18 3.17-1.18.63 1.59.23 2.76.11 3.05.74.8 1.19 1.83 1.19 3.08 0 4.41-2.69 5.38-5.25 5.67.41.36.78 1.06.78 2.14v3.17c0 .31.21.67.8.56A11.5 11.5 0 0 0 12 .5Z" /></svg>
              GitHub
            </a>
          </div>
          <div class="install">
            <code><span class="prompt">$</span> {{ installCommand }}</code>
            <button type="button" class="copy" @click="copyInstall">{{ copied ? t.hero.copied : t.hero.copy }}</button>
          </div>
          <p class="install-note">{{ t.hero.installLabel }}</p>
        </div>

        <div class="hero-art">
          <div class="corner tl" aria-hidden="true" /><div class="corner tr" aria-hidden="true" /><div class="corner bl" aria-hidden="true" /><div class="corner br" aria-hidden="true" />
          <AsciiCanvas kind="logo" :cell="12" :fill="0.68" :shift-y="-0.08" label="OpenAgentCore" />
          <dl class="hud">
            <div v-for="[k, v] in t.hero.hud" :key="k"><dt>{{ k }}</dt><dd>{{ v }}</dd></div>
          </dl>
        </div>
      </div>

      <div class="ticker" aria-hidden="true">
        <div class="ticker-track">
          <span v-for="n in 2" :key="n" class="ticker-run">
            <span v-for="item in t.ticker" :key="item + n">{{ item }}<b>/</b></span>
          </span>
        </div>
      </div>
    </section>

    <!-- 01 Problem -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.problem.index }}</span><span class="kicker">{{ t.problem.kicker }}</span></header>
        <div class="split">
          <div data-reveal>
            <h2>{{ t.problem.title }}</h2>
            <p class="sec-lede">{{ t.problem.lede }}</p>
          </div>
          <ol class="qa" data-reveal>
            <li v-for="(item, i) in t.problem.questions" :key="i" :style="{ '--i': i }">
              <p class="q"><span class="sigil">?</span>{{ item.q }}</p>
              <p class="a"><span class="sigil">→</span>{{ item.a }}</p>
            </li>
          </ol>
        </div>
        <p class="punch" data-reveal>{{ t.problem.punch }}</p>
      </div>
    </section>

    <!-- 02 Compose -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.compose.index }}</span><span class="kicker">{{ t.compose.kicker }}</span></header>
        <h2 data-reveal>{{ t.compose.title }}</h2>
        <p class="sec-lede" data-reveal>{{ t.compose.lede }}</p>
        <div data-reveal><ComposeLab :t="t.compose" :lang="lang" /></div>
        <ul class="pillars">
          <li v-for="(p, i) in t.pillars" :key="p.tag" data-reveal :style="{ '--i': i }">
            <span class="tag">[{{ p.tag }}]</span>
            <h3>{{ p.title }}</h3>
            <p>{{ p.body }}</p>
          </li>
        </ul>
      </div>
    </section>

    <!-- 03 Architecture -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.architecture.index }}</span><span class="kicker">{{ t.architecture.kicker }}</span></header>
        <h2 data-reveal>{{ t.architecture.title }}</h2>
        <p class="sec-lede" data-reveal>{{ t.architecture.lede }}</p>
        <div class="figure" data-reveal>
          <div class="tabs" role="tablist">
            <button
              v-for="tab in t.architecture.tabs"
              :key="tab.id"
              type="button"
              role="tab"
              :aria-selected="archTab === tab.id"
              :class="{ on: archTab === tab.id }"
              @click="archTab = tab.id"
            >{{ tab.label }}</button>
          </div>
          <a class="plate" :href="archImages[archTab]" target="_blank" rel="noreferrer" :title="lang === 'zh' ? '查看原图' : 'Open full size'">
            <img :key="archTab" :src="archImages[archTab]" :alt="archAlt" loading="lazy" decoding="async" />
          </a>
        </div>
        <div class="boundaries" data-reveal>
          <p class="mono-label">{{ t.architecture.boundariesTitle }}</p>
          <a v-for="(b, i) in t.architecture.boundaries" :key="b.link" class="boundary" :href="localLink(b.link)" :style="{ '--i': i }">
            <span class="b-from">{{ b.from }}</span>
            <span class="b-wire" aria-hidden="true"><i /></span>
            <span class="b-to">{{ b.to }}</span>
            <span class="b-doc">{{ b.doc }} <span aria-hidden="true">↗</span></span>
          </a>
          <a class="more" :href="localLink('/docs/architecture')">{{ t.architecture.more }} →</a>
        </div>
      </div>
    </section>

    <!-- 04 Session semantics -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.session.index }}</span><span class="kicker">{{ t.session.kicker }}</span></header>
        <h2 data-reveal>{{ t.session.title }}</h2>
        <p class="sec-lede" data-reveal>{{ t.session.lede }}</p>
        <ul class="rules">
          <li v-for="(rule, i) in t.session.rules" :key="i" data-reveal :style="{ '--i': i }">
            <pre class="art" aria-hidden="true">{{ rule.art }}</pre>
            <h3>{{ rule.title }}</h3>
            <p>{{ rule.body }}</p>
          </li>
        </ul>
        <p class="note" data-reveal><span class="sigil">!</span>{{ t.session.honest }}</p>
      </div>
    </section>

    <!-- 05 Observe -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.observe.index }}</span><span class="kicker">{{ t.observe.kicker }}</span></header>
        <div class="split narrow-right">
          <div data-reveal>
            <h2>{{ t.observe.title }}</h2>
            <p class="sec-lede">{{ t.observe.lede }}</p>
            <div class="tabs vertical" role="tablist">
              <button
                v-for="tab in t.observe.tabs"
                :key="tab.id"
                type="button"
                role="tab"
                :aria-selected="consoleTab === tab.id"
                :class="{ on: consoleTab === tab.id }"
                @click="consoleTab = tab.id"
              >{{ tab.label }}</button>
            </div>
            <p class="caption">{{ t.observe.caption }}</p>
          </div>
          <div class="window" data-reveal>
            <div class="win-bar"><span class="dots"><i /><i /><i /></span><span>web · {{ consoleTab }}</span></div>
            <img :key="consoleTab + lang" :src="consoleImages[consoleTab]" :alt="consoleAlt" loading="lazy" decoding="async" />
          </div>
        </div>
      </div>
    </section>

    <!-- 06 Compare -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.compare.index }}</span><span class="kicker">{{ t.compare.kicker }}</span></header>
        <h2 data-reveal>{{ t.compare.title }}</h2>
        <div class="table-wrap" data-reveal>
          <table class="compare">
            <thead>
              <tr><th v-for="h in t.compare.head" :key="h" scope="col">{{ h }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in t.compare.rows" :key="i" :class="{ ours: i === t.compare.rows.length - 1 }">
                <th scope="row">{{ row[0] }}</th>
                <td>{{ row[1] }}</td>
                <td>{{ row[2] }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <!-- 07 Trade-offs -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.tradeoffs.index }}</span><span class="kicker">{{ t.tradeoffs.kicker }}</span></header>
        <p class="motto" data-reveal><span>{{ t.tradeoffs.motto[0] }}</span><span class="accent">{{ t.tradeoffs.motto[1] }}</span></p>
        <h2 class="sr-only">{{ t.tradeoffs.title }}</h2>
        <ol class="tradeoffs">
          <li v-for="(item, i) in t.tradeoffs.items" :key="i" data-reveal :style="{ '--i': i }">
            <span class="num">0{{ i + 1 }}</span>
            <h3>{{ item.title }}</h3>
            <p>{{ item.body }}</p>
          </li>
        </ol>
      </div>
    </section>

    <!-- 08 Get started -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.start.index }}</span><span class="kicker">{{ t.start.kicker }}</span></header>
        <div class="split">
          <div data-reveal>
            <h2>{{ t.start.title }}</h2>
            <p class="sec-lede">{{ t.start.lede }}</p>
            <ol class="steps">
              <li v-for="(step, i) in t.start.steps" :key="i">
                <span class="step-n">[{{ i + 1 }}]</span>
                <div><h3>{{ step.title }}</h3><p>{{ step.body }}</p></div>
              </li>
            </ol>
            <div class="ctas">
              <a class="btn primary" :href="localLink('/docs/getting-started/install')">{{ t.start.cta }} →</a>
              <a class="btn ghost" :href="localLink('/docs/getting-started/quickstart')">{{ t.start.trial }}</a>
            </div>
          </div>
          <div data-reveal>
            <TypeTerminal :lines="termLines" title="~/openagentcore — zsh" />
          </div>
        </div>
      </div>
    </section>

    <!-- 09 Roadmap -->
    <section class="sec">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.roadmap.index }}</span><span class="kicker">{{ t.roadmap.kicker }}</span></header>
        <h2 data-reveal>{{ t.roadmap.title }}</h2>
        <p class="sec-lede" data-reveal>{{ t.roadmap.lede }}</p>
        <ol class="roadmap">
          <li v-for="(track, i) in t.roadmap.tracks" :key="track.layer" data-reveal :style="{ '--i': i }">
            <span class="layer">{{ track.layer }}</span>
            <h3>{{ track.title }}</h3>
            <p>{{ track.body }}</p>
          </li>
        </ol>
      </div>
    </section>

    <!-- 10 Docs -->
    <section class="sec docs-section">
      <div class="wrap">
        <header class="sec-head" data-reveal><span class="idx">// {{ t.docs.index }}</span><span class="kicker">{{ t.docs.kicker }}</span></header>
        <h2 data-reveal>{{ t.docs.title }}</h2>
        <p class="sec-lede" data-reveal>{{ t.docs.lede(pageCount) }}</p>
        <div class="docs-grid">
          <section v-for="(group, gi) in localizedDocs" :key="group.group" class="docs-group" data-reveal :style="{ '--i': gi }">
            <h3><span class="dir">{{ String(gi + 1).padStart(2, '0') }}/</span>{{ group.group }}</h3>
            <ul>
              <li v-for="page in group.pages" :key="page.link">
                <a :href="withBase(page.link)">
                  <span class="doc-title">{{ page.title }}</span>
                  <span v-if="page.summary" class="doc-summary">{{ page.summary }}</span>
                </a>
              </li>
            </ul>
          </section>
        </div>
      </div>
    </section>

    <LogoWall :lang="lang" />

    <!-- Outro -->
    <section class="outro">
      <div class="outro-art">
        <AsciiCanvas kind="text" text="ONE CORE.&#10;MANY AGENTS." :cell="11" ramp=" .:░▒▓█" :noise="0.02" :fill="0.82" label="One core. Many agents." />
      </div>
      <div class="wrap outro-copy">
        <p>{{ t.outro.line }}</p>
        <div class="ctas center">
          <a class="btn primary" :href="localLink('/docs/getting-started/')">{{ t.outro.primary }} →</a>
          <a class="btn ghost" :href="repoUrl" target="_blank" rel="noreferrer">★ {{ t.outro.secondary }}</a>
        </div>
      </div>
    </section>
  </div>
</template>

<style>
/* ---------- palette ---------- */
.landing {
  --l-bg: #f3efe3;
  --l-panel: rgba(255, 255, 255, 0.72);
  --l-hover: rgba(21, 128, 61, 0.06);
  --l-text: #0b1510;
  --l-text-2: #2b3a31;
  --l-muted: #5d6b62;
  --l-faint: #7c8a81;
  --l-line: rgba(11, 21, 16, 0.12);
  --l-line-strong: rgba(11, 21, 16, 0.22);
  --l-accent: #15803d;
  --l-accent-soft: rgba(21, 128, 61, 0.09);
  --l-danger: #dc2626;
  --l-danger-soft: rgba(220, 38, 38, 0.08);
  --l-glow: rgba(21, 128, 61, 0.35);
  --l-grid: rgba(11, 21, 16, 0.05);
  --ascii-dim: #c4d1c7;
  --ascii-mid: #3f8a5c;
  --ascii-hot: #15803d;
  --ascii-white: #04210f;
  /* Code and terminals stay dark in both themes. */
  --l-code-bg: #0b100e;
  --l-code-text: #d7e5dc;
  --l-kw: #f472b6;
  --l-str: #a3e6b8;
  --l-fn: #7dd3fc;
  --l-mono: 'Geist Mono Variable', ui-monospace, SFMono-Regular, Menlo, monospace;
  --l-display: 'Space Grotesk Variable', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  --l-sans: 'Inter Variable', Inter, system-ui, -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;

  position: relative;
  overflow: hidden;
  background: var(--l-bg);
  color: var(--l-text);
  font-family: var(--l-sans);
}
.dark .landing {
  --l-bg: #111712;
  --l-panel: rgba(18, 24, 21, 0.72);
  --l-hover: rgba(74, 222, 128, 0.06);
  --l-text: #e8f1eb;
  --l-text-2: #b7c5bc;
  --l-muted: #7f8f85;
  --l-faint: #56645b;
  --l-line: rgba(232, 241, 235, 0.09);
  --l-line-strong: rgba(232, 241, 235, 0.18);
  --l-accent: #4ade80;
  --l-accent-soft: rgba(74, 222, 128, 0.08);
  --l-danger: #fb7185;
  --l-danger-soft: rgba(251, 113, 133, 0.08);
  --l-glow: rgba(74, 222, 128, 0.35);
  --l-grid: rgba(232, 241, 235, 0.035);
  --ascii-dim: #1d3326;
  --ascii-mid: #2f8a52;
  --ascii-hot: #4ade80;
  --ascii-white: #eafff1;
}
</style>

<style scoped>
.landing :deep(a) {
  text-decoration: none;
}
.wrap {
  position: relative;
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 28px;
}
.bg-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: linear-gradient(var(--l-grid) 1px, transparent 1px), linear-gradient(90deg, var(--l-grid) 1px, transparent 1px);
  background-size: 96px 96px;
  mask-image: linear-gradient(to bottom, #000 0, #000 70%, transparent);
}
.bg-grid::after {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(60% 40% at 75% 8%, var(--l-accent-soft), transparent 70%);
}
.accent {
  color: var(--l-accent);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

/* ---------- reveal ---------- */
.animate [data-reveal] {
  opacity: 0;
  transform: translateY(14px);
  transition: opacity 0.7s ease, transform 0.7s cubic-bezier(0.2, 0.7, 0.2, 1);
  transition-delay: calc(var(--i, 0) * 70ms);
}
.animate [data-reveal].in {
  opacity: 1;
  transform: none;
}

/* ---------- buttons ---------- */
.ctas {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 28px;
}
.ctas.center {
  justify-content: center;
}
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 42px;
  padding: 0 18px;
  border-radius: 2px;
  font: 500 14px/1 var(--l-mono);
  border: 1px solid var(--l-line-strong);
  color: var(--l-text);
  transition: transform 0.15s, background 0.15s, border-color 0.15s, box-shadow 0.15s;
}
.btn:hover {
  transform: translateY(-1px);
  border-color: var(--l-accent);
}
.btn.primary {
  background: var(--l-accent);
  border-color: var(--l-accent);
  color: var(--l-bg);
  box-shadow: 0 0 0 0 var(--l-glow);
}
.btn.primary:hover {
  box-shadow: 0 8px 30px -8px var(--l-glow);
}
.btn.ghost {
  background: var(--l-panel);
  backdrop-filter: blur(6px);
}

/* ---------- hero ---------- */
.hero {
  position: relative;
  padding: 56px 0 0;
}
.hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.9fr);
  gap: 32px;
  align-items: center;
  min-height: min(72vh, 660px);
}
.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 22px;
  padding: 6px 12px;
  border: 1px solid var(--l-line);
  border-radius: 999px;
  background: var(--l-panel);
  font: 12px/1 var(--l-mono);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--l-text-2);
}
.led {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--l-accent);
  box-shadow: 0 0 10px var(--l-accent);
  animation: pulse 2s ease-in-out infinite;
}
.hero-title {
  display: flex;
  flex-direction: column;
  margin: 0;
  font: 700 clamp(44px, 6vw, 78px) / 0.98 var(--l-display);
  letter-spacing: -0.045em;
  white-space: nowrap;
}
.cursor {
  color: var(--l-accent);
  animation: blink 1.1s steps(1) infinite;
}
.lede {
  max-width: 34em;
  margin: 26px 0 0;
  font-size: 18px;
  line-height: 1.6;
  color: var(--l-text-2);
}
.install {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  margin-top: 26px;
  padding: 6px 6px 6px 14px;
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-code-bg);
}
.install code {
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  white-space: nowrap;
  scrollbar-width: none;
  font: 13px/2.2 var(--l-mono);
  color: var(--l-code-text);
  background: none;
  padding: 0;
}
.install .prompt {
  color: #4ade80;
}
.copy {
  flex: none;
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid rgba(232, 241, 235, 0.16);
  font: 12px var(--l-mono);
  color: #b7c5bc;
}
.copy:hover {
  color: #4ade80;
  border-color: #4ade80;
}
.install-note {
  margin: 10px 0 0;
  font: 12px var(--l-mono);
  color: var(--l-muted);
}
.hero-art {
  position: relative;
  aspect-ratio: 1 / 1;
  width: 100%;
  max-width: 600px;
  justify-self: end;
}
.corner {
  position: absolute;
  width: 18px;
  height: 18px;
  border-color: var(--l-accent);
  border-style: solid;
  opacity: 0.7;
}
.corner.tl { top: 0; left: 0; border-width: 1px 0 0 1px; }
.corner.tr { top: 0; right: 0; border-width: 1px 1px 0 0; }
.corner.bl { bottom: 0; left: 0; border-width: 0 0 1px 1px; }
.corner.br { bottom: 0; right: 0; border-width: 0 1px 1px 0; }
.hud {
  position: absolute;
  left: 14px;
  bottom: 12px;
  margin: 0;
  font: 11px/1.7 var(--l-mono);
  color: var(--l-muted);
  pointer-events: none;
}
.hud div {
  display: flex;
  gap: 8px;
}
.hud dt {
  min-width: 6.5em;
  color: var(--l-accent);
}
.hud dt::after {
  content: ':';
}
.hud dd {
  margin: 0;
}

/* ticker */
.ticker {
  margin-top: 48px;
  border-block: 1px solid var(--l-line);
  overflow: hidden;
  background: var(--l-panel);
  mask-image: linear-gradient(90deg, transparent, #000 10%, #000 90%, transparent);
}
.ticker-track {
  display: flex;
  width: max-content;
  animation: marquee 48s linear infinite;
}
.ticker-run span {
  display: inline-block;
  padding: 14px 0 14px 22px;
  font: 13px var(--l-mono);
  color: var(--l-text-2);
  text-transform: lowercase;
}
.ticker-run b {
  margin-left: 22px;
  font-weight: 400;
  color: var(--l-accent);
}

/* ---------- sections ---------- */
.sec {
  position: relative;
  padding: 112px 0 0;
}
.sec-head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 20px;
  font: 12px/1 var(--l-mono);
  text-transform: uppercase;
  letter-spacing: 0.12em;
}
.sec-head::after {
  content: '';
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, var(--l-line-strong), transparent);
}
.idx {
  color: var(--l-accent);
}
.kicker {
  color: var(--l-muted);
}
h2 {
  max-width: 22em;
  margin: 0;
  font: 700 clamp(28px, 3.6vw, 44px) / 1.12 var(--l-sans);
  letter-spacing: -0.03em;
  border: 0;
  padding: 0;
}
.landing[lang='zh-CN'] h2 {
  letter-spacing: -0.01em;
}
h3 {
  margin: 0;
  font: 600 17px/1.35 var(--l-sans);
  letter-spacing: -0.01em;
}
.sec-lede {
  max-width: 44em;
  margin: 18px 0 36px;
  font-size: 17px;
  line-height: 1.65;
  color: var(--l-text-2);
}
.split {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr);
  gap: 56px;
  align-items: start;
}
.split.narrow-right {
  grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.5fr);
}
.mono-label {
  margin: 0 0 12px;
  font: 12px var(--l-mono);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--l-muted);
}
.sigil {
  display: inline-block;
  width: 1.6em;
  font-family: var(--l-mono);
  color: var(--l-accent);
}

/* 01 */
.qa {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-panel);
  overflow: hidden;
}
.qa li {
  padding: 16px 20px;
  border-bottom: 1px dashed var(--l-line);
  transition: background 0.2s;
}
.qa li:last-child {
  border-bottom: 0;
}
.qa li:hover {
  background: var(--l-hover);
}
.qa p {
  display: flex;
  margin: 0;
  line-height: 1.55;
}
.q {
  font-weight: 600;
  font-size: 15px;
}
.q .sigil {
  color: var(--l-danger);
}
.a {
  margin-top: 6px !important;
  font-size: 14px;
  color: var(--l-text-2);
}
.punch {
  max-width: 36em;
  margin: 56px 0 0;
  padding-left: 18px;
  border-left: 2px solid var(--l-accent);
  font: 500 clamp(18px, 2vw, 22px) / 1.55 var(--l-sans);
}

/* 02 */
.pillars {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin: 36px 0 0;
  padding: 0;
  list-style: none;
}
.pillars li {
  padding: 20px;
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-panel);
  transition: border-color 0.2s, transform 0.2s;
}
.pillars li:hover {
  border-color: var(--l-accent);
  transform: translateY(-2px);
}
.tag {
  display: block;
  margin-bottom: 12px;
  font: 12px var(--l-mono);
  color: var(--l-accent);
}
.pillars p,
.rules p,
.tradeoffs p,
.roadmap p {
  margin: 8px 0 0;
  font-size: 14.5px;
  line-height: 1.6;
  color: var(--l-text-2);
}

/* 03 */
.figure {
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-panel);
  overflow: hidden;
}
.tabs {
  display: flex;
  gap: 4px;
  padding: 8px;
  border-bottom: 1px solid var(--l-line);
  overflow-x: auto;
}
.tabs button {
  padding: 7px 14px;
  border-radius: 7px;
  font: 13px var(--l-mono);
  color: var(--l-muted);
  white-space: nowrap;
  transition: color 0.15s, background 0.15s;
}
.tabs button::before {
  content: '> ';
  opacity: 0;
}
.tabs button.on {
  color: var(--l-accent);
  background: var(--l-accent-soft);
}
.tabs button.on::before {
  opacity: 1;
}
.tabs button:hover {
  color: var(--l-text);
}
.tabs.vertical {
  flex-direction: column;
  align-items: flex-start;
  padding: 0;
  border: 0;
}
.plate {
  display: block;
  padding: 18px;
  background: #fff;
  cursor: zoom-in;
}
.plate img {
  display: block;
  width: 100%;
  height: auto;
  animation: fade 0.4s ease;
}
.boundaries {
  display: grid;
  gap: 0;
  margin-top: 28px;
}
.boundary {
  display: grid;
  grid-template-columns: 9em minmax(40px, 1fr) 9em minmax(0, 1.3fr);
  align-items: center;
  gap: 14px;
  padding: 12px 6px;
  border-bottom: 1px solid var(--l-line);
  font: 14px var(--l-mono);
  color: var(--l-text);
  transition: background 0.15s;
}
.boundary:hover {
  background: var(--l-hover);
}
.b-to {
  text-align: left;
}
.b-wire {
  position: relative;
  height: 1px;
  background: var(--l-line-strong);
}
.b-wire::after {
  content: '▶';
  position: absolute;
  right: -2px;
  top: 50%;
  transform: translateY(-52%);
  font-size: 9px;
  color: var(--l-line-strong);
}
.b-wire i {
  position: absolute;
  top: -1px;
  left: 0;
  width: 26px;
  height: 3px;
  border-radius: 2px;
  background: var(--l-accent);
  box-shadow: 0 0 10px var(--l-accent);
  animation: packet 2.6s linear infinite;
  animation-delay: calc(var(--i) * -0.43s);
}
.b-doc {
  justify-self: end;
  color: var(--l-muted);
  font-size: 13px;
}
.boundary:hover .b-doc {
  color: var(--l-accent);
}
.more {
  justify-self: start;
  margin-top: 18px;
  font: 14px var(--l-mono);
  color: var(--l-accent);
}

/* 04 */
.rules {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rules li {
  padding: 20px;
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-panel);
}
.art {
  margin: 0 0 18px;
  padding: 14px 16px;
  border-radius: 2px;
  background: var(--l-code-bg);
  color: #4ade80;
  font: 12.5px/1.7 var(--l-mono);
  overflow-x: auto;
  white-space: pre;
}
.note {
  display: flex;
  margin: 26px 0 0;
  font: 14px/1.6 var(--l-mono);
  color: var(--l-text-2);
}

/* 05 */
.caption {
  margin: 22px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--l-muted);
}
.window {
  border: 1px solid var(--l-line);
  border-radius: 3px;
  overflow: hidden;
  background: var(--l-panel);
  box-shadow: 0 40px 100px -50px var(--l-glow);
}
.window img {
  display: block;
  width: 100%;
  height: auto;
  animation: fade 0.4s ease;
}
.win-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--l-line);
  font: 12px var(--l-mono);
  color: var(--l-muted);
}
.dots {
  display: flex;
  gap: 6px;
}
.dots i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.dots i:nth-child(1) { background: #ff5f57; }
.dots i:nth-child(2) { background: #febc2e; }
.dots i:nth-child(3) { background: #28c840; }

/* 06 */
.table-wrap {
  margin-top: 36px;
  overflow-x: auto;
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-panel);
}
.compare {
  display: table;
  width: 100%;
  margin: 0;
  border-collapse: collapse;
  font-size: 14.5px;
}
.compare th,
.compare td {
  padding: 16px 20px;
  border: 0;
  border-bottom: 1px solid var(--l-line);
  text-align: left;
  vertical-align: top;
  line-height: 1.55;
  background: none;
}
.compare thead th {
  font: 12px var(--l-mono);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--l-muted);
}
.compare tbody th {
  font: 600 14px var(--l-mono);
  white-space: nowrap;
}
.compare td {
  color: var(--l-text-2);
}
.compare tr {
  background: none;
  border: 0;
}
.compare tr.ours th,
.compare tr.ours td {
  border-bottom: 0;
  background: var(--l-accent-soft);
}
.compare tr.ours th {
  color: var(--l-accent);
}

/* 07 */
.motto {
  display: flex;
  flex-direction: column;
  margin: 8px 0 44px;
  font: 700 clamp(28px, 4.4vw, 56px) / 1.1 var(--l-mono);
  letter-spacing: -0.035em;
}
.tradeoffs {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 28px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.tradeoffs li {
  padding-top: 18px;
  border-top: 1px solid var(--l-line-strong);
}
.num {
  display: block;
  margin-bottom: 10px;
  font: 12px var(--l-mono);
  color: var(--l-accent);
}

/* 08 */
.steps {
  display: grid;
  gap: 16px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.steps li {
  display: flex;
  gap: 14px;
}
.step-n {
  font: 14px/1.4 var(--l-mono);
  color: var(--l-accent);
}
.steps p {
  margin: 4px 0 0;
  font-size: 14.5px;
  color: var(--l-text-2);
}

/* 09 */
.roadmap {
  position: relative;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.roadmap li {
  position: relative;
  padding: 20px;
  border: 1px dashed var(--l-line-strong);
  border-radius: 3px;
}
.layer {
  display: inline-block;
  margin-bottom: 12px;
  padding: 3px 8px;
  border-radius: 4px;
  background: var(--l-accent-soft);
  font: 12px var(--l-mono);
  color: var(--l-accent);
}

/* 10 */
.docs-grid {
  display: grid;
  align-items: start;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}
.docs-group {
  padding: 18px;
  border: 1px solid var(--l-line);
  border-radius: 3px;
  background: var(--l-panel);
}
.docs-group h3 {
  margin-bottom: 12px;
  font: 600 14px var(--l-mono);
}
.dir {
  color: var(--l-accent);
  margin-right: 4px;
}
.docs-group ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
.docs-group li + li {
  border-top: 1px dashed var(--l-line);
}
.docs-group a {
  display: block;
  padding: 10px 0;
  color: var(--l-text);
}
.doc-title {
  display: block;
  font-size: 14px;
  font-weight: 500;
  transition: color 0.15s;
}
.doc-title::before {
  content: '› ';
  color: var(--l-faint);
  font-family: var(--l-mono);
}
.docs-group a:hover .doc-title {
  color: var(--l-accent);
}
.doc-summary {
  display: -webkit-box;
  margin-top: 3px;
  overflow: hidden;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--l-muted);
}

/* outro */
.outro {
  position: relative;
  margin-top: 24px;
  padding-bottom: 96px;
  border-top: 1px solid var(--l-line);
}
.outro-art {
  height: clamp(200px, 30vw, 380px);
  max-width: 1280px;
  margin: 0 auto;
}
.outro-copy {
  text-align: center;
}
.outro-copy p {
  margin: 8px auto 0;
  max-width: 36em;
  font-size: 18px;
  line-height: 1.6;
  color: var(--l-text-2);
}

/* ---------- motion ---------- */
@keyframes blink {
  50% { opacity: 0; }
}
@keyframes pulse {
  50% { opacity: 0.35; }
}
@keyframes marquee {
  to { transform: translateX(-50%); }
}
@keyframes packet {
  from { left: 0; opacity: 0; }
  10% { opacity: 1; }
  90% { opacity: 1; }
  to { left: calc(100% - 26px); opacity: 0; }
}
@keyframes fade {
  from { opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .ticker-track, .b-wire i, .led, .cursor { animation: none; }
}

/* ---------- responsive ---------- */
@media (max-width: 1080px) {
  .pillars, .roadmap, .docs-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 900px) {
  .hero { padding-top: 24px; }
  .hero-grid { grid-template-columns: minmax(0, 1fr); gap: 8px; min-height: 0; }
  .hero-art { order: -1; width: min(100%, 340px); justify-self: center; }
  .hud { display: none; }
  .split, .split.narrow-right { grid-template-columns: minmax(0, 1fr); gap: 32px; }
  .tradeoffs { grid-template-columns: minmax(0, 1fr); }
  .rules { grid-template-columns: minmax(0, 1fr); }
  .boundary { grid-template-columns: 7.5em minmax(24px, 1fr) 7.5em; }
  .b-doc { grid-column: 1 / -1; justify-self: start; }
  .sec { padding-top: 84px; }
}
@media (max-width: 560px) {
  .wrap { padding: 0 18px; }
  .pillars, .roadmap, .docs-grid { grid-template-columns: minmax(0, 1fr); }
  .eyebrow { flex-wrap: wrap; border-radius: 3px; line-height: 1.5; }
  .hero-title { font-size: clamp(38px, 12vw, 56px); }
  .lede { font-size: 16px; }
  .compare tbody th { white-space: normal; }
}

/* Editorial typography and ruled sections retain the terminal demonstrations. */
.hero-title .accent { font-style: italic; }
.cursor { font-style: normal; }
.eyebrow { border-radius: 0; background: transparent; border: 0; border-left: 3px solid var(--l-accent); padding-left: 12px; }
.sec { border-top: 1px solid var(--l-line-strong); padding-bottom: 80px; }
.sec.docs-section { padding-bottom: 32px; }
.sec-head { align-items: center; }
.idx { background: var(--l-accent); color: var(--l-bg); padding: 7px 10px; }
h2, .motto { font-family: var(--l-display); letter-spacing: -0.045em; }
h3 { font-family: var(--l-display); }
.hero-art { border: 1px solid var(--l-line-strong); background: var(--l-panel); transform: rotate(1deg); }
.ticker { background: var(--l-text); color: var(--l-bg); border-top: 3px solid var(--l-accent); }
.ticker-run span { color: inherit; }
.btn.primary { box-shadow: 3px 3px 0 var(--l-text); }
.btn.primary:hover { box-shadow: 5px 5px 0 var(--l-text); }
.btn:focus-visible, .copy:focus-visible { outline: 2px solid var(--l-accent); outline-offset: 5px; }
@media (max-width: 640px) {
  .hero-art { transform: none; }
  .hero-title { letter-spacing: -0.05em; }
}
</style>
