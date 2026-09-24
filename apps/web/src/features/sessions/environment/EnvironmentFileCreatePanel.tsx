import { FilePlus2 } from "lucide-react";
import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useTranslation } from "react-i18next";

import type { AgentCore, EnvironmentFile } from "@agents-core-web/agents-client";

import {
  createEnvironmentFileCreateGate,
  maxInlineEnvironmentFileBytes,
  submitInlineEnvironmentFile,
  validInlineEnvironmentFilePath,
} from "./environment-file-create";
import "./EnvironmentFileCreatePanel.css";

interface SelectedFileInfo {
  name: string;
  size: number;
}

export interface EnvironmentFileCreatePanelProps {
  environmentId: string;
  workspaceDirectory: string;
  onCreateFile: AgentCore["createEnvironmentFile"];
}

function destinationPlaceholder(workspaceDirectory: string): string {
  const root = workspaceDirectory.replace(/\/+$/u, "");
  return root === "/workspace" || root.startsWith("/workspace/")
    ? `${root}/input.txt`
    : "/workspace/input.txt";
}

export function EnvironmentFileCreatePanel({
  environmentId,
  workspaceDirectory,
  onCreateFile,
}: EnvironmentFileCreatePanelProps) {
  const { t, i18n } = useTranslation("sessions");
  const locale = i18n.resolvedLanguage || "en";
  const fileRef = useRef<File | null>(null);
  const gateRef = useRef(createEnvironmentFileCreateGate());
  const generationRef = useRef(0);
  const [selectedFile, setSelectedFile] = useState<SelectedFileInfo | null>(null);
  const [fileInputKey, setFileInputKey] = useState(0);
  const [path, setPath] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [createdFile, setCreatedFile] = useState<EnvironmentFile | null>(null);
  const pathValid = validInlineEnvironmentFilePath(path);
  const sizeValid = selectedFile === null || selectedFile.size <= maxInlineEnvironmentFileBytes;

  useEffect(() => {
    generationRef.current += 1;
    fileRef.current = null;
    setSelectedFile(null);
    setFileInputKey((value) => value + 1);
    setPath("");
    setSubmitting(false);
    setMessage(null);
    setCreatedFile(null);
  }, [environmentId, workspaceDirectory]);

  const chooseFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] ?? null;
    fileRef.current = file;
    setSelectedFile(file ? { name: file.name, size: file.size } : null);
    setMessage(null);
    setCreatedFile(null);
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (gateRef.current.pending) return;

    const generation = generationRef.current;
    const draft = { file: fileRef.current, path };
    setSubmitting(true);
    setMessage(null);
    setCreatedFile(null);
    const result = await submitInlineEnvironmentFile(
      gateRef.current,
      environmentId,
      draft,
      onCreateFile,
    );
    if (generation !== generationRef.current || result.kind === "ignored") return;

    setSubmitting(false);
    if (result.kind === "success") {
      fileRef.current = null;
      setSelectedFile(null);
      setFileInputKey((value) => value + 1);
      setPath("");
      setCreatedFile(result.file);
      setMessage(null);
      return;
    }
    setMessage(result.kind === "read_failure"
      ? t("fileCreate.readFailure")
      : result.kind === "request_failure"
        ? t("fileCreate.requestFailure")
        : result.message === "Choose one local file before writing."
          ? t("fileCreate.chooseFile")
          : result.message.includes("canonical absolute")
            ? t("fileCreate.canonicalPath")
            : t("fileCreate.sizeLimit"));
  };

  return (
    <section className="environment-file-create" aria-labelledby="environment-file-create-heading">
      <header>
        <FilePlus2 size={14} strokeWidth={1.5} aria-hidden="true" />
        <div>
          <strong id="environment-file-create-heading">{t("fileCreate.title")}</strong>
          <small>{t("fileCreate.subtitle")}</small>
        </div>
      </header>

      <form onSubmit={(event) => void submit(event)} noValidate>
        <label>
          <span>{t("fileCreate.localFile")}</span>
          <input
            key={fileInputKey}
            type="file"
            onChange={chooseFile}
            disabled={submitting}
          />
        </label>
        {selectedFile ? (
          <p className="environment-file-create-selection">
            <strong>{selectedFile.name}</strong> · {t("fileCreate.bytesSelected", { bytes: selectedFile.size.toLocaleString(locale) })}
          </p>
        ) : null}
        {!sizeValid ? (
          <p className="environment-file-create-validation">{t("fileCreate.tooLarge")}</p>
        ) : null}

        <label>
          <span>{t("fileCreate.destinationPath")}</span>
          <input
            type="text"
            value={path}
            onChange={(event) => {
              setPath(event.target.value);
              setMessage(null);
              setCreatedFile(null);
            }}
            placeholder={destinationPlaceholder(workspaceDirectory)}
            aria-invalid={path.length > 0 && !pathValid}
            disabled={submitting}
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        {path.length > 0 && !pathValid ? (
          <p className="environment-file-create-validation">
            {t("fileCreate.pathValidationPrefix")} <code>/workspace/</code>{t("fileCreate.pathValidationSuffix")}
          </p>
        ) : null}

        {message ? <p className="environment-file-create-error" role="alert">{message}</p> : null}
        {createdFile ? (
          <p className="environment-file-create-success" role="status">
            <strong>{t("fileCreate.writeConfirmed")}</strong> <code>{createdFile.path}</code> · {t("fileCreate.bytes", { bytes: createdFile.size_bytes.toLocaleString(locale) })}
          </p>
        ) : null}

        <button
          className="button primary"
          type="submit"
          disabled={submitting || !selectedFile || !sizeValid || !pathValid}
        >
          <FilePlus2 size={13} aria-hidden="true" />
          {submitting ? t("fileCreate.writing") : t("fileCreate.writeSelected")}
        </button>
      </form>

      <p className="environment-file-create-boundary">
        {t("fileCreate.boundary")}
      </p>
    </section>
  );
}
