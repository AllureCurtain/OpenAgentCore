import { Info, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { CoreStartupConfiguration } from "@agents-core-web/agents-client";

import { StatusIcon, type StatusKind } from "../../components/StatusIcon";
import type { CoreConnectionState } from "../../lib/connection";

import "./SystemView.css";

function stateKind(state: CoreConnectionState): StatusKind {
  if (state === "ready") return "completed";
  if (state === "failed") return "failed";
  return "running";
}

function harnessLabel(harness: string): string {
  if (harness === "claude_sdk") return "Claude SDK";
  if (harness === "mcode") return "MiniMax Code";
  if (harness === "codex") return "Codex";
  return harness;
}

function providerLabel(provider: string | null, noneLabel: string): string {
  if (provider === "microsandbox") return "Microsandbox";
  if (provider === "docker") return "Docker";
  return noneLabel;
}

interface SystemStatusCard {
  detail: string;
  label: string;
  status: StatusKind;
  value: string;
}

function StartupStatus({ enabled, label, enabledLabel, disabledLabel }: { enabled: boolean; label?: string; enabledLabel: string; disabledLabel: string }) {
  return (
    <span className={`system-config-status ${enabled ? "enabled" : "disabled"}`}>
      <span aria-hidden="true" />
      {label ?? (enabled ? enabledLabel : disabledLabel)}
    </span>
  );
}

export function SystemView({
  startupConfiguration,
  startupConfigurationState,
  startupConfigurationSupported,
  selfHostedWebEnabled,
  managedWebEnabled,
  refreshing,
  onRefresh,
}: {
  startupConfiguration: CoreStartupConfiguration | null;
  startupConfigurationState: CoreConnectionState;
  startupConfigurationSupported: boolean | null;
  selfHostedWebEnabled: boolean;
  managedWebEnabled: boolean;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const { t } = useTranslation(["system", "pages"]);
  const configuration = startupConfigurationState === "ready" && startupConfigurationSupported === true
    ? startupConfiguration
    : null;
  const endpointOverrideConfigured = configuration?.configured.model_providers.some((provider) => provider.endpoint_configured) ?? false;
  const executionAdaptersEnabled = (configuration?.configured.enabled_harnesses.length ?? 0) > 0;
  const startupUnavailable = startupConfigurationSupported === false;

  const startupValue = (value: string): string => {
    if (configuration) return value;
    if (startupUnavailable) return t("status.notExposed");
    if (startupConfigurationState === "failed") return t("status.unavailable");
    return t("status.checking");
  };
  const startupStatus: StatusKind = configuration
    ? "completed"
    : startupUnavailable
      ? "interrupted"
      : stateKind(startupConfigurationState);
  const startupDetail = startupUnavailable
    ? t("startup.notExposed")
    : startupConfigurationState === "failed"
      ? t("startup.failed")
      : t("startup.reported");
  const cards: SystemStatusCard[] = [
    {
      label: t("cards.daemon.label"),
      status: startupStatus,
      value: startupValue(configuration?.configured.daemon_gateway ? t("status.enabled") : t("status.notConfigured")),
      detail: configuration
        ? configuration.configured.daemon_gateway
          ? t(selfHostedWebEnabled ? "cards.daemon.acceptsWithWeb" : "cards.daemon.acceptsWithoutWeb")
          : t("cards.daemon.disabled")
        : startupDetail,
    },
    {
      label: t("cards.sandbox.label"),
      status: startupStatus,
      value: startupValue(configuration
        ? `${providerLabel(configuration.configured.managed_sandbox.provider, t("providers.none"))}${configuration.configured.managed_sandbox.maintenance ? ` · ${t("status.maintenance")}` : ""}`
        : ""),
      detail: configuration
        ? `${configuration.configured.managed_sandbox.maintenance ? t("cards.sandbox.maintenance") : ""}${t(managedWebEnabled ? "cards.sandbox.supportedWithWeb" : "cards.sandbox.supportedWithoutWeb", { providers: configuration.supported.managed_sandbox_providers.map((provider) => providerLabel(provider, t("providers.none"))).join(", ") })}`
        : startupDetail,
    },
    {
      label: t("cards.endpoints.label"),
      status: startupStatus,
      value: startupValue(endpointOverrideConfigured ? t("status.configured") : t("status.notConfigured")),
      detail: configuration
        ? endpointOverrideConfigured
          ? t("cards.endpoints.configured")
          : executionAdaptersEnabled
            ? t("cards.endpoints.nativeDefaults")
            : t("cards.endpoints.noAdapters")
        : startupDetail,
    },
  ];

  return (
    <section className="page-section architecture-page system-page" aria-labelledby="system-heading">
      <header className="page-header">
        <h1 id="system-heading">{t("system.title", { ns: "pages" })} <span>{t("system.subtitle", { ns: "pages" })}</span></h1>
        <div className="page-actions">
          <button
            className="button outline"
            type="button"
            onClick={onRefresh}
            disabled={refreshing}
            aria-label={refreshing ? t("refreshingLabel") : t("refreshLabel")}
          >
            <RefreshCw className={refreshing ? "refresh-spinning" : undefined} size={14} strokeWidth={1.5} aria-hidden="true" />
            {refreshing ? t("refreshing") : t("refresh")}
          </button>
        </div>
      </header>

      <div className="system-summary" role="list" aria-label={t("summaryLabel")} aria-live="polite" aria-busy={refreshing}>
        {cards.map((card) => (
          <div className="system-summary-cell" role="listitem" key={card.label}>
            <span><StatusIcon status={card.status} />{card.label}</span>
            <strong>{card.value}</strong>
            <small>{card.detail}</small>
          </div>
        ))}
      </div>

      {configuration ? (
        <>
          <section className="system-config-section" aria-labelledby="configured-harnesses-heading">
            <header>
              <h2 id="configured-harnesses-heading">{t("adapters.title")} <span>{t("adapters.qualifier")}</span></h2>
              <span>{t("adapters.subtitle")}</span>
            </header>
            <p className="system-config-explanation">
              {executionAdaptersEnabled
                ? t("adapters.selectable")
                : t("adapters.inactive")}
            </p>
            <div className="system-harness-grid">
              {configuration.supported.harnesses.map((harness) => {
                const enabled = configuration.configured.enabled_harnesses.includes(harness);
                const endpoint = configuration.configured.model_providers.find((provider) => provider.harness === harness);
                return (
                  <article className="system-harness-card" key={harness}>
                    <div>
                      <strong>{harnessLabel(harness)}</strong>
                      <StartupStatus
                        enabled={enabled}
                        label={enabled ? t("status.enabled") : t("adapters.buildOnly")}
                        enabledLabel={t("status.enabled")}
                        disabledLabel={t("status.notConfigured")}
                      />
                    </div>
                    <small>
                      {enabled
                        ? endpoint?.endpoint_configured
                          ? t("adapters.endpointConfigured")
                          : t("adapters.endpointNative")
                        : t("adapters.compiledOnly")}
                    </small>
                  </article>
                );
              })}
            </div>
          </section>

          <p className="system-boundary-note"><Info size={15} aria-hidden="true" />{t("boundary")}</p>
        </>
      ) : null}
    </section>
  );
}
