import { Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { VaultCatalog } from "../../vaults/vault-catalog";
import { vaultName } from "../../vaults/vault-catalog";
import type { AgentToolDraft } from "../../agents/agent-form";

export interface SessionToolsEditorProps {
  catalog: VaultCatalog | null;
  disabled?: boolean;
  label: string;
  onChange: (tools: AgentToolDraft[]) => void;
  tools: AgentToolDraft[];
}

export function SessionToolsEditor({
  catalog,
  disabled = false,
  label,
  onChange,
  tools,
}: SessionToolsEditorProps) {
  const { t } = useTranslation("sessions");
  const update = (index: number, tool: AgentToolDraft) => {
    onChange(tools.map((candidate, position) => position === index ? tool : candidate));
  };
  const remove = (index: number) => {
    onChange(tools.filter((_, position) => position !== index));
  };

  return (
    <section className="agent-form-section agent-tools-section" aria-label={label}>
      <div className="agent-form-section-heading">
        <h3>{label}</h3>
        <span>{t("tools.wholeReplacement")}</span>
      </div>
      <p className="agent-form-capability-note">
        {t("tools.capabilityNote")}
      </p>
      <div className="agent-tools-list">
        {tools.map((tool, index) => tool.kind === "read-only" ? (
          <article className="agent-tool-card agent-tool-read-only" key={`read-only-${index}`} aria-label={t("tools.unsupportedInherited")}>
            <header>
              <div><strong>{t("tools.unsupportedInherited")}</strong><span>{tool.label}</span></div>
              <button className="icon-button danger" type="button" aria-label={t("tools.removeUnsupported", { number: index + 1 })} onClick={() => remove(index)} disabled={disabled}><Trash2 size={14} /></button>
            </header>
            <small>{t("tools.unsupportedHint")}</small>
          </article>
        ) : tool.kind === "function" ? (
          <article className="agent-tool-card" key={`function-${index}`}>
            <header>
              <strong>{t("tools.function")}</strong>
              <button className="button outline" type="button" onClick={() => remove(index)} disabled={disabled}>{t("common.remove")}</button>
            </header>
            <div className="agent-tool-grid">
              <label className="field"><span>{t("common.name")}</span><input value={tool.name} onChange={(event) => update(index, { ...tool, name: event.target.value })} placeholder="lookup_customer" disabled={disabled} /></label>
              <label className="field"><span>{t("common.description")}</span><input value={tool.description} onChange={(event) => update(index, { ...tool, description: event.target.value })} placeholder={t("tools.functionDescriptionPlaceholder")} disabled={disabled} /></label>
            </div>
            <label className="field">
              <span>{t("tools.parametersSchema")}</span>
              <textarea value={tool.parameters} onChange={(event) => update(index, { ...tool, parameters: event.target.value })} rows={6} spellCheck={false} disabled={disabled} />
              <small>{t("tools.parametersHint")}</small>
            </label>
          </article>
        ) : (
          <article className="agent-tool-card" key={`mcp-${index}`}>
            <header>
              <strong>{tool.credentialId ? t("tools.vaultMcp") : t("tools.httpMcp")}</strong>
              <button className="button outline" type="button" onClick={() => remove(index)} disabled={disabled}>{t("common.remove")}</button>
            </header>
            <div className="agent-tool-grid">
              <label className="field">
                <span>{t("tools.serverLabel")}</span>
                <input value={tool.serverLabel} onChange={(event) => update(index, { ...tool, serverLabel: event.target.value })} placeholder="docs" disabled={disabled} />
              </label>
              <label className="field">
                <span>{t("tools.authentication")}</span>
                <select
                  value={tool.credentialId ?? ""}
                  onChange={(event) => {
                    const credentialId = event.target.value || null;
                    const credential = credentialId
                      ? catalog?.credentials.find((candidate) => candidate.id === credentialId)
                      : null;
                    update(index, {
                      ...tool,
                      credentialId,
                      serverUrl: credential?.auth.mcp_server_url ?? tool.serverUrl,
                    });
                  }}
                  disabled={disabled}
                >
                  <option value="">{t("tools.implicitVaults")}</option>
                  {catalog?.vaults.map((vault) => {
                    const credentials = catalog.credentials.filter((credential) => credential.vault_id === vault.id);
                    return credentials.length ? (
                      <optgroup label={vaultName(vault)} key={vault.id}>
                        {credentials.map((credential) => (
                          <option value={credential.id} key={credential.id}>
                            {credential.name} · {credential.auth.mcp_server_url}
                          </option>
                        ))}
                      </optgroup>
                    ) : null;
                  })}
                </select>
                <small>
                  {t("tools.credentialHint")}
                </small>
              </label>
              <label className="field">
                <span>{t("tools.serverUrl")}</span>
                <input value={tool.serverUrl} onChange={(event) => update(index, { ...tool, serverUrl: event.target.value })} placeholder="https://mcp.example/tools" inputMode="url" spellCheck={false} readOnly={Boolean(tool.credentialId)} disabled={disabled} />
              </label>
            </div>
            <fieldset className="agent-mcp-allowed-tools" disabled={disabled}>
              <legend>{t("tools.allowedTools")}</legend>
              <label><input type="radio" checked={tool.allowedToolsMode === "all"} onChange={() => update(index, { ...tool, allowedToolsMode: "all", allowedToolsValue: null })} /> {t("tools.allAdvertised")}</label>
              <label><input type="radio" checked={tool.allowedToolsMode === "list"} onChange={() => update(index, { ...tool, allowedToolsMode: "list" })} /> {t("tools.onlyListed")}</label>
              {tool.allowedToolsMode === "list" ? (
                <textarea value={tool.allowedTools} onChange={(event) => update(index, { ...tool, allowedTools: event.target.value })} rows={4} placeholder={"search\nread_document"} spellCheck={false} aria-label={t("tools.allowedFor", { server: tool.serverLabel || t("tools.mcpServer") })} />
              ) : null}
              <small>{t("tools.allowedHint")}</small>
            </fieldset>
            <label className="agent-mcp-required">
              <input type="checkbox" checked={tool.required === true} onChange={(event) => update(index, { ...tool, required: event.target.checked })} disabled={disabled} />
              {t("tools.requireServer")}
            </label>
          </article>
        ))}
      </div>
      <div className="agent-tool-actions">
        <button className="button outline" type="button" onClick={() => onChange([...tools, { kind: "function", name: "", description: "", parameters: "{\n  \"type\": \"object\"\n}" }])} disabled={disabled}>
          <Plus size={13} /> {t("tools.addFunction")}
        </button>
        <button className="button outline" type="button" onClick={() => onChange([...tools, { kind: "mcp", serverLabel: "", serverUrl: "", allowedToolsMode: "all", allowedTools: "", allowedToolsValue: null, required: false, credentialId: null }])} disabled={disabled}>
          <Plus size={13} /> {t("tools.addHttpMcp")}
        </button>
      </div>
    </section>
  );
}
