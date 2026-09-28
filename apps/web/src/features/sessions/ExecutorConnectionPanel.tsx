import type { ExecutorCredentialList } from "@agents-core-web/agents-client";
import { useId } from "react";
import { useTranslation } from "react-i18next";

import { HelpTip, RefreshButton, StatusDot } from "../../components/console-ui";
import { CopyableId } from "../../components/list-ui";
import { formatDateTime, shortId } from "../../lib/format";
import { boundExecutorCredential, executorConnectionState } from "./executor-connection";
import "./executor-connection.css";

export function ExecutorConnectionPanel({ read, stale, failed, refreshing, archived, busy, onRefresh, onRotate }: {
  read: ExecutorCredentialList | undefined;
  stale: boolean;
  failed: boolean;
  refreshing: boolean;
  archived: boolean;
  busy: boolean;
  onRefresh: () => void;
  onRotate: (keyId: string, reason: "active" | "revoked") => void;
}) {
  const { t, i18n } = useTranslation("sessions");
  const headingId = useId();
  const state = executorConnectionState(read, stale);
  const connection = read?.connection;
  const bound = boundExecutorCredential(read);
  return (
    <section className="executor-connection" aria-labelledby={headingId}>
      <div className="executor-connection-heading">
        <h3 id={headingId}>{t("executor.connection.title")}</h3>
        <HelpTip>{t("executor.connection.help")}</HelpTip>
        <StatusDot tone={state === "connected" ? "ok" : state === "revoked" || state === "disconnected" ? "warning" : "neutral"} label={t(`executor.connection.status.${state}`)} />
        <RefreshButton onClick={onRefresh} refreshing={refreshing} />
      </div>
      {stale && read ? <p className="executor-connection-warning" role="status">{t(failed ? "executor.connection.stale" : "executor.connection.refreshing")}</p> : null}
      {!read && failed ? <p className="executor-connection-warning" role="status">{t("executor.connection.failed")}</p> : null}
      <dl className="executor-connection-facts">
        <div>
          <dt>{t("executor.connection.boundKey")}</dt>
          <dd>
            {connection?.bound_key_id ? <>
              <CopyableId id={connection.bound_key_id} compact label={t("executor.copyKeyId")} />
              {bound && !archived ? <button type="button" className="text-action" disabled={busy || stale} onClick={() => onRotate(bound.key_id, bound.revoked_at ? "revoked" : "active")}>{t(bound.revoked_at ? "executor.connection.rotateRestore" : "executor.connection.rotateBound")}</button> : null}
              <HelpTip>{t(bound?.revoked_at ? "executor.connection.restore" : "executor.connection.bound", { id: shortId(connection.bound_key_id) })}</HelpTip>
            </> : t(connection?.status === "never_enrolled" ? "executor.connection.notBound" : "executor.connection.status.unknown")}
          </dd>
        </div>
        <div><dt>{t("executor.connection.lastSeen")}</dt><dd>{connection?.last_seen_at ? formatDateTime(Date.parse(connection.last_seen_at) / 1000, i18n.resolvedLanguage) : t(connection ? "executor.connection.noHeartbeat" : "executor.connection.status.unknown")}</dd></div>
      </dl>
    </section>
  );
}
