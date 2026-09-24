import { common as enCommon } from "./locales/en/common";
import { navigation as enNavigation } from "./locales/en/navigation";
import { connection as enConnection } from "./locales/en/connection";
import { pages as enPages } from "./locales/en/pages";
import { agents as enAgents } from "./locales/en/agents";
import { templates as enTemplates } from "./locales/en/templates";
import { vaults as enVaults } from "./locales/en/vaults";
import { system as enSystem } from "./locales/en/system";
import { dashboard as enDashboard } from "./locales/en/dashboard";
import { app as enApp } from "./locales/en/app";
import { sessions as enSessions } from "./locales/en/sessions";
import { common as zhCNCommon } from "./locales/zh-CN/common";
import { navigation as zhCNNavigation } from "./locales/zh-CN/navigation";
import { connection as zhCNConnection } from "./locales/zh-CN/connection";
import { pages as zhCNPages } from "./locales/zh-CN/pages";
import { agents as zhCNAgents } from "./locales/zh-CN/agents";
import { templates as zhCNTemplates } from "./locales/zh-CN/templates";
import { vaults as zhCNVaults } from "./locales/zh-CN/vaults";
import { system as zhCNSystem } from "./locales/zh-CN/system";
import { dashboard as zhCNDashboard } from "./locales/zh-CN/dashboard";
import { app as zhCNApp } from "./locales/zh-CN/app";
import { sessions as zhCNSessions } from "./locales/zh-CN/sessions";
import { apiKeyChinese } from "../lib/api-key-strings";
import { consoleAuthChinese } from "../lib/console-auth-strings";
import { firstRunChinese } from "../lib/first-run-strings";
import { chinese as zhCNSandbox } from "../lib/locale-strings";

const enSandbox = Object.fromEntries(
  Object.keys(zhCNSandbox).map((key) => [key, key]),
) as { [K in keyof typeof zhCNSandbox]: K };

const zhCNFirstRun = {
  ...consoleAuthChinese,
  ...apiKeyChinese,
  ...firstRunChinese,
} as const;

const enFirstRun = Object.fromEntries(
  Object.keys(zhCNFirstRun).map((key) => [key, key]),
) as { [K in keyof typeof zhCNFirstRun]: K };

export const defaultNamespace = "common";

export const resources = {
  en: {
    common: enCommon,
    navigation: enNavigation,
    connection: enConnection,
    pages: enPages,
    agents: enAgents,
    templates: enTemplates,
    vaults: enVaults,
    system: enSystem,
    dashboard: enDashboard,
    app: enApp,
    sessions: enSessions,
    sandbox: enSandbox,
    firstRun: enFirstRun,
  },
  "zh-CN": {
    common: zhCNCommon,
    navigation: zhCNNavigation,
    connection: zhCNConnection,
    pages: zhCNPages,
    agents: zhCNAgents,
    templates: zhCNTemplates,
    vaults: zhCNVaults,
    system: zhCNSystem,
    dashboard: zhCNDashboard,
    app: zhCNApp,
    sessions: zhCNSessions,
    sandbox: zhCNSandbox,
    firstRun: zhCNFirstRun,
  },
} as const;
