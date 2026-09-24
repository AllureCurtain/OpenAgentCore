import { Bot, ChevronDown, ChevronUp, MessageSquare, PanelsTopLeft, Plus } from "lucide-react";
import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import i18n from "../../i18n";

import type { SavedAgent } from "@agents-core-web/agents-client";

import type { VaultCatalog } from "../vaults/vault-catalog";
import { sessionAdmissionBlocker } from "./session-admission";
import { AGENT_TEMPLATES, type AgentTemplate } from "./agent-templates";

import "./AgentCatalog.css";

const useIsomorphicLayoutEffect = typeof document === "undefined" ? useEffect : useLayoutEffect;

export function twoRowSavedAgentCapacity(columnCount: number): number {
  return Math.max(1, Math.floor(columnCount) * 2 - 1);
}

function resolvedGridColumnCount(grid: HTMLElement): number {
  const template = window.getComputedStyle(grid).gridTemplateColumns.trim();
  if (!template || template === "none") return 1;
  return Math.max(1, template.split(/\s+/u).length);
}

function formatShortDate(seconds: number): string {
  return new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", { month: "short", day: "numeric" }).format(new Date(seconds * 1000));
}

function AgentSessionStartAction({
  agent,
  busy,
  onStart,
  vaultCatalog,
}: {
  agent: SavedAgent;
  busy: boolean;
  onStart: (agentId: string) => void;
  vaultCatalog: VaultCatalog | null;
}) {
  const { t } = useTranslation("agents");
  const blocker = sessionAdmissionBlocker(agent, vaultCatalog);
  const descriptionId = useId();
  return (
    <span className="action-tooltip">
      <button
        className="button outline agent-session-start"
        type="button"
        onClick={() => {
          if (!blocker) onStart(agent.id);
        }}
        disabled={busy}
        aria-disabled={blocker ? true : undefined}
        aria-label={t("catalog.startLabel", { name: agent.name || t("catalog.untitled"), id: agent.id })}
        aria-describedby={blocker ? descriptionId : undefined}
      >
        <MessageSquare size={14} strokeWidth={1.5} aria-hidden="true" />
        <span>{blocker ? t("catalog.unavailable") : t("catalog.startSession")}</span>
      </button>
      {blocker ? (
        <span className="action-tooltip-content" role="tooltip" id={descriptionId}>
          {t("catalog.sessionUnavailable", { reason: blocker })}
        </span>
      ) : null}
    </span>
  );
}

export function AgentCatalog({
  agents,
  busy,
  coreReady,
  hasSavedAgents,
  openingAgentId,
  vaultCatalog,
  onClearSearch,
  onCreate,
  onEdit,
  onExpandedChange,
  onStartSession,
  onUseTemplate,
  expanded,
  isFiltering,
}: {
  agents: SavedAgent[];
  busy: boolean;
  coreReady: boolean;
  expanded: boolean;
  hasSavedAgents: boolean;
  isFiltering: boolean;
  openingAgentId: string | null;
  vaultCatalog: VaultCatalog | null;
  onClearSearch: () => void;
  onCreate: (returnFocus: HTMLButtonElement) => void;
  onEdit: (agent: SavedAgent, returnFocus: HTMLButtonElement) => void;
  onExpandedChange: (expanded: boolean) => void;
  onStartSession: (agentId: string) => void;
  onUseTemplate: (template: AgentTemplate, returnFocus: HTMLButtonElement) => void;
}) {
  const { t } = useTranslation("agents");
  const count = (value: number) => new Intl.NumberFormat(i18n.resolvedLanguage ?? "en").format(value);
  const interactionDisabled = busy;
  const gridId = useId();
  const gridRef = useRef<HTMLDivElement>(null);
  const [columnCount, setColumnCount] = useState(1);
  const savedCapacity = twoRowSavedAgentCapacity(columnCount);
  const canExpand = !isFiltering && agents.length > savedCapacity;
  const visibleAgents = isFiltering || expanded ? agents : agents.slice(0, savedCapacity);
  const hiddenAgentCount = Math.max(0, agents.length - savedCapacity);

  useIsomorphicLayoutEffect(() => {
    const grid = gridRef.current;
    if (!grid) return;

    const updateColumnCount = () => {
      const next = resolvedGridColumnCount(grid);
      setColumnCount((current) => current === next ? current : next);
    };
    updateColumnCount();

    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", updateColumnCount);
      return () => window.removeEventListener("resize", updateColumnCount);
    }

    const observer = new ResizeObserver(updateColumnCount);
    observer.observe(grid);
    return () => observer.disconnect();
  }, []);

  return (
    <div className="agents-catalog-scroll">
      <section className="agent-catalog-section" aria-labelledby="saved-agents-heading">
        <h2 id="saved-agents-heading">{t("catalog.listLabel")}</h2>
        <div ref={gridRef} id={gridId} className="agent-card-grid" role="list" aria-label={t("catalog.listLabel")} aria-busy={openingAgentId ? true : undefined}>
          <div className="agent-catalog-slot" role="listitem">
            <button
              className="agent-catalog-card agent-create-card"
              type="button"
              data-create-agent-entry="true"
              onClick={(event) => onCreate(event.currentTarget)}
              disabled={!coreReady || interactionDisabled}
            >
              <span className="agent-card-icon"><Plus size={19} strokeWidth={1.7} aria-hidden="true" /></span>
              <strong>{t("catalog.create")}</strong>
              <span className="agent-card-description">{t("catalog.createDescription")}</span>
            </button>
          </div>
          {visibleAgents.map((agent) => (
            <article className="agent-catalog-card agent-saved-card" role="listitem" key={agent.id}>
              <button
                className="agent-card-main"
                type="button"
                onClick={(event) => onEdit(agent, event.currentTarget)}
                disabled={interactionDisabled || openingAgentId === agent.id}
                aria-label={t("catalog.editLabel", { name: agent.name || t("catalog.untitled"), id: agent.id })}
                data-agent-id={agent.id}
              >
                <span className="agent-card-icon"><Bot size={18} strokeWidth={1.7} aria-hidden="true" /></span>
                <strong>{agent.name || t("catalog.untitled")}</strong>
                <span className="agent-card-description">{agent.instructions || t("catalog.noInstructions")}</span>
                <span className="agent-card-meta">
                  <code>{agent.model}</code>
                  <span>{t("catalog.tools", { count: agent.tools.length, formattedCount: count(agent.tools.length) })}</span>
                  <time dateTime={new Date(agent.updated_at * 1000).toISOString()}>{formatShortDate(agent.updated_at)}</time>
                </span>
                {openingAgentId === agent.id ? <span className="agent-card-opening" role="status">{t("catalog.opening")}</span> : null}
              </button>
              <footer>
                <AgentSessionStartAction
                  agent={agent}
                  busy={interactionDisabled}
                  onStart={onStartSession}
                  vaultCatalog={vaultCatalog}
                />
              </footer>
            </article>
          ))}
        </div>
        {canExpand ? (
          <div className="agent-catalog-more">
            <span>{expanded ? t("catalog.showingAll", { count: agents.length, formattedCount: count(agents.length) }) : t("catalog.more", { count: hiddenAgentCount, formattedCount: count(hiddenAgentCount) })}</span>
            <button
              className="button outline"
              type="button"
              aria-controls={gridId}
              aria-expanded={expanded}
              onClick={() => onExpandedChange(!expanded)}
            >
              {expanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
              {expanded ? t("catalog.showLess") : t("catalog.showMore", { count: hiddenAgentCount, formattedCount: count(hiddenAgentCount) })}
            </button>
          </div>
        ) : null}
        {!agents.length ? (
          <div className="agent-catalog-empty" role="status">
            <strong>{hasSavedAgents ? t("catalog.noMatch") : t("catalog.noSaved")}</strong>
            <span>{hasSavedAgents ? t("catalog.trySearch") : t("catalog.emptyHelp")}</span>
            {hasSavedAgents ? <button className="button outline" type="button" onClick={onClearSearch}>{t("catalog.clearSearch")}</button> : null}
          </div>
        ) : null}
      </section>

      <section className="agent-catalog-section agent-template-section" aria-labelledby="agent-templates-heading">
        <header>
          <div>
            <h2 id="agent-templates-heading">{t("catalog.templates")}</h2>
            <p>{t("catalog.templatesHelp")}</p>
          </div>
        </header>
        <div className="agent-template-grid" role="list" aria-label={t("catalog.templateList")}>
          {AGENT_TEMPLATES.map((template) => (
            <div className="agent-catalog-slot" role="listitem" key={template.id}>
              <button
                className="agent-catalog-card agent-template-card"
                type="button"
                data-agent-template-id={template.id}
                onClick={(event) => onUseTemplate(template, event.currentTarget)}
                disabled={!coreReady || interactionDisabled}
                aria-label={t("catalog.useTemplate", { name: template.name })}
              >
                <span className="agent-card-icon"><PanelsTopLeft size={18} strokeWidth={1.7} aria-hidden="true" /></span>
                <strong>{template.name}</strong>
                <span className="agent-card-description">{template.description}</span>
              </button>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
