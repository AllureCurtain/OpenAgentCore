import type { IssuedExecutorCredential } from "@agents-core-web/agents-client";
import { Check, Copy, Download } from "lucide-react";
import { useEffect, useRef } from "react";
import { Trans, useTranslation } from "react-i18next";

import { useCopy } from "../api-keys/IssuedKey";
import { saveBlob } from "../skills/skill-operations";
import { InstallCommand, useSelectWhenCopyFails } from "./ExecutorInstallPanel";

/**
 * The self-hosted executor's credential as one line of JSON. The installer
 * also accepts it pretty-printed, but a one-line paste survives terminals that
 * warn about or bracket multi-line pastes; `--credential-file` reads the same
 * JSON from the downloaded file.
 */
function credentialText(credential: IssuedExecutorCredential): string {
  const { key_id, environment_id, executor_token } = credential;
  return JSON.stringify({ key_id, environment_id, executor_token });
}

/**
 * The one-time credential: copied to paste at the installer's hidden prompt,
 * or downloaded as a file for automation (`--credential-file`). What to do
 * next comes first: in the dialog, run the install command shown with it; on
 * the page, run the Connect a host command below; without the installer, save
 * the credential.
 */
export function CredentialFile({ credential, next, command = null }: { credential: IssuedExecutorCredential; next: "inline" | "panel" | "save"; command?: string | null }) {
  const { t } = useTranslation("sessions");
  const text = credentialText(credential);
  const { state, copy } = useCopy(text);
  const file = useRef<HTMLPreElement>(null);
  useSelectWhenCopyFails(state, file);
  // A downloaded credential's object URL goes with the credential: on Done or when this leaves the page.
  const downloads = useRef<(() => void)[]>([]);
  useEffect(() => {
    const revokes = downloads.current;
    return () => { for (const revoke of revokes.splice(0)) revoke(); };
  }, []);
  const download = () => {
    downloads.current.push(saveBlob(new Blob([`${text}\n`], { type: "application/json" }), `executor-credential-${credential.environment_id.slice(0, 8)}.json`));
  };
  return (
    <div className="executor-credential">
      <p className="executor-credential-notice">{t("executor.issued.notice")}</p>
      <p className="executor-credential-next">{t(`executor.issued.next.${next}`)}</p>
      {command ? <InstallCommand value={command} /> : null}
      <div role="region" aria-label={t("executor.issued.fileLabel")}><pre ref={file} className="executor-credential-file"><code>{text}</code></pre></div>
      <div className="executor-credential-actions">
        <button className="button primary" type="button" onClick={() => void copy()}>
          {state === "copied" ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}
          {state === "copied" ? t("executor.issued.copied") : t("executor.issued.copy")}
        </button>
        <button className="button outline" type="button" onClick={download}>
          <Download size={14} aria-hidden="true" />{t("executor.issued.download")}
        </button>
      </div>
      {state === "failed" ? <p className="executor-credential-error" role="alert">{t("executor.issued.copyFailed")}</p> : null}
      <p className="executor-credential-hint">
        <Trans t={t} i18nKey={next === "save" ? "executor.issued.downloadHint.installer" : "executor.issued.downloadHint.command"} components={{ chmod: <code>chmod 600 &lt;file&gt;</code>, flag: <code>--credential-file &lt;absolute path&gt;</code> }} />
      </p>
    </div>
  );
}
