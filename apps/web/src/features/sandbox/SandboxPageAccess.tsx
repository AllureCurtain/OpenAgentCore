import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { SandboxConsoleConfig } from "./console-config";
import { sandboxConsoleConfigQuery } from "./sandbox-queries";

/** Both sandbox pages use the same console capability gate. */
export function SandboxPageAccess({ header, children }: { header: ReactNode; children: (config: SandboxConsoleConfig) => ReactNode }) {
  const { t } = useTranslation("sandbox");
  const { data: config, isPending: checking, isFetching, isError, refetch } = useQuery(sandboxConsoleConfigQuery);
  if (isError && config === undefined) return <>{header}<div className="console-page-body"><p role="alert">{t("The console configuration could not be read. Refresh to try again.")}</p><button type="button" className="button outline" disabled={isFetching} onClick={() => { void refetch(); }}>{t("Refresh sandbox state")}</button></div></>;
  if (checking) return <>{header}<div className="console-page-body"><p role="status">{t("Connecting to this console's Core…")}</p></div></>;
  if (!config?.sandbox_admin) return <>{header}<div className="console-page-body"><p role="alert">{t("Sandbox administration is not configured on this console.")}</p><button type="button" className="button outline" disabled={isFetching} onClick={() => { void refetch(); }}>{t("Refresh sandbox state")}</button></div></>;
  return children(config);
}
