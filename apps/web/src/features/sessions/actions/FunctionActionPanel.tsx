import {
  ArrowDown,
  ArrowUp,
  Code2,
  Plus,
  Square,
  Trash2,
} from "lucide-react";
import { useEffect, useReducer, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  FunctionCallAction,
  FunctionResultContent,
  FunctionResultInput,
} from "@agents-core-web/agents-client";

import "./FunctionActionPanel.css";

export type FunctionResultMode = "text" | "structured";

export interface FunctionResultPartDraft {
  id: string;
  type: FunctionResultContent["type"];
  value: string;
}

export interface FunctionResultDraft {
  mode: FunctionResultMode;
  text: string;
  parts: FunctionResultPartDraft[];
}

export type FunctionResultDraftAction =
  | { type: "set-mode"; mode: FunctionResultMode }
  | { type: "set-text"; value: string }
  | { type: "add-part"; part: FunctionResultPartDraft }
  | { type: "update-part"; id: string; value: string }
  | { type: "remove-part"; id: string }
  | { type: "move-part"; id: string; direction: -1 | 1 };

export interface FunctionActionPanelProps {
  actions: FunctionCallAction[];
  agentName: string;
  autoFocus: boolean;
  busy: boolean;
  onCancel: () => void;
  onSubmit: (input: FunctionResultInput) => Promise<void>;
}

export const defaultFunctionRejection = "Function rejected by the operator.";

export function createFunctionResultDraft(): FunctionResultDraft {
  return { mode: "text", text: "", parts: [] };
}

export function functionResultDraftReducer(
  draft: FunctionResultDraft,
  action: FunctionResultDraftAction,
): FunctionResultDraft {
  if (action.type === "set-mode") return { ...draft, mode: action.mode };
  if (action.type === "set-text") return { ...draft, text: action.value };
  if (action.type === "add-part") {
    if (draft.parts.some((part) => part.id === action.part.id)) return draft;
    return { ...draft, parts: [...draft.parts, action.part] };
  }
  if (action.type === "update-part") {
    return {
      ...draft,
      parts: draft.parts.map((part) => (
        part.id === action.id ? { ...part, value: action.value } : part
      )),
    };
  }
  if (action.type === "remove-part") {
    return { ...draft, parts: draft.parts.filter((part) => part.id !== action.id) };
  }

  const index = draft.parts.findIndex((part) => part.id === action.id);
  const destination = index + action.direction;
  if (index < 0 || destination < 0 || destination >= draft.parts.length) return draft;
  const parts = [...draft.parts];
  const sourcePart = parts[index];
  const destinationPart = parts[destination];
  if (!sourcePart || !destinationPart) return draft;
  parts[index] = destinationPart;
  parts[destination] = sourcePart;
  return { ...draft, parts };
}

export function validFunctionResultDraft(draft: FunctionResultDraft): boolean {
  if (draft.mode === "text") return Boolean(draft.text.trim());
  return draft.parts.length > 0 && draft.parts.every((part) => Boolean(part.value.trim()));
}

export function buildFunctionResultInput(
  action: FunctionCallAction,
  draft: FunctionResultDraft,
  success: boolean,
): FunctionResultInput | null {
  const identity = { callId: action.call_id, turnId: action.turn_id };
  if (!success) {
    return {
      ...identity,
      success: false,
      error: draft.text || defaultFunctionRejection,
    };
  }
  if (!validFunctionResultDraft(draft)) return null;
  if (draft.mode === "text") {
    return { ...identity, success: true, output: draft.text };
  }

  const output: FunctionResultContent[] = draft.parts.map((part) => (
    part.type === "input_text"
      ? { type: "input_text", text: part.value }
      : { type: "input_image", image_url: part.value }
  ));
  return { ...identity, success: true, output };
}

export async function submitFunctionResultDraft(
  action: FunctionCallAction,
  draft: FunctionResultDraft,
  success: boolean,
  onSubmit: (input: FunctionResultInput) => Promise<void>,
): Promise<boolean> {
  const input = buildFunctionResultInput(action, draft, success);
  if (!input) return false;
  await onSubmit(input);
  return true;
}

function pretty(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function FunctionActionEditor({
  action,
  actionCount,
  agentName,
  autoFocus,
  busy,
  onCancel,
  onSubmit,
}: Omit<FunctionActionPanelProps, "actions"> & {
  action: FunctionCallAction;
  actionCount: number;
}) {
  const { t } = useTranslation("sessions");
  const [draft, dispatch] = useReducer(
    functionResultDraftReducer,
    undefined,
    createFunctionResultDraft,
  );
  const [submitting, setSubmitting] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const nextPartId = useRef(0);
  const mounted = useRef(true);

  useEffect(() => () => {
    mounted.current = false;
  }, []);

  useEffect(() => {
    if (autoFocus) inputRef.current?.focus({ preventScroll: true });
  }, [autoFocus]);

  const disabled = busy || submitting;
  const addPart = (type: FunctionResultContent["type"]) => {
    nextPartId.current += 1;
    dispatch({
      type: "add-part",
      part: { id: `function-result-part-${nextPartId.current}`, type, value: "" },
    });
  };

  const submit = (success: boolean) => {
    if (disabled) return;
    if (success && !validFunctionResultDraft(draft)) return;
    setSubmitting(true);
    void submitFunctionResultDraft(action, draft, success, onSubmit)
      .catch(() => undefined)
      .finally(() => {
        if (mounted.current) setSubmitting(false);
      });
  };

  return (
    <section className="approval-bar function-action-panel" aria-label={t("functionAction.required")}>
      <div className="approval-heading">
        <Code2 size={14} strokeWidth={1.5} aria-hidden="true" />
        <span title={action.name}>{action.name}</span>
        {actionCount > 1 ? <span className="approval-count">1 / {actionCount}</span> : null}
      </div>
      <p>{t("functionAction.waiting", { agent: agentName || t("common.agent") })}</p>
      <pre className="approval-arguments">{pretty(action.arguments)}</pre>

      <fieldset className="function-result-mode" disabled={disabled}>
        <legend>{t("functionAction.resultFormat")}</legend>
        <label>
          <input
            type="radio"
            name={`function-result-mode-${action.call_id}`}
            value="text"
            checked={draft.mode === "text"}
            onChange={() => dispatch({ type: "set-mode", mode: "text" })}
          />
          {t("functionAction.textResult")}
        </label>
        <label>
          <input
            type="radio"
            name={`function-result-mode-${action.call_id}`}
            value="structured"
            checked={draft.mode === "structured"}
            onChange={() => dispatch({ type: "set-mode", mode: "structured" })}
          />
          {t("functionAction.structuredResult")}
        </label>
      </fieldset>

      {draft.mode === "text" ? (
        <textarea
          ref={inputRef}
          className="approval-input"
          value={draft.text}
          onChange={(event) => dispatch({ type: "set-text", value: event.target.value })}
          placeholder={t("functionAction.resultPlaceholder")}
          aria-label={t("functionAction.resultOrError")}
          rows={3}
          disabled={disabled}
        />
      ) : (
        <div className="function-result-builder">
          <div className="function-result-builder-heading">
            <div>
              <strong>{t("functionAction.orderedParts")}</strong>
              <span>{t("functionAction.orderedPartsHint")}</span>
            </div>
            <div className="function-result-add-actions">
              <button className="button outline" type="button" disabled={disabled} onClick={() => addPart("input_text")}>
                <Plus size={13} aria-hidden="true" /> {t("functionAction.textPart")}
              </button>
              <button className="button outline" type="button" disabled={disabled} onClick={() => addPart("input_image")}>
                <Plus size={13} aria-hidden="true" /> {t("functionAction.imagePart")}
              </button>
            </div>
          </div>

          {draft.parts.length === 0 ? (
            <p className="function-result-empty" role="note">{t("functionAction.addPartHint")}</p>
          ) : (
            <ol className="function-result-parts">
              {draft.parts.map((part, index) => (
                <li className="function-result-part" key={part.id}>
                  <div className="function-result-part-heading">
                    <span>{t(part.type === "input_text" ? "functionAction.numberedTextPart" : "functionAction.numberedImagePart", { number: index + 1 })}</span>
                    <div className="function-result-part-actions">
                      <button
                        className="icon-button ghost"
                        type="button"
                        aria-label={t("functionAction.movePartUp", { number: index + 1 })}
                        title={t("common.moveUp")}
                        disabled={disabled || index === 0}
                        onClick={() => dispatch({ type: "move-part", id: part.id, direction: -1 })}
                      >
                        <ArrowUp size={13} aria-hidden="true" />
                      </button>
                      <button
                        className="icon-button ghost"
                        type="button"
                        aria-label={t("functionAction.movePartDown", { number: index + 1 })}
                        title={t("common.moveDown")}
                        disabled={disabled || index === draft.parts.length - 1}
                        onClick={() => dispatch({ type: "move-part", id: part.id, direction: 1 })}
                      >
                        <ArrowDown size={13} aria-hidden="true" />
                      </button>
                      <button
                        className="icon-button ghost"
                        type="button"
                        aria-label={t("functionAction.deletePart", { number: index + 1 })}
                        title={t("functionAction.deletePartTitle")}
                        disabled={disabled}
                        onClick={() => dispatch({ type: "remove-part", id: part.id })}
                      >
                        <Trash2 size={13} aria-hidden="true" />
                      </button>
                    </div>
                  </div>
                  {part.type === "input_text" ? (
                    <textarea
                      className="approval-input function-result-part-input"
                      value={part.value}
                      onChange={(event) => dispatch({ type: "update-part", id: part.id, value: event.target.value })}
                      aria-label={t("functionAction.numberedTextPart", { number: index + 1 })}
                      aria-invalid={!part.value.trim()}
                      placeholder={t("functionAction.textContent")}
                      rows={2}
                      disabled={disabled}
                    />
                  ) : (
                    <input
                      className="function-result-image-input"
                      type="text"
                      value={part.value}
                      onChange={(event) => dispatch({ type: "update-part", id: part.id, value: event.target.value })}
                      aria-label={t("functionAction.imageUrlForPart", { number: index + 1 })}
                      aria-invalid={!part.value.trim()}
                      placeholder="https://… or data:image/…"
                      disabled={disabled}
                    />
                  )}
                </li>
              ))}
            </ol>
          )}

          <p className="function-result-image-note" role="note">
            {t("functionAction.imageNote")}
          </p>
          <label className="function-result-error-field">
            <span>{t("functionAction.errorDetail")}</span>
            <textarea
              className="approval-input"
              value={draft.text}
              onChange={(event) => dispatch({ type: "set-text", value: event.target.value })}
              placeholder={t("functionAction.defaultRejection")}
              aria-label={t("functionAction.errorDetailLabel")}
              rows={2}
              disabled={disabled}
            />
          </label>
        </div>
      )}

      <div className="approval-actions">
        <button className="button outline" type="button" disabled={disabled} onClick={() => submit(false)}>
          {t("functionAction.returnError")}
        </button>
        <button
          className="button primary"
          type="button"
          disabled={disabled || !validFunctionResultDraft(draft)}
          onClick={() => submit(true)}
        >
          {t("functionAction.submitResult")}
        </button>
        <button
          className="composer-action"
          type="button"
          onClick={onCancel}
          disabled={disabled}
          aria-label={t("common.cancelActiveTurn")}
          title={t("common.cancelActiveTurn")}
        >
          <Square size={13} fill="currentColor" strokeWidth={1.5} aria-hidden="true" />
        </button>
      </div>
    </section>
  );
}

export function FunctionActionPanel(props: FunctionActionPanelProps) {
  const action = props.actions[0];
  if (!action) return null;
  const actionKey = JSON.stringify([action.turn_id, action.call_id]);
  return (
    <FunctionActionEditor
      key={actionKey}
      action={action}
      actionCount={props.actions.length}
      agentName={props.agentName}
      autoFocus={props.autoFocus}
      busy={props.busy}
      onCancel={props.onCancel}
      onSubmit={props.onSubmit}
    />
  );
}
