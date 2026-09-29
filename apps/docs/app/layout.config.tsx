import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared"

export const baseOptions: BaseLayoutProps = {
  nav: {
    title: <><img src="/openagentcore.svg" alt="" width={24} height={24} className="dark:invert" /><span>OpenAgentCore Docs</span></>,
  },
  searchToggle: {
    enabled: false,
  },
}
