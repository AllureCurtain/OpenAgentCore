import { useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import i18n from "../../i18n";

import "./VaultCreateForm.css";
import {
  parseVaultMetadata,
  VaultMetadataValidationError,
  type VaultMetadata,
} from "./vault-metadata";

const VAULT_NAME_MAX_BYTES = 256;

const tv = (key: string) => i18n.t(key as never, { ns: "vaults" });
export const vaultCreateFailureMessage = tv("errors.createUncertain");

export interface VaultCreateDraft {
  metadata: string;
  name: string;
}

export interface VaultCreateSubmissionGate {
  pending: boolean;
}

export type VaultCreateAttemptResult =
  | { kind: "success"; draft: VaultCreateDraft }
  | { kind: "validation_error"; draft: VaultCreateDraft; message: string }
  | { kind: "failure"; draft: VaultCreateDraft; message: string }
  | { kind: "ignored"; draft: VaultCreateDraft };

export interface VaultCreateFormProps {
  disabled?: boolean;
  onCreate: (name: string, metadata: VaultMetadata) => Promise<void>;
}

export function createVaultSubmissionGate(): VaultCreateSubmissionGate {
  return { pending: false };
}

function trimmedVaultName(value: string): string {
  return value.trim();
}

function validVaultName(value: string): boolean {
  return Boolean(value) && new TextEncoder().encode(value).length <= VAULT_NAME_MAX_BYTES;
}

/**
 * One explicit Vault creation attempt. The mutable gate is set synchronously,
 * before calling Core, so a second submit cannot race a React render. This
 * helper never retries and never exposes an arbitrary transport exception.
 */
export async function submitVaultCreateDraft(
  gate: VaultCreateSubmissionGate,
  draft: VaultCreateDraft,
  onCreate: VaultCreateFormProps["onCreate"],
  disabled = false,
): Promise<VaultCreateAttemptResult> {
  if (disabled || gate.pending) return { kind: "ignored", draft };

  const name = trimmedVaultName(draft.name);
  if (!validVaultName(name)) {
    return {
      kind: "validation_error",
      draft,
      message: tv("errors.vaultName"),
    };
  }

  let metadata: VaultMetadata;
  try {
    metadata = parseVaultMetadata(draft.metadata);
  } catch (error) {
    return {
      kind: "validation_error",
      draft,
      message: error instanceof VaultMetadataValidationError
        ? error.message
        : tv("errors.metadataUnknown"),
    };
  }

  gate.pending = true;
  try {
    await onCreate(name, metadata);
    return { kind: "success", draft: { metadata: "", name: "" } };
  } catch {
    return { kind: "failure", draft, message: tv("errors.createUncertain") };
  } finally {
    gate.pending = false;
  }
}

export function VaultCreateForm({ disabled = false, onCreate }: VaultCreateFormProps) {
  const { t } = useTranslation("vaults");
  const [draft, setDraft] = useState<VaultCreateDraft>({ metadata: "", name: "" });
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const gateRef = useRef<VaultCreateSubmissionGate>(createVaultSubmissionGate());
  const unavailable = disabled || submitting;

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (disabled || gateRef.current.pending) return;

    setSubmitting(true);
    setError(null);
    const result = await submitVaultCreateDraft(gateRef.current, draft, onCreate, disabled);
    if (result.kind !== "ignored") {
      setDraft(result.draft);
      setError(result.kind === "validation_error" || result.kind === "failure" ? result.message : null);
      setSubmitting(false);
    }
  };

  return (
    <form className="form-stack vault-create-form" onSubmit={(event) => void submit(event)} noValidate>
      <label className="field">
        <span>{t("createForm.name")}</span>
        <input
          autoFocus
          disabled={unavailable}
          onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
          placeholder={t("createForm.namePlaceholder")}
          value={draft.name}
        />
        <small>{t("createForm.nameHelp")}</small>
      </label>

      <label className="field">
        <span>{t("createForm.metadata")} <span className="field-optional">{t("createForm.optional")}</span></span>
        <textarea
          aria-describedby="vault-create-metadata-help"
          disabled={unavailable}
          onChange={(event) => setDraft((current) => ({ ...current, metadata: event.target.value }))}
          placeholder={'{"team":"runtime"}'}
          rows={7}
          spellCheck={false}
          value={draft.metadata}
        />
        <small id="vault-create-metadata-help">{t("createForm.metadataHelp", { emptyObject: "{}" })}</small>
      </label>

      <div className="notice warning vault-create-metadata-warning" role="note">
        {t("createForm.metadataWarning")}
      </div>

      {error ? <p className="field-error" role="alert">{error}</p> : null}

      <div className="vault-create-actions">
        <button className="button primary" type="submit" disabled={unavailable || !draft.name.trim()}>
          {submitting ? t("createForm.creating") : t("createForm.create")}
        </button>
      </div>
    </form>
  );
}
