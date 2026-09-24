import {
  Bot,
  ChevronDown,
  MessageSquare,
  Plus,
} from "lucide-react";
import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";

interface CreateMenuActions {
  canCreateAgent: boolean;
  canStartSession: boolean;
  onCreateAgent: () => void;
  onStartSession: () => void;
}

export function CreateMenuContent({
  canCreateAgent,
  canStartSession,
  onCreateAgent,
  onStartSession,
}: CreateMenuActions) {
  const { t } = useTranslation();
  return (
    <div className="create-menu-panel" role="menu" aria-label={t("actions.create")}>
      <button type="button" role="menuitem" onClick={onCreateAgent} disabled={!canCreateAgent}>
        <Bot size={16} strokeWidth={1.5} aria-hidden="true" />
        <span><strong>{t("createMenu.agent")}</strong><small>{t("createMenu.agentDescription")}</small></span>
      </button>
      <button type="button" role="menuitem" onClick={onStartSession} disabled={!canStartSession}>
        <MessageSquare size={16} strokeWidth={1.5} aria-hidden="true" />
        <span>
          <strong>{t("createMenu.session")}</strong>
          <small>{canStartSession ? t("createMenu.sessionDescription") : t("createMenu.sessionDisabledDescription")}</small>
        </span>
      </button>
    </div>
  );
}

export function CreateMenu(actions: CreateMenuActions) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsidePointer = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", closeOnOutsidePointer);
    return () => document.removeEventListener("mousedown", closeOnOutsidePointer);
  }, [open]);

  const focusMenuItem = (edge: "first" | "last") => {
    window.requestAnimationFrame(() => {
      const items = rootRef.current?.querySelectorAll<HTMLButtonElement>('button[role="menuitem"]:not([disabled])');
      const item = edge === "first" ? items?.[0] : items?.[items.length - 1];
      item?.focus();
    });
  };

  const onMenuKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      setOpen(false);
      triggerRef.current?.focus();
      return;
    }
    if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
    const items = Array.from(
      rootRef.current?.querySelectorAll<HTMLButtonElement>('button[role="menuitem"]:not([disabled])') ?? [],
    );
    if (!items.length) return;
    event.preventDefault();
    if (event.key === "Home") return void items[0]?.focus();
    if (event.key === "End") return void items[items.length - 1]?.focus();
    const current = items.indexOf(document.activeElement as HTMLButtonElement);
    const delta = event.key === "ArrowDown" ? 1 : -1;
    items[(current + delta + items.length) % items.length]?.focus();
  };

  return (
    <div className="create-menu" ref={rootRef} onKeyDown={onMenuKeyDown}>
      <button
        ref={triggerRef}
        className="button primary create-menu-trigger"
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={(event) => {
          const nextOpen = !open;
          setOpen(nextOpen);
          if (nextOpen && event.detail === 0) focusMenuItem("first");
        }}
        onKeyDown={(event) => {
          if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
          event.preventDefault();
          setOpen(true);
          focusMenuItem(event.key === "ArrowDown" ? "first" : "last");
        }}
      >
        <Plus size={14} strokeWidth={1.8} aria-hidden="true" />
        {t("actions.create")}
        <ChevronDown size={13} strokeWidth={1.5} aria-hidden="true" />
      </button>
      {open ? (
        <CreateMenuContent
          {...actions}
          onCreateAgent={() => {
            setOpen(false);
            triggerRef.current?.focus();
            actions.onCreateAgent();
          }}
          onStartSession={() => {
            setOpen(false);
            triggerRef.current?.focus();
            actions.onStartSession();
          }}
        />
      ) : null}
    </div>
  );
}
