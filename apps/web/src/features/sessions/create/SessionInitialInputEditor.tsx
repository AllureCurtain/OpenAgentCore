import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { useId, useRef } from "react";
import { useTranslation } from "react-i18next";

import {
  projectSessionInitialInput,
  sessionInitialInputDraftReducer,
  type SessionInitialInputDraft,
  type SessionInitialInputDraftAction,
} from "./session-initial-input";

import "./SessionInitialInputEditor.css";

export interface SessionInitialInputEditorProps {
  draft: SessionInitialInputDraft;
  disabled?: boolean;
  required?: boolean;
  showTextField?: boolean;
  onChange: (draft: SessionInitialInputDraft) => void;
}

export function SessionInitialInputEditor({
  draft,
  disabled = false,
  required = false,
  showTextField = true,
  onChange,
}: SessionInitialInputEditorProps) {
  const { t } = useTranslation("sessions");
  const id = useId();
  const nextIdRef = useRef(0);
  const projection = projectSessionInitialInput(draft);

  const nextId = (kind: "message" | "part") => {
    nextIdRef.current += 1;
    return `${id}-${kind}-${nextIdRef.current}`;
  };
  const dispatch = (action: SessionInitialInputDraftAction) => {
    onChange(sessionInitialInputDraftReducer(draft, action));
  };
  const selectMessages = () => {
    dispatch({
      type: "set-mode",
      mode: "messages",
      seed: { messageId: nextId("message"), partId: nextId("part") },
    });
  };
  const addMessage = () => {
    dispatch({
      type: "add-message",
      message: {
        id: nextId("message"),
        parts: [{ id: nextId("part"), text: "" }],
      },
    });
  };

  return (
    <section className="session-initial-input-editor" aria-labelledby={`${id}-heading`}>
      <div className="session-initial-input-heading">
        <div>
          <span id={`${id}-heading`}>{t("initialInput.title")}</span>
          <small>{t("initialInput.description")}</small>
        </div>
      </div>

      <fieldset className="session-initial-input-modes" disabled={disabled}>
        <legend>{t("initialInput.format")}</legend>
        <label>
          <input
            type="radio"
            name={`${id}-mode`}
            value="text"
            checked={draft.mode === "text"}
            onChange={() => dispatch({ type: "set-mode", mode: "text" })}
          />
          {t("initialInput.text")}
        </label>
        <label>
          <input
            type="radio"
            name={`${id}-mode`}
            value="messages"
            checked={draft.mode === "messages"}
            onChange={selectMessages}
          />
          {t("initialInput.messageArray")}
        </label>
      </fieldset>

      <p className="session-initial-input-switch-note" role="note">
        {t("initialInput.switchNote")}
      </p>

      {draft.mode === "text" && showTextField ? (
        <label className="field session-initial-text-field">
          <span>{t("initialInput.firstUserMessage")}</span>
          <textarea
            aria-label={t("initialInput.title")}
            value={draft.text}
            onChange={(event) => dispatch({ type: "set-text", value: event.target.value })}
            rows={5}
            placeholder={required ? t("initialInput.requiredPlaceholder") : t("initialInput.optionalPlaceholder")}
            aria-required={required}
            disabled={disabled}
          />
          <small>{required ? t("initialInput.requiredPrefix") : t("initialInput.optionalPrefix")}{t("initialInput.preservationNote")}</small>
        </label>
      ) : draft.mode === "text" ? (
        <p className="session-initial-input-switch-note">
          {t("initialInput.basicFieldActive")}
        </p>
      ) : (
        <div className="session-initial-message-builder">
          <div className="session-initial-message-builder-heading">
            <div>
              <strong>{t("initialInput.orderedMessages")}</strong>
              <span>{t("initialInput.orderedMessagesHint")}</span>
            </div>
            <button
              className="button outline"
              type="button"
              onClick={addMessage}
              disabled={disabled}
            >
              <Plus size={13} aria-hidden="true" />
              {t("initialInput.addMessage")}
            </button>
          </div>

          <ol className="session-initial-messages">
            {draft.messages.map((message, messageIndex) => (
              <li className="session-initial-message" key={message.id}>
                <div className="session-initial-message-heading">
                  <strong>{t("initialInput.userMessageNumber", { number: messageIndex + 1 })}</strong>
                  <div className="session-initial-editor-actions">
                    <button
                      className="icon-button"
                      type="button"
                      aria-label={t("initialInput.moveMessageUp", { number: messageIndex + 1 })}
                      title={t("initialInput.moveMessageUpTitle")}
                      disabled={disabled || messageIndex === 0}
                      onClick={() => dispatch({
                        type: "move-message",
                        messageId: message.id,
                        direction: -1,
                      })}
                    >
                      <ArrowUp size={14} aria-hidden="true" />
                    </button>
                    <button
                      className="icon-button"
                      type="button"
                      aria-label={t("initialInput.moveMessageDown", { number: messageIndex + 1 })}
                      title={t("initialInput.moveMessageDownTitle")}
                      disabled={disabled || messageIndex === draft.messages.length - 1}
                      onClick={() => dispatch({
                        type: "move-message",
                        messageId: message.id,
                        direction: 1,
                      })}
                    >
                      <ArrowDown size={14} aria-hidden="true" />
                    </button>
                    <button
                      className="icon-button danger"
                      type="button"
                      aria-label={t("initialInput.removeMessage", { number: messageIndex + 1 })}
                      title={t("initialInput.removeMessageTitle")}
                      disabled={disabled || draft.messages.length === 1}
                      onClick={() => dispatch({
                        type: "remove-message",
                        messageId: message.id,
                      })}
                    >
                      <Trash2 size={14} aria-hidden="true" />
                    </button>
                  </div>
                </div>

                <ol className="session-initial-parts">
                  {message.parts.map((part, partIndex) => (
                    <li className="session-initial-part" key={part.id}>
                      <div className="session-initial-part-heading">
                        <label htmlFor={`${id}-${part.id}`}>{t("initialInput.textPartNumber", { number: partIndex + 1 })}</label>
                        <div className="session-initial-editor-actions">
                          <button
                            className="icon-button"
                            type="button"
                            aria-label={t("initialInput.moveTextPartUp", { part: partIndex + 1, message: messageIndex + 1 })}
                            title={t("initialInput.moveTextPartUpTitle")}
                            disabled={disabled || partIndex === 0}
                            onClick={() => dispatch({
                              type: "move-part",
                              messageId: message.id,
                              partId: part.id,
                              direction: -1,
                            })}
                          >
                            <ArrowUp size={14} aria-hidden="true" />
                          </button>
                          <button
                            className="icon-button"
                            type="button"
                            aria-label={t("initialInput.moveTextPartDown", { part: partIndex + 1, message: messageIndex + 1 })}
                            title={t("initialInput.moveTextPartDownTitle")}
                            disabled={disabled || partIndex === message.parts.length - 1}
                            onClick={() => dispatch({
                              type: "move-part",
                              messageId: message.id,
                              partId: part.id,
                              direction: 1,
                            })}
                          >
                            <ArrowDown size={14} aria-hidden="true" />
                          </button>
                          <button
                            className="icon-button danger"
                            type="button"
                            aria-label={t("initialInput.removeTextPart", { part: partIndex + 1, message: messageIndex + 1 })}
                            title={t("initialInput.removeTextPartTitle")}
                            disabled={disabled || message.parts.length === 1}
                            onClick={() => dispatch({
                              type: "remove-part",
                              messageId: message.id,
                              partId: part.id,
                            })}
                          >
                            <Trash2 size={14} aria-hidden="true" />
                          </button>
                        </div>
                      </div>
                      <textarea
                        id={`${id}-${part.id}`}
                        value={part.text}
                        onChange={(event) => dispatch({
                          type: "update-part",
                          messageId: message.id,
                          partId: part.id,
                          value: event.target.value,
                        })}
                        rows={3}
                        placeholder={t("initialInput.requiredUserText")}
                        aria-invalid={!message.parts.some((candidate) => candidate.text.match(/[^\p{White_Space}]/u))}
                        disabled={disabled}
                      />
                    </li>
                  ))}
                </ol>

                <button
                  className="button outline session-initial-add-part"
                  type="button"
                  disabled={disabled}
                  onClick={() => dispatch({
                    type: "add-part",
                    messageId: message.id,
                    part: { id: nextId("part"), text: "" },
                  })}
                >
                  <Plus size={13} aria-hidden="true" />
                  {t("initialInput.addTextPart")}
                </button>
              </li>
            ))}
          </ol>

          {"error" in projection ? (
            <small className="field-error" role="alert">{projection.error}</small>
          ) : null}
        </div>
      )}
    </section>
  );
}
