import type { SandboxDeployment } from "@agents-core-web/agents-client";
import { useId } from "react";
import { useTranslation } from "react-i18next";
import { HelpTip } from "../../components/console-ui";
import { formatInteger } from "../../lib/format";
import "./SandboxManagerView.css";

/** Core's projection is authoritative; node lists never establish rollout completion. */
export function SandboxRolloutSummary({ deployment, stale = false, compact = false, onOpen }: { deployment: SandboxDeployment; stale?: boolean; compact?: boolean; onOpen?: () => void }) {
  const { t, i18n } = useTranslation("sandbox");
  const id = useId();
  if (!deployment.provider || deployment.reset) return null;
  const { rollout } = deployment;
  const needsAttention = rollout.nodes && (rollout.nodes.failed > 0 || rollout.nodes.update_required > 0 || rollout.nodes.unknown > 0);
  const count = (value: number) => formatInteger(value, i18n.resolvedLanguage);
  return <section className={`form-stack sandbox-rollout${compact ? " sandbox-rollout-compact" : ""}`} aria-labelledby={id}>
    <div className="sandbox-provider-title"><h3 id={id}>{t("Configuration rollout")}</h3><HelpTip>{t("Existing sandboxes keep their configuration generation. Saving does not move them or prove the target is ready.")} {rollout.state === "settled" ? t("Core reports no active preparation. This does not mean every node is ready.") : null}</HelpTip></div>
    {stale ? <p role="alert" className="sandbox-error">{t("Rollout state is unconfirmed. These are the last confirmed observations.")}</p> : null}
    <p role="status">{t(rollout.state === "preparing" ? "Core is preparing the target configuration." : "No active preparation")}</p>
    <dl className="sandbox-summary">
      <div><dt>{t("Target generation")}</dt><dd>{count(deployment.generation)}</dd></div>
      {!compact || rollout.previous_generation_sandboxes > 0 ? <div><dt>{t("Previous-generation sandboxes")}</dt><dd>{count(rollout.previous_generation_sandboxes)}</dd></div> : null}
    </dl>
    {rollout.nodes && (!compact || rollout.state === "preparing" || needsAttention) ? <dl className="sandbox-summary sandbox-rollout-nodes">
        <div><dt>{t("Ready for target")}</dt><dd>{count(rollout.nodes.ready)}</dd></div>
        <div><dt>{t("Preparing target")}</dt><dd>{count(rollout.nodes.preparing)}</dd></div>
        <div><dt>{t("Preparation failed")}</dt><dd>{count(rollout.nodes.failed)}</dd></div>
        <div><dt>{t("Node update required")}</dt><dd>{count(rollout.nodes.update_required)}</dd></div>
        <div><dt>{t("Target readiness unknown")}</dt><dd>{count(rollout.nodes.unknown)}</dd></div>
    </dl> : null}
    {needsAttention ? <p className="sandbox-notice">{t("Review affected nodes for preparation errors, required updates or an unconfirmed connection. Qualified earlier generations may still serve work.")}</p> : null}
    {onOpen ? <button type="button" className="text-action" onClick={onOpen}>{t("Review nodes")}</button> : null}
  </section>;
}
