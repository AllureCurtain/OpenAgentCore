import { Check, Container, Copy, ExternalLink, Info, KeyRound } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

import {
  isLocalProxyBaseUrl,
  isValidDirectCoreBaseUrl,
  type CoreConnection,
} from "../lib/connection";
import {
  probeCore,
  type CoreProbeResult,
} from "../lib/core-probe";
import type { LocalDockerBackendGuideProfile } from "../lib/docker-guide-config";
import { Modal } from "./Modal";
import "./ConnectionModal.css";

interface ConnectionModalProps {
  connection: CoreConnection;
  open: boolean;
  proxyAuthEnabled?: boolean;
  dockerBackendGuide?: LocalDockerBackendGuideProfile | null;
  onClose: () => void;
  onSave: (connection: CoreConnection) => void;
}

type ConnectionMode = "local" | "advanced";

export type ConnectionProbeState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "complete"; result: CoreProbeResult };

const operatorGuideUrl = "https://github.com/MiniMax-AI/parsar-core/blob/main/docs/web/core-connection.md";
const troubleshootingUrl = `${operatorGuideUrl}#troubleshooting`;
const parsarCoreSetupUrl = "https://github.com/MiniMax-AI/parsar-core/blob/main/services/agents-api/README.md#standalone-http-service";
const parsarContainerSetupUrl = "https://github.com/MiniMax-AI/parsar-core/blob/main/services/agents-api/CONTAINER.md";
const parsarDaemonSetupUrl = "https://github.com/MiniMax-AI/parsar-core/blob/main/services/agents-api/README.md#internal-execution-device-connection";

function DockerCommand({ label, command }: { label: string; command: string }) {
  const { t } = useTranslation("connection");
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    if (!navigator.clipboard) return;
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1_500);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="connection-docker-command">
      <code>{command}</code>
      <button className="button outline" type="button" onClick={() => void copy()} aria-label={t("copyLabel", { label })}>
        {copied ? <Check size={12} aria-hidden="true" /> : <Copy size={12} aria-hidden="true" />}
        {copied ? t("copied") : t("copy")}
      </button>
    </div>
  );
}

function DockerBackendGuide({ profile }: { profile: LocalDockerBackendGuideProfile | null }) {
  const { t } = useTranslation("connection");
  return (
    <section className="connection-docker-guide" aria-labelledby="connection-docker-guide-title">
      <div className="connection-guide-header">
        <Container size={15} strokeWidth={1.5} aria-hidden="true" />
        <div>
          <strong id="connection-docker-guide-title">{t("dockerTitle")}</strong>
          <p>{t("dockerIntro")}</p>
        </div>
      </div>
      {profile ? (
        <>
          <div className="connection-docker-path">
            <div className="connection-docker-path-heading">
              <strong>{t("existingTitle")}</strong>
              <span>{t("existingSubtitle")}</span>
            </div>
            <ol className="connection-docker-steps">
              <li>
                <span><strong>{t("databaseStart")}</strong><small>{t("databaseWait")}</small></span>
                <DockerCommand label="database start command" command={`docker start ${profile.databaseContainer}`} />
              </li>
              <li>
                <span><strong>{t("coreDaemonStart")}</strong><small>{t("daemonReconnects")}</small></span>
                <DockerCommand
                  label="Core API and daemon start command"
                  command={`docker start ${profile.apiContainer} ${profile.daemonContainer}`}
                />
              </li>
              <li>
                <span><strong>{t("verifyHealth")}</strong><small>{t("healthBoundary")}</small></span>
                <DockerCommand
                  label="Core health command"
                  command={`curl --fail --silent --show-error http://127.0.0.1:${profile.corePort}/healthz`}
                />
              </li>
            </ol>
          </div>
          <div className="connection-docker-path connection-docker-first-use">
            <div className="connection-docker-path-heading">
              <strong>{t("firstTime")}</strong>
              <span>{t("firstTimeSubtitle")}</span>
            </div>
            <p>{t("dockerAssumption")}</p>
            <ol className="connection-docker-first-steps">
              <li>
                <span><strong>{t("buildImage")}</strong><small>{t("reviewedCheckout")}</small></span>
                <DockerCommand label="Core image build command" command="make docker-build-agents-api" />
              </li>
              <li><span><strong>{t("createStack")}</strong><small>{t("createStackHelp")}</small></span></li>
              <li><span><strong>{t("provisionDaemon")}</strong><small>{t("provisionDaemonHelp")}</small></span></li>
              <li><span><strong>{t("returnTest")}</strong><small>{t("returnTestHelp")}</small></span></li>
            </ol>
            <div className="connection-docker-first-links">
              <a href={parsarContainerSetupUrl} target="_blank" rel="noreferrer">
                {t("containerSetup")}
                <ExternalLink size={12} strokeWidth={1.5} aria-hidden="true" />
              </a>
              <a href={parsarDaemonSetupUrl} target="_blank" rel="noreferrer">
                {t("daemonProvisioning")}
                <ExternalLink size={12} strokeWidth={1.5} aria-hidden="true" />
              </a>
              <a href={operatorGuideUrl} target="_blank" rel="noreferrer">
                {t("proxySetup")}
                <ExternalLink size={12} strokeWidth={1.5} aria-hidden="true" />
              </a>
            </div>
          </div>
        </>
      ) : (
        <div className="connection-docker-unconfigured">
          <strong>{t("dockerUnconfigured")}</strong>
          <p>{t("dockerUnconfiguredHelp")}</p>
        </div>
      )}
      <p className="connection-docker-boundary">{t("dockerBoundary")}</p>
    </section>
  );
}

function initialMode(connection: CoreConnection): ConnectionMode {
  return isLocalProxyBaseUrl(connection.baseUrl) ? "local" : "advanced";
}

function initialAdvancedDraft(connection: CoreConnection): CoreConnection {
  const direct = !isLocalProxyBaseUrl(connection.baseUrl);
  return {
    baseUrl: direct ? connection.baseUrl : "",
    token: direct ? connection.token : "",
  };
}

function resultCopy(result: CoreProbeResult, t: TFunction<"connection">): { title: string; detail: string } {
  switch (result.kind) {
    case "authenticated":
      return {
        title: t("result.authenticatedTitle"), detail: t("result.authenticatedDetail"),
      };
    case "invalid_configuration":
      return {
        title: t("result.invalidTitle"), detail: t("result.invalidDetail"),
      };
    case "unauthorized":
      return {
        title: t("result.unauthorizedTitle"), detail: t("result.unauthorizedDetail"),
      };
    case "protocol_mismatch":
      return {
        title: t("result.mismatchTitle"), detail: t("result.mismatchDetail"),
      };
    case "http_error":
      return {
        title: t("result.httpTitle", { status: result.httpStatus ?? "error" }), detail: t("result.httpDetail"),
      };
    case "unreachable":
      return {
        title: t("result.unreachableTitle"), detail: t("result.unreachableDetail"),
      };
  }
}

export function ConnectionProbeStatus({ state }: { state: ConnectionProbeState }) {
  const { t } = useTranslation("connection");
  if (state.status === "idle") return null;
  if (state.status === "loading") {
    return (
      <div className="connection-modal-probe-result loading" role="status" aria-live="polite">
        <strong>{t("testingTitle")}</strong><p>{t("testingDetail")}</p>
      </div>
    );
  }

  const copy = resultCopy(state.result, t);
  const failed = state.result.kind !== "authenticated";
  return (
    <div
      className={`connection-modal-probe-result ${failed ? "failed" : "succeeded"}`}
      role={failed ? "alert" : "status"}
      aria-live="polite"
    >
      <strong>{copy.title}</strong>
      <p>{copy.detail}</p>
      <small>{t("probeBoundary")}</small>
    </div>
  );
}

export function ConnectionModal({
  connection,
  open,
  proxyAuthEnabled = import.meta.env.DEV && __AGENTS_CORE_WEB_DEV_PROXY_AUTH__,
  dockerBackendGuide = __AGENTS_CORE_WEB_DOCKER_BACKEND_GUIDE__,
  onClose,
  onSave,
}: ConnectionModalProps) {
  const { t } = useTranslation("connection");
  const { t: translateCommon } = useTranslation("common");
  const modeName = useId();
  const [mode, setMode] = useState<ConnectionMode>(() => initialMode(connection));
  const [advancedDraft, setAdvancedDraft] = useState<CoreConnection>(() => initialAdvancedDraft(connection));
  const [probeState, setProbeState] = useState<ConnectionProbeState>({ status: "idle" });
  const probeControllerRef = useRef<AbortController | undefined>(undefined);
  const probeRevisionRef = useRef(0);
  const advancedUrlValid = isValidDirectCoreBaseUrl(advancedDraft.baseUrl);
  const draft = mode === "local"
    ? { baseUrl: "/v1", token: "" }
    : { baseUrl: advancedDraft.baseUrl.trim(), token: advancedDraft.token.trim() };

  useEffect(() => {
    setMode(initialMode(connection));
    setAdvancedDraft(initialAdvancedDraft(connection));
  }, [connection, open]);

  useEffect(() => {
    probeRevisionRef.current += 1;
    probeControllerRef.current?.abort();
    probeControllerRef.current = undefined;
    setProbeState({ status: "idle" });
  }, [advancedDraft.baseUrl, advancedDraft.token, mode, open, proxyAuthEnabled]);

  useEffect(() => () => probeControllerRef.current?.abort(), []);

  const testConnection = async () => {
    const controller = new AbortController();
    probeControllerRef.current?.abort();
    probeControllerRef.current = controller;
    const revision = ++probeRevisionRef.current;
    setProbeState({ status: "loading" });

    try {
      const result = await probeCore({
        baseUrl: draft.baseUrl,
        token: mode === "advanced" ? draft.token : undefined,
        signal: controller.signal,
      });
      if (probeRevisionRef.current !== revision || controller.signal.aborted) return;
      probeControllerRef.current = undefined;
      setProbeState({ status: "complete", result });
    } catch {
      if (probeRevisionRef.current !== revision || controller.signal.aborted) return;
      probeControllerRef.current = undefined;
      setProbeState({
        status: "complete",
        result: { kind: "unreachable", executionReadiness: "unknown" },
      });
    }
  };

  const testDisabled = probeState.status === "loading" || (mode === "advanced" && !advancedUrlValid);
  const applyDisabled = mode === "advanced" && !advancedUrlValid;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("title")}
      footer={
        <>
          <button className="button outline" type="button" onClick={onClose}>
            {translateCommon("actions.cancel")}
          </button>
          <button
            className="button primary"
            type="button"
            disabled={applyDisabled}
            onClick={() => onSave(draft)}
          >
            {t("apply")}
          </button>
        </>
      }
    >
      <fieldset className="connection-modal-modes">
        <legend>{t("mode")}</legend>
        <label className={mode === "local" ? "selected" : ""}>
          <input
            type="radio"
            name={modeName}
            value="local"
            checked={mode === "local"}
            onChange={() => setMode("local")}
          />
          <span>
            <strong>{t("localCore")}</strong><small>{t("localDefault")}</small>
          </span>
        </label>
        <label className={mode === "advanced" ? "selected" : ""}>
          <input
            type="radio"
            name={modeName}
            value="advanced"
            checked={mode === "advanced"}
            onChange={() => setMode("advanced")}
          />
          <span>
            <strong>{t("compatibleCore")}</strong><small>{t("advanced")}</small>
          </span>
        </label>
      </fieldset>

      {mode === "local" ? (
        <>
          <section className="connection-modal-local" aria-labelledby={`${modeName}-local-title`}>
            <div className="connection-guide-header">
              <KeyRound size={15} strokeWidth={1.5} aria-hidden="true" />
              <div>
                <strong id={`${modeName}-local-title`}>{t("localProxy")}</strong><p>{t("fixedBase")}</p>
              </div>
            </div>
            <dl>
              <div>
                <dt>{t("apiBase")}</dt>
                <dd><code>/v1</code></dd>
              </div>
              <div>
                <dt>{t("authentication")}</dt><dd>{proxyAuthEnabled ? t("keyDetected") : t("keyMissing")}</dd>
              </div>
            </dl>
            <p className="connection-modal-local-note">
              {proxyAuthEnabled
                ? t("proxySecure") : t("proxyConfigure")}
            </p>
          </section>
          <DockerBackendGuide profile={dockerBackendGuide} />
        </>
      ) : (
        <div className="form-stack connection-modal-advanced">
          <label className="field">
            <span>{t("compatibleUrl")}</span>
            <input
              value={advancedDraft.baseUrl}
              onChange={(event) => setAdvancedDraft((value) => ({ ...value, baseUrl: event.target.value }))}
              placeholder="https://core.example/v1"
              inputMode="url"
              spellCheck={false}
              aria-invalid={Boolean(advancedDraft.baseUrl) && !advancedUrlValid}
              aria-describedby={
                `${modeName}-advanced-url-help${advancedDraft.baseUrl && !advancedUrlValid
                  ? ` ${modeName}-advanced-url-error`
                  : ""}`
              }
            />
            <small id={`${modeName}-advanced-url-help`}>
              {t("urlHelp")}
            </small>
            {advancedDraft.baseUrl && !advancedUrlValid ? (
              <small id={`${modeName}-advanced-url-error`} className="field-error" role="alert">
                {t("urlError")}
              </small>
            ) : null}
          </label>
          <label className="field">
            <span className="field-label">
              {t("bearerToken")}<span className="field-optional">{t("tabOnly")}</span>
            </span>
            <input
              type="password"
              value={advancedDraft.token}
              onChange={(event) => setAdvancedDraft((value) => ({ ...value, token: event.target.value }))}
              placeholder={t("tokenPlaceholder")}
              autoComplete="off"
              aria-describedby={`${modeName}-advanced-token-help`}
            />
            <small id={`${modeName}-advanced-token-help`}>
              {t("tokenHelp")}
            </small>
          </label>
        </div>
      )}

      <section className="connection-modal-test" aria-labelledby={`${modeName}-test-title`}>
        <div>
          <strong id={`${modeName}-test-title`}>{t("testTitle")}</strong><p>{t("testHelp")}</p>
        </div>
        <button className="button outline" type="button" disabled={testDisabled} onClick={() => void testConnection()}>
          {probeState.status === "loading" ? t("testing") : t("test")}
        </button>
      </section>
      <ConnectionProbeStatus state={probeState} />

      <section className="connection-guide" aria-labelledby={`${modeName}-guide-title`}>
        <div className="connection-guide-header">
          <Info size={15} strokeWidth={1.5} aria-hidden="true" />
          <div>
            <strong id={`${modeName}-guide-title`}>{t("operatorSetup")}</strong><p>{t("operatorHelp")}</p>
          </div>
        </div>
        <div className="connection-guide-links">
          <a className="connection-guide-link" href={operatorGuideUrl} target="_blank" rel="noreferrer">
            {t("webGuide")}
            <ExternalLink size={12} strokeWidth={1.5} aria-hidden="true" />
          </a>
          <a className="connection-guide-link" href={troubleshootingUrl} target="_blank" rel="noreferrer">
            {t("troubleshooting")}
            <ExternalLink size={12} strokeWidth={1.5} aria-hidden="true" />
          </a>
          <a className="connection-guide-link" href={parsarCoreSetupUrl} target="_blank" rel="noreferrer">
            {t("coreSetup")}
            <ExternalLink size={12} strokeWidth={1.5} aria-hidden="true" />
          </a>
        </div>
      </section>
      <div className="notice neutral">
        <Info size={14} strokeWidth={1.5} aria-hidden="true" />
        {t("finalBoundary")}
      </div>
    </Modal>
  );
}
