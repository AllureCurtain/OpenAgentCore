import { useState } from "react";
import type { UpdateSandboxDeployment, SandboxDeployment, StartSandboxReset } from "@agents-core-web/agents-client";
import { useTranslation } from "react-i18next";
import { HelpTip } from "../../components/console-ui";
import { formatBytes, formatPeriod, MISSING } from "../../lib/format";
import type { MessageKey } from "../../lib/locale-strings";
import { sandboxProviderLabel } from "../../lib/sandbox-labels";
import { sandboxSize, templateBuildSize, templateBuildStatus } from "./deployment-specification";
import { SandboxRolloutSummary } from "./SandboxRolloutSummary";
import { SandboxResetControls } from "./SandboxResetControls";
import { SandboxSetupWizard } from "./SandboxSetupWizard";

const MIB = 2 ** 20;
const buildStatusLabel: Record<ReturnType<typeof templateBuildStatus>, MessageKey> = { ready: "Ready", notReady: "Not ready", unknown: "Unknown state" };

export function SandboxDeploymentSettings({ deployment, disabled, fresh, onReset, onCancelReset, onUpdate }: {
  deployment: SandboxDeployment;
  disabled: boolean;
  fresh: boolean;
  onReset: (input: StartSandboxReset) => Promise<boolean>;
  onCancelReset: (expectedGeneration: number) => Promise<boolean>;
  onUpdate: (input: UpdateSandboxDeployment) => Promise<void>;
}) {
  const { t, i18n } = useTranslation("sandbox");
  const locale = i18n.resolvedLanguage?.startsWith("zh") ? "zh" : "en";
  const [changing, setChanging] = useState(false);
  const spec = deployment.specification;
  // An E2B selection may adopt its template build's size instead of saving one.
  const size = sandboxSize(deployment);
  const build = deployment.e2b?.template_build;
  const buildSize = templateBuildSize(deployment);
  const sizeLabel = (value: { cpus: number; memory_mib: number }) => t("{{cpus}} CPU · {{memory}}", { cpus: value.cpus, memory: formatBytes(value.memory_mib * MIB) });
  const canEdit = fresh && !deployment.reset;
  return <section className="sandbox-provider-settings form-stack" aria-labelledby="sandbox-provider-heading">
    <div className="sandbox-provider-title">
      <h2 id="sandbox-provider-heading">{t("Deployment provider")}</h2>
      <HelpTip>
        {t("One provider serves this deployment. Reset it before choosing a different backend.")}
        {deployment.provider === "e2b" ? ` ${t("Core creates E2B sandboxes directly. No node enrollment is needed.")} ${t("Saved configuration does not confirm execution readiness. Session and Environment state report actual execution.")}` : ""}
      </HelpTip>
    </div>
    <dl className="sandbox-summary">
      <div><dt>{t("Provider")}</dt><dd>{sandboxProviderLabel(deployment.provider, locale)}</dd></div>
      <div><dt>{t("Each sandbox")}</dt><dd>{size ? `${sizeLabel(size)}${size.root_disk_mib ? ` · ${t("Root disk {{root}} · data disk {{data}}", { root: formatBytes(size.root_disk_mib * MIB), data: formatBytes((size.environment_disk_mib ?? 0) * MIB) })}` : ""}` : MISSING}</dd></div>
      {spec?.runtime ? <div><dt>{t("Runtime")}</dt><dd><code title={spec.runtime.source_commit}>{spec.runtime.source_commit.slice(0, 12)}</code></dd></div> : null}
      {deployment.suspension ? <div><dt>{t("Idle suspension")}</dt><dd>{t("After {{idle}} · kept {{retention}}", { idle: formatPeriod(deployment.suspension.idle_seconds, i18n.resolvedLanguage), retention: formatPeriod(deployment.suspension.retention_seconds, i18n.resolvedLanguage) })}</dd></div> : null}
      <div><dt>{t("Allocated resources")}</dt><dd>{deployment.resources?.allocations ?? t("Unknown state")}</dd></div>
      <div><dt>{t("Pending environments")}</dt><dd>{deployment.resources?.pending ?? t("Unknown state")}</dd></div>
    </dl>
    {deployment.provider === "e2b" ? <div className="sandbox-cloud-summary">
      <dl className="sandbox-summary">
        <div><dt>{t("Immutable Runtime template")}</dt><dd>{deployment.e2b?.template || t("Unknown state")}</dd></div>
        <div>
          <dt>{t("Template build")}</dt>
          <dd>{[
            t(buildStatusLabel[templateBuildStatus(build)]),
            ...(buildSize ? [sizeLabel(buildSize)] : []),
            ...(build?.resources.root_disk_mib != null ? [t("{{disk}} disk", { disk: formatBytes(build.resources.root_disk_mib * MIB) })] : []),
          ].join(" · ")}</dd>
        </div>
        <div><dt>{t("E2B credential")}</dt><dd>{t(deployment.e2b?.credential_configured ? "Configured" : "Not configured")}</dd></div>
      </dl>
    </div> : null}
    <SandboxRolloutSummary deployment={deployment} stale={!fresh} />
    {!deployment.reset ? <>
      <div className="sandbox-actions">
        <button type="button" className="button outline" disabled={disabled || !canEdit || changing} onClick={() => setChanging(true)}>{t("Change resources")}</button>
        <HelpTip>{t("Changes keep this backend and existing Sessions and nodes. Core prepares the new target for future work; placement may continue on qualified earlier generations.")}</HelpTip>
      </div>

      {changing ? <>
        <SandboxSetupWizard
          coreUrl={deployment.core_url}
          expectedGeneration={deployment.generation}
          current={deployment.provider ? { provider: deployment.provider, specification: deployment.specification, e2bTemplate: deployment.e2b?.template } : undefined}
          disabled={disabled || !canEdit}
          editing
          onSubmit={onUpdate}
        />
        <p>{t("Existing sandboxes keep their configuration generation. Saving does not move them or prove the target is ready.")}</p>
        <button type="button" className="button outline" disabled={disabled} onClick={() => setChanging(false)}>{t("Cancel editing")}</button>
      </> : null}
    </> : null}
    <SandboxResetControls deployment={deployment} disabled={disabled || (changing && !deployment.reset)} stale={!fresh} onStart={onReset} onCancel={onCancelReset} />
  </section>;
}
