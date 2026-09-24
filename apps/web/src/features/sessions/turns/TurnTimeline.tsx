import { Activity, Clock3 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  AgentTurn,
  SessionItem,
  TokenUsage,
  TurnStatus,
} from "@agents-core-web/agents-client";

import { StatusIcon, type StatusKind } from "../../../components/StatusIcon";

export type TurnTimelineLoadState = "idle" | "loading" | "ready" | "failed";

interface TurnTimelineProps {
  turns: AgentTurn[];
  items: SessionItem[];
  sessionUsage: TokenUsage | null;
  loadState: TurnTimelineLoadState;
  error?: string | null;
  nowSeconds?: number;
}

const usageMetrics = [
  ["input", ["input_tokens"]],
  ["output", ["output_tokens"]],
  ["total", ["total_tokens"]],
  ["cached", ["input_tokens_details", "cached_tokens"]],
  ["reasoning", ["output_tokens_details", "reasoning_tokens"]],
] as const;

function metric(value: unknown, path: readonly string[]): number | null {
  let current = value;
  for (const part of path) {
    if (current === null || typeof current !== "object" || Array.isArray(current)) return null;
    current = (current as Record<string, unknown>)[part];
  }
  return typeof current === "number" && Number.isSafeInteger(current) && current >= 0 ? current : null;
}

function UsageGrid({
  label,
  usage,
  accessibleLabel = label,
  landmark = false,
}: {
  label: string;
  usage: unknown;
  accessibleLabel?: string;
  landmark?: boolean;
}) {
  const { t, i18n } = useTranslation("sessions");
  const locale = i18n.resolvedLanguage || "en";
  const content = (
    <>
      <strong>{label}</strong>
      <dl>
        {usageMetrics.map(([name, path]) => {
          const value = metric(usage, path);
          return <div key={name}><dt>{t(`usage.${name}` as never)}</dt><dd>{value === null ? t("common.unknown") : value.toLocaleString(locale)}</dd></div>;
        })}
      </dl>
    </>
  );
  return landmark
    ? <section className="turn-usage" aria-label={accessibleLabel}>{content}</section>
    : <div className="turn-usage" role="group" aria-label={accessibleLabel}>{content}</div>;
}

function statusKind(status: TurnStatus): StatusKind {
  if (status === "in_progress" || status === "waiting") return "running";
  if (status === "failed") return "failed";
  if (status === "cancelled") return "cancelled";
  if (status === "completed") return "completed";
  return "queued";
}

function seconds(value: unknown): number | null {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0 ? value : null;
}

export function formatTurnTimestamp(value: unknown, locale?: string, unknownLabel = "Unknown"): string {
  const timestamp = seconds(value);
  if (timestamp === null) return unknownLabel;
  const date = new Date(timestamp * 1_000);
  if (Number.isNaN(date.getTime())) return unknownLabel;
  if (locale) return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "medium" }).format(date);
  return `${date.toISOString().slice(0, 19).replace("T", " ")} UTC`;
}

export function formatTurnElapsed(startedAt: unknown, endedAt: unknown): string {
  const start = seconds(startedAt);
  const end = seconds(endedAt);
  if (start === null || end === null || end < start) return "Unknown";
  const elapsed = end - start;
  const hours = Math.floor(elapsed / 3_600);
  const minutes = Math.floor(elapsed % 3_600 / 60);
  const remainder = elapsed % 60;
  if (hours) return `${hours}h ${String(minutes).padStart(2, "0")}m ${String(remainder).padStart(2, "0")}s`;
  if (minutes) return `${minutes}m ${String(remainder).padStart(2, "0")}s`;
  return `${remainder}s`;
}

function errorProjection(value: unknown): { code: string; message: string } | null {
  if (value === null || typeof value !== "object" || Array.isArray(value)) return null;
  const candidate = value as Record<string, unknown>;
  if (typeof candidate.code !== "string" || typeof candidate.message !== "string") return null;
  return { code: candidate.code || "unknown_error", message: candidate.message || "Unknown" };
}

function TurnCard({ turn, itemCount, now }: { turn: AgentTurn; itemCount: number; now: number }) {
  const { t, i18n } = useTranslation("sessions");
  const locale = i18n.resolvedLanguage || "en";
  const active = turn.status === "in_progress" || turn.status === "waiting";
  const elapsed = active
    ? formatTurnElapsed(turn.started_at, now)
    : formatTurnElapsed(turn.started_at, turn.completed_at);
  const error = errorProjection(turn.error);
  const headingId = `turn-${turn.id.replace(/[^a-zA-Z0-9_-]/g, "-")}-heading`;

  return (
    <article className={`turn-card turn-card-${turn.status}`} data-turn-id={turn.id} data-turn-status={turn.status} aria-labelledby={headingId}>
      <header className="turn-card-heading">
        <StatusIcon status={statusKind(turn.status)} title={t("turns.statusTitle", { status: t(`status.${turn.status}` as never) })} />
        <div>
          <strong id={headingId}>{t("turns.statusTurn", { status: t(`status.${turn.status}` as never) })}</strong>
          <code title={turn.id}>{turn.id}</code>
        </div>
        <span className="turn-item-count">{t("turns.linkedItems", { count: itemCount })}</span>
      </header>

      <dl className="turn-timing">
        <div><dt>{t("turns.started")}</dt><dd>{formatTurnTimestamp(turn.started_at, locale, t("common.unknown"))}</dd></div>
        <div><dt>{t("turns.completed")}</dt><dd>{formatTurnTimestamp(turn.completed_at, locale, t("common.unknown"))}</dd></div>
        <div>
          <dt>{active ? t("turns.runningElapsed") : t("turns.wallClock")}</dt>
          <dd aria-label={active ? t("turns.runningElapsedValue", { elapsed }) : t("turns.wallClockValue", { elapsed })}>
            {active && elapsed !== "Unknown" ? t("turns.runningValue", { elapsed }) : elapsed === "Unknown" ? t("common.unknown") : elapsed}
          </dd>
        </div>
      </dl>

      {error ? (
        <div className="turn-error">
          <strong>{t("turns.error")}</strong>
          <code>{error.code}</code>
          <p>{error.message}</p>
          <small>{t("turns.errorItemsRemain")}</small>
        </div>
      ) : null}

      <UsageGrid label={t("turns.usage")} accessibleLabel={t("turns.usageFor", { id: turn.id })} usage={turn.usage} />
    </article>
  );
}

export function TurnTimeline({
  turns,
  items,
  sessionUsage,
  loadState,
  error = null,
  nowSeconds,
}: TurnTimelineProps) {
  const { t } = useTranslation("sessions");
  const [clock, setClock] = useState(() => nowSeconds ?? Math.floor(Date.now() / 1_000));
  const active = turns.some((turn) => turn.status === "in_progress" || turn.status === "waiting");
  const counts = useMemo(() => {
    const next = new Map<string, number>();
    for (const item of items) next.set(item.turn_id, (next.get(item.turn_id) ?? 0) + 1);
    return next;
  }, [items]);
  const knownTurnIds = useMemo(() => new Set(turns.map((turn) => turn.id)), [turns]);
  const unassociatedItems = items.filter((item) => !knownTurnIds.has(item.turn_id)).length;

  useEffect(() => {
    if (nowSeconds !== undefined) {
      setClock(nowSeconds);
      return;
    }
    if (!active) return;
    setClock(Math.floor(Date.now() / 1_000));
    const interval = window.setInterval(() => setClock(Math.floor(Date.now() / 1_000)), 1_000);
    return () => window.clearInterval(interval);
  }, [active, nowSeconds]);

  return (
    <section className="turn-timeline" aria-label={t("turns.timeline")} aria-busy={loadState === "loading" || undefined}>
      <header className="turn-timeline-heading">
        <div>
          <Activity size={14} strokeWidth={1.5} aria-hidden="true" />
          <strong>{t("turns.timeline")}</strong>
        </div>
        <span>{loadState === "ready" || turns.length ? t("turns.observed", { count: turns.length }) : t("turns.coreState")}</span>
      </header>

      <UsageGrid label={t("turns.sessionUsage")} usage={sessionUsage} landmark />

      {loadState === "loading" ? (
        <div className="turn-timeline-state">
          <Clock3 size={14} strokeWidth={1.5} aria-hidden="true" />
          <span>{turns.length ? t("turns.loadingComplete") : t("turns.loadingEveryPage")}</span>
        </div>
      ) : null}

      {loadState === "failed" ? (
        <div className="turn-timeline-failure" role="alert">
          <strong>{t("turns.loadFailed")}</strong>
          <p>{error || t("turns.coreReadFailed")}</p>
          {turns.length ? <small>{t("turns.lastObservedVisible")}</small> : null}
        </div>
      ) : null}

      {loadState === "ready" && !turns.length ? (
        <div className="turn-timeline-state">
          <Clock3 size={14} strokeWidth={1.5} aria-hidden="true" />
          <span>{t("turns.none")}</span>
        </div>
      ) : null}

      {turns.length ? (
        <ol className="turn-list">
          {turns.map((turn) => (
            <li key={turn.id}><TurnCard turn={turn} itemCount={counts.get(turn.id) ?? 0} now={clock} /></li>
          ))}
        </ol>
      ) : null}

      {unassociatedItems ? (
        <p className="turn-unassociated" role="status">
          {t("turns.unassociated", { count: unassociatedItems })}
        </p>
      ) : null}
    </section>
  );
}
