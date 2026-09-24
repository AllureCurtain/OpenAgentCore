import { useId, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import type { EnvironmentTemplate } from "@agents-core-web/agents-client";
import { templatePatch, type TemplateDraft } from "./template-editor";

export function TemplateForm({ template, busy, onSave, onCancel }: {
  template?: EnvironmentTemplate;
  busy: boolean;
  onSave: (draft: TemplateDraft) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation("templates");
  const { t: tCommon } = useTranslation("common");
  const id = useId();
  const [draft, setDraft] = useState<TemplateDraft>({ name: template?.name ?? "", access: template?.network.access ?? "enabled" });
  const tooLong = [...draft.name.trim()].length > 256;
  const networkEditable = !template || template.network.allowed_domains.length === 0;
  const unchanged = template && !tooLong && Object.keys(templatePatch(template, draft)).length === 0;
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!busy && !tooLong && !unchanged) onSave(draft);
  };
  return (
    <form className="template-form" onSubmit={submit}>
      <label htmlFor={`${id}-name`}>{t("form.name")} <span aria-hidden="true">{t("form.optional")}</span></label>
      <input id={`${id}-name`} value={draft.name} disabled={busy} aria-invalid={tooLong || undefined} aria-describedby={tooLong ? `${id}-error` : undefined}
        onChange={(event) => setDraft({ ...draft, name: event.target.value })} autoComplete="off" placeholder={t("form.namePlaceholder")} />
      {tooLong ? <p id={`${id}-error`} role="alert">{t("form.nameTooLong")}</p> : null}
      <label htmlFor={`${id}-network`}>{t("form.networkAccess")}</label>
      <select id={`${id}-network`} value={draft.access} disabled={busy || !networkEditable}
        onChange={(event) => setDraft({ ...draft, access: event.target.value as TemplateDraft["access"] })}>
        <option value="enabled">{t("enabled")}</option><option value="disabled">{t("disabled")}</option>
      </select>
      {!networkEditable ? <p>{t("form.policyLocked")}</p> : null}
      <p>{t("form.sessionBoundary")}</p>
      <div className="template-form-actions">
        <button className="button outline" type="button" disabled={busy} onClick={onCancel}>{tCommon("actions.cancel")}</button>
        <button className="button primary" type="submit" disabled={busy || tooLong || Boolean(unchanged)}>{busy ? t("form.saving") : template ? t("form.saveChanges") : t("form.create")}</button>
      </div>
    </form>
  );
}
