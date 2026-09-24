import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";

import { AgentCoreError } from "@agents-core-web/agents-client";

import { Modal } from "../../components/Modal";
import i18n from "../../i18n";

const tv = (key: string, options?: Record<string, unknown>) => i18n.t(key as never, { ns: "vaults", ...options });
export const credentialStorageUnavailableMessage = tv("errors.storageUnavailable");

export function safeCredentialMutationError(error: unknown): string {
  if (error instanceof AgentCoreError) {
    if (error.status === 503 && error.code === "credential_write_failed") {
      return tv("errors.storageUnavailable");
    }
    if (error.status === 401) return tv("errors.auth");
    if (error.status === 404) return tv("errors.missing");
    if (error.status === 413) return tv("errors.tooLarge");
    if (error.status === 400) return tv("errors.invalidFields");
  }
  return tv("errors.credentialUncertain");
}

function executableCredentialURL(value: string): boolean {
  if (
    value !== value.trim() || /[\u0000-\u0020\u007f\\]/u.test(value) ||
    value.includes("?") || value.includes("#")
  ) return false;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && Boolean(url.hostname) && !url.username && !url.password;
  } catch {
    return false;
  }
}

function executableBearerToken(value: string): boolean {
  return /^[A-Za-z0-9\-._~+/]+=*$/.test(value);
}

export function CredentialDialog({
  credentialName,
  open,
  onClose,
  onCreate,
  onReplace,
}: {
  credentialName?: string;
  open: boolean;
  onClose: () => void;
  onCreate?: (name: string, serverURL: string, token: string) => Promise<void>;
  onReplace?: (token: string) => Promise<void>;
}) {
  const { t } = useTranslation("vaults");
  const { t: tCommon } = useTranslation("common");
  const formId = useId();
  const tokenRef = useRef<HTMLInputElement>(null);
  const [name, setName] = useState("");
  const [serverURL, setServerURL] = useState("");
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const replacing = Boolean(onReplace);

  useEffect(() => {
    if (!open) {
      if (tokenRef.current) tokenRef.current.value = "";
      return;
    }
    if (tokenRef.current) tokenRef.current.value = "";
    setName("");
    setServerURL("");
    setFieldError(null);
    setRequestError(null);
  }, [open, replacing]);

  const close = () => {
    if (tokenRef.current) tokenRef.current.value = "";
    onClose();
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting) return;
    const tokenInput = tokenRef.current;
    const token = tokenInput?.value ?? "";
    const trimmedName = name.trim();
    if (!executableBearerToken(token)) {
      setFieldError(t("errors.bearer"));
      return;
    }
    if (!replacing && (!trimmedName || new TextEncoder().encode(trimmedName).length > 256)) {
      setFieldError(t("errors.credentialName"));
      return;
    }
    if (!replacing && !executableCredentialURL(serverURL)) {
      setFieldError(t("errors.exactUrl"));
      return;
    }

    setSubmitting(true);
    setFieldError(null);
    setRequestError(null);
    try {
      const request = replacing
        ? onReplace?.(token)
        : onCreate?.(trimmedName, serverURL, token);
      // fetch receives its immutable request body synchronously. Clear the only
      // DOM-held copy immediately; retries always require deliberate re-entry.
      if (tokenInput) tokenInput.value = "";
      await request;
      onClose();
    } catch (error) {
      if (tokenInput) tokenInput.value = "";
      setRequestError(safeCredentialMutationError(error));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={close}
      title={replacing ? t("credentialDialog.replaceTitle", { name: credentialName ?? t("credential") }) : t("credentialDialog.addTitle")}
      footer={(
        <>
          <button className="button outline" type="button" onClick={close} disabled={submitting}>{tCommon("actions.cancel")}</button>
          <button className="button primary" type="submit" form={formId} disabled={submitting}>
            {submitting ? t("credentialDialog.saving") : replacing ? t("replaceToken") : t("credentialDialog.create")}
          </button>
        </>
      )}
    >
      <form id={formId} className="form-stack" onSubmit={(event) => void submit(event)} noValidate>
        {requestError ? <div className="session-action-error" role="alert"><strong>{t("credentialDialog.notConfirmed")}</strong><span>{requestError}</span></div> : null}
        {!replacing ? (
          <>
            <label className="field">
              <span>{t("credentialDialog.name")}</span>
              <input value={name} onChange={(event) => setName(event.target.value)} maxLength={256} disabled={submitting} placeholder={t("credentialDialog.namePlaceholder")} />
            </label>
            <label className="field">
              <span>{t("credentialDialog.exactUrl")}</span>
              <input value={serverURL} onChange={(event) => setServerURL(event.target.value)} disabled={submitting} placeholder="https://mcp.example.com/endpoint" inputMode="url" spellCheck={false} />
              <small>{t("credentialDialog.urlHelp")}</small>
            </label>
          </>
        ) : (
          <div className="notice warning" role="note">{t("credentialDialog.replaceWarning")}</div>
        )}
        <label className="field">
          <span>{replacing ? t("credentialDialog.newToken") : t("credentialDialog.token")}</span>
          <input ref={tokenRef} type="password" autoComplete="new-password" spellCheck={false} disabled={submitting} aria-describedby={`${formId}-token-help`} />
          <small id={`${formId}-token-help`}>{t("credentialDialog.tokenHelp")}</small>
        </label>
        {fieldError ? <p className="field-error" role="alert">{fieldError}</p> : null}
      </form>
    </Modal>
  );
}
