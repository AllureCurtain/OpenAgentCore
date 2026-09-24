import { Plus, RefreshCw, Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import i18n from "../../i18n";
import type { AgentCore, EnvironmentTemplate } from "@agents-core-web/agents-client";
import { Modal } from "../../components/Modal";
import { ErrorState } from "../../components/ErrorState";
import type { EnvironmentTemplateCatalog } from "../sessions/environment/environment-templates";
import { TemplateForm } from "./TemplateForm";
import { createTemplateWriteScope, templateName, templatePatch, type TemplateDraft } from "./template-editor";
import "./EnvironmentTemplatesView.css";

export interface EnvironmentTemplatesViewProps {
  catalog: EnvironmentTemplateCatalog | null;
  operations: Pick<AgentCore, "createEnvironmentTemplate" | "updateEnvironmentTemplate" | "deleteEnvironmentTemplate">;
  onRefresh: () => Promise<void>;
  onConfigureConnection: () => void;
}

type Dialog = { kind: "create" } | { kind: "edit" | "delete"; template: EnvironmentTemplate } | null;

export function EnvironmentTemplatesView({ catalog, operations, onRefresh, onConfigureConnection }: EnvironmentTemplatesViewProps) {
  const { t } = useTranslation("templates");
  const { t: tPages } = useTranslation("pages");
  const { t: tCommon } = useTranslation("common");
  const language = i18n.resolvedLanguage ?? "en";
  const formatTimestamp = (seconds: number) => new Intl.DateTimeFormat(language, { dateStyle: "medium", timeStyle: "short" }).format(new Date(seconds * 1000));
  const formatCount = (value: number) => new Intl.NumberFormat(language).format(value);
  const [query, setQuery] = useState("");
  const [dialog, setDialog] = useState<Dialog>(null);
  const [busy, setBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [needsRefresh, setNeedsRefresh] = useState(false);
  const scope = useRef(createTemplateWriteScope());
  const mounted = useRef(true);
  const refreshPending = useRef(false);
  const writePending = useRef(false);
  const failedCatalog = useRef<EnvironmentTemplateCatalog | null>(null);

  useEffect(() => {
    mounted.current = true;
    scope.current = createTemplateWriteScope();
    return () => { mounted.current = false; scope.current.dispose(); };
  }, []);

  useEffect(() => {
    if (needsRefresh && catalog?.state === "ready" && catalog !== failedCatalog.current) setNeedsRefresh(false);
  }, [catalog, needsRefresh]);

  const ready = catalog?.state === "ready";
  const blocked = busy || refreshing || needsRefresh || !ready;
  const templates = ready ? catalog.templates : [];
  const needle = query.trim().toLocaleLowerCase();
  const visible = templates.filter((template) => [templateName(template), template.id, template.network.access].some((value) => value.toLocaleLowerCase().includes(needle)));

  const refresh = async () => {
    if (refreshPending.current || writePending.current) return;
    refreshPending.current = true;
    setRefreshing(true);
    try { await onRefresh(); }
    catch { if (mounted.current) setError(t("refreshFailed")); }
    finally { refreshPending.current = false; if (mounted.current) setRefreshing(false); }
  };

  const write = async (request: (signal: AbortSignal) => Promise<unknown>, success: string) => {
    if (blocked || writePending.current) return;
    writePending.current = true;
    setBusy(true); setError(null); setNotice(null);
    const result = await scope.current.run(request, onRefresh);
    writePending.current = false;
    if (!mounted.current || result.kind === "ignored") return;
    setBusy(false);
    setDialog(null);
    if (result.kind === "failed") {
      failedCatalog.current = catalog;
      setNeedsRefresh(true);
      setError(result.message);
    } else {
      setNotice(success);
      if (result.kind === "saved-refresh-failed") {
        failedCatalog.current = catalog;
        setNeedsRefresh(true);
        setError(t("savedRefreshFailed"));
      }
    }
  };

  const save = (draft: TemplateDraft) => {
    if (dialog?.kind === "create") {
      void write((signal) => operations.createEnvironmentTemplate({ name: draft.name.trim() || null, network: { access: draft.access } }, { signal }), t("created"));
    } else if (dialog?.kind === "edit") {
      const template = dialog.template;
      const patch = templatePatch(template, draft);
      if (Object.keys(patch).length) void write((signal) => operations.updateEnvironmentTemplate(template.id, patch, { signal }), t("updated"));
    }
  };
  const close = () => { if (!busy) setDialog(null); };

  return (
    <section className="page-section templates-page" aria-labelledby="templates-heading">
      <header className="page-header">
        <div><h1 id="templates-heading">{tPages("templates.title")}</h1><p>{tPages("templates.subtitle")}</p></div>
        <div className="page-actions">
          <button className="button outline" type="button" disabled={busy || refreshing} onClick={() => void refresh()}><RefreshCw size={14} aria-hidden="true" />{refreshing ? tPages("templates.refreshing") : tPages("templates.refresh")}</button>
          <button className="button primary" type="button" disabled={blocked} onClick={() => setDialog({ kind: "create" })}><Plus size={14} aria-hidden="true" />{tPages("templates.newTemplate")}</button>
        </div>
      </header>
      <div className="templates-content">
        <p className="templates-boundary">{t("boundary")}</p>
        {notice ? <p className="notice success" role="status">{notice}</p> : null}
        {error ? <p className="notice warning" role="alert">{error}</p> : null}
        {catalog === null ? <p role="status" aria-busy="true">{t("loading")}</p> : catalog.state === "unsupported" ? (
          <ErrorState title={t("unavailableTitle")} description={t("unavailableDescription")} action={<button className="button outline" onClick={onConfigureConnection}>{t("configureConnection")}</button>} />
        ) : catalog.state === "failed" ? (
          <ErrorState title={t("loadFailedTitle")} description={t("loadFailedDescription")} action={<button className="button outline" onClick={onConfigureConnection}>{t("configureConnection")}</button>} />
        ) : (
          <>
            <div className="templates-toolbar">
              <label className="templates-search"><Search size={15} aria-hidden="true" /><span className="sr-only">{t("filterLabel")}</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("filterPlaceholder")} /></label>
              <span>{t("count", { visible: formatCount(visible.length), total: formatCount(templates.length) })}</span>
            </div>
            {templates.length === 0 ? <div className="templates-empty"><h2>{t("emptyTitle")}</h2><p>{t("emptyDescription")}</p></div> : visible.length === 0 ? <div className="templates-empty"><h2>{t("noMatch")}</h2><button className="button outline" onClick={() => setQuery("")}>{t("clearFilter")}</button></div> : (
              <div className="templates-list" aria-label={t("listLabel")}>
                {visible.map((template) => (
                  <article className="template-row" key={template.id}>
                    <div className="template-identity"><h2>{templateName(template)}</h2><code>{template.id}</code><small>{t("updatedAt", { date: formatTimestamp(template.updated_at) })}</small></div>
                    <dl className="template-summary"><div><dt>{t("network")}</dt><dd>{template.network.access === "enabled" ? t("enabled") : t("disabled")}</dd></div><div><dt>{t("allowedDomains")}</dt><dd>{formatCount(template.network.allowed_domains.length)}</dd></div></dl>
                    <div className="template-row-actions"><button className="button outline" type="button" disabled={blocked} aria-label={t("editLabel", { name: templateName(template) })} onClick={() => setDialog({ kind: "edit", template })}>{t("edit")}</button><button className="button outline" type="button" disabled={blocked} aria-label={t("deleteLabel", { name: templateName(template) })} onClick={() => setDialog({ kind: "delete", template })}>{t("delete")}</button></div>
                  </article>
                ))}
              </div>
            )}
          </>
        )}
      </div>
      <Modal open={dialog !== null} title={dialog?.kind === "delete" ? t("modal.delete") : dialog?.kind === "edit" ? t("modal.edit") : t("modal.create")} onClose={close}>
        {dialog?.kind === "delete" ? <div className="template-delete">
          <p>{t("deletePrompt", { name: templateName(dialog.template) })}</p><code>{dialog.template.id}</code>
          <p>{t("deleteWarning")}</p>
          <div className="template-form-actions"><button className="button outline" disabled={busy} onClick={close}>{tCommon("actions.cancel")}</button><button className="button danger" disabled={blocked} onClick={() => { const template = dialog.template; void write((signal) => operations.deleteEnvironmentTemplate(template.id, { signal }), t("deleted")); }}>{busy ? t("deleting") : t("deleteTemplate")}</button></div>
        </div> : dialog ? <TemplateForm key={dialog.kind === "edit" ? dialog.template.id : "new"} template={dialog.kind === "edit" ? dialog.template : undefined} busy={blocked} onSave={save} onCancel={close} /> : null}
      </Modal>
    </section>
  );
}
