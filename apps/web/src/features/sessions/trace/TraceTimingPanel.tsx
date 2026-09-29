import { useTranslation } from "react-i18next";

import { formatDateTime, formatDuration, MISSING } from "../../../lib/format";
import { ItemReceiptTiming } from "./ItemReceiptTiming";
import type { TraceGroup, TraceRow } from "./trace-model";

/** Native tool duration, public Turn times and Core receipts remain separate. */
export function TraceTimingPanel({ row, group }: { row: TraceRow; group: TraceGroup }) {
  const { t, i18n } = useTranslation("sessions");
  const duration = row.durationMs.state === "available" && row.durationMs.value !== null
    ? formatDuration(row.durationMs.value / 1000) : t(row.durationMs.state === "unknown" ? "common.unknown" : "common.unavailable");
  return <div className="trace-detail-timing">
    <dl>
      <div><dt>{t("trace.turnStarted")}</dt><dd>{formatDateTime(group.turn?.started_at, i18n.resolvedLanguage)}</dd></div>
      <div><dt>{t("trace.turnCompleted")}</dt><dd>{formatDateTime(group.turn?.completed_at, i18n.resolvedLanguage)}</dd></div>
      <div><dt>{t("trace.turnWallClock")}</dt><dd>{group.turnWallClockDurationMs.value === null ? MISSING : formatDuration(group.turnWallClockDurationMs.value / 1000)}</dd></div>
      <div><dt>{t("trace.toolReportedDuration")}</dt><dd>{duration}</dd></div>
    </dl>
    <ItemReceiptTiming turn={group.turn} items={row.sourceItems} />
  </div>;
}
