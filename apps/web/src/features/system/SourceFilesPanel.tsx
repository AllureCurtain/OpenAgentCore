import { FileUp, FolderInput, RefreshCw, Search, Trash2 } from "lucide-react";
import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { useTranslation } from "react-i18next";

import {
  AgentCoreError,
  type AgentCore,
  type AgentEnvironmentResource,
  type EnvironmentFile,
  type SourceFile,
} from "@agents-core-web/agents-client";

import { formatFileSize } from "../sessions/environment/EnvironmentFilesPanel";
import { isWritableBasicHostedEnvironmentResource } from "../sessions/environment/environment-state";
import "./SourceFilesPanel.css";

export interface SourceFilesOperations {
  uploadSourceFile: AgentCore["uploadSourceFile"];
  retrieveSourceFile: AgentCore["retrieveSourceFile"];
  deleteSourceFile: AgentCore["deleteSourceFile"];
  retrieveEnvironment: AgentCore["retrieveEnvironment"];
  createEnvironmentFile: AgentCore["createEnvironmentFile"];
  listEnvironmentFiles: AgentCore["listEnvironmentFiles"];
}

type Operation = "upload" | "retrieve" | "delete" | "environment" | "copy" | null;
type NoticeTone = "info" | "success" | "error" | "warning";

interface Notice {
  tone: NoticeTone;
  text: string;
}

const maxSourceBytes = 512 * 1024 * 1024;
const maxDestinationBytes = 50 * 1024 * 1024;

function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

function safeFailure(error: unknown, operation: Exclude<Operation, null>, text: (key: string) => string): string {
  if (error instanceof TypeError && !error.message.toLowerCase().includes("fetch")) return error.message;
  if (!(error instanceof AgentCoreError)) return text("sourceFiles.errors.transport");
  if (error.status === 400) return text("sourceFiles.errors.invalid");
  if (error.status === 401) return text("sourceFiles.errors.unauthorized");
  if (error.status === 404) return operation === "environment"
    ? text("sourceFiles.errors.environmentMissing")
    : text("sourceFiles.errors.sourceMissing");
  if (error.status === 409) return text("sourceFiles.errors.conflict");
  if (error.status === 413) return operation === "upload"
    ? text("sourceFiles.errors.sourceTooLarge")
    : text("sourceFiles.errors.destinationTooLarge");
  if (error.status === 503) return operation === "environment"
    ? text("sourceFiles.errors.environmentUnavailable")
    : text("sourceFiles.errors.serviceUnavailable");
  return text("sourceFiles.errors.generic");
}

export function validHostedDestinationPath(value: string): boolean {
  if (
    !value.startsWith("/workspace/") ||
    new TextEncoder().encode(value).length > 4096 ||
    value.includes("\\") ||
    value.includes("\0") ||
    value.includes("\r") ||
    value.includes("\n") ||
    value.split("/").includes("..") ||
    value.split("/").includes(".") ||
    value.includes("//") ||
    value.endsWith("/")
  ) return false;
  return value.length > "/workspace/".length;
}

function parentDirectory(path: string): string {
  return path.slice(0, path.lastIndexOf("/")) || "/workspace";
}

function knownRejected(error: unknown): boolean {
  return error instanceof AgentCoreError && error.status >= 400 && error.status < 500;
}

export function SourceFilesPanel({
  operations,
  environmentFilesEnabled,
}: {
  operations: SourceFilesOperations;
  environmentFilesEnabled: boolean;
}) {
  const { t } = useTranslation("system");
  const text = (key: string) => String(t(key as never));
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [fileInputKey, setFileInputKey] = useState(0);
  const [fileId, setFileId] = useState("");
  const [metadata, setMetadata] = useState<SourceFile | null>(null);
  const [notice, setNotice] = useState<Notice>({
    tone: "info",
    text: t("sourceFiles.initial"),
  });
  const [environmentId, setEnvironmentId] = useState("");
  const [environment, setEnvironment] = useState<AgentEnvironmentResource | null>(null);
  const [destinationPath, setDestinationPath] = useState("/workspace/input/");
  const [copyResult, setCopyResult] = useState<EnvironmentFile | null>(null);
  const [copyUnknown, setCopyUnknown] = useState(false);
  const [operation, setOperation] = useState<Operation>(null);
  const requestRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const busy = operation !== null;
  const currentId = fileId.trim();
  const sourceReady = metadata !== null && metadata.id === currentId;
  const destinationValid = validHostedDestinationPath(destinationPath);
  const hosted = isWritableBasicHostedEnvironmentResource(environment, environmentId.trim())
    ? environment
    : null;

  useEffect(() => () => {
    requestRef.current += 1;
    abortRef.current?.abort();
  }, []);

  const start = (nextOperation: Exclude<Operation, null>) => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const request = requestRef.current + 1;
    requestRef.current = request;
    setOperation(nextOperation);
    return { controller, request };
  };

  const current = (request: number, controller: AbortController) =>
    request === requestRef.current && !controller.signal.aborted;

  const finish = (request: number, controller: AbortController) => {
    if (!current(request, controller)) return false;
    setOperation(null);
    return true;
  };

  const changeFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] ?? null;
    setSelectedFile(file);
    setNotice(file && file.size > maxSourceBytes
      ? { tone: "error", text: t("sourceFiles.notices.tooLarge") }
      : { tone: "info", text: file ? t("sourceFiles.notices.ready", { name: file.name }) : t("sourceFiles.notices.choose") });
  };

  const upload = async () => {
    if (!selectedFile || selectedFile.size > maxSourceBytes) return;
    const file = selectedFile;
    const { controller, request } = start("upload");
    setSelectedFile(null);
    setFileInputKey((value) => value + 1);
    setMetadata(null);
    setCopyResult(null);
    setCopyUnknown(false);
    setNotice({ tone: "info", text: t("sourceFiles.notices.uploading") });
    try {
      const next = await operations.uploadSourceFile(
        { file, filename: file.name },
        { signal: controller.signal },
      );
      if (!finish(request, controller)) return;
      setMetadata(next);
      setFileId(next.id);
      setNotice({ tone: "success", text: t("sourceFiles.notices.uploaded") });
    } catch (error) {
      if (!current(request, controller) || isAbort(error)) return;
      finish(request, controller);
      setNotice(knownRejected(error)
        ? { tone: "error", text: safeFailure(error, "upload", text) }
        : {
            tone: "warning",
            text: t("sourceFiles.notices.uploadUnknown"),
          });
    }
  };

  const retrieve = async () => {
    if (!currentId) return;
    const { controller, request } = start("retrieve");
    setMetadata(null);
    setCopyResult(null);
    setCopyUnknown(false);
    try {
      const next = await operations.retrieveSourceFile(currentId, { signal: controller.signal });
      if (!finish(request, controller)) return;
      setMetadata(next);
      setNotice({ tone: "success", text: t("sourceFiles.notices.retrieved") });
    } catch (error) {
      if (!current(request, controller) || isAbort(error)) return;
      finish(request, controller);
      setNotice({ tone: "error", text: safeFailure(error, "retrieve", text) });
    }
  };

  const deleteFile = async () => {
    if (!currentId) return;
    const id = currentId;
    const { controller, request } = start("delete");
    setNotice({ tone: "info", text: t("sourceFiles.notices.deleting") });
    try {
      await operations.deleteSourceFile(id, { signal: controller.signal });
      if (!finish(request, controller)) return;
      if (metadata?.id === id) setMetadata(null);
      setCopyResult(null);
      setNotice({ tone: "success", text: t("sourceFiles.notices.deleted") });
    } catch (error) {
      if (!current(request, controller) || isAbort(error)) return;
      if (error instanceof AgentCoreError && error.status === 404) {
        finish(request, controller);
        if (metadata?.id === id) setMetadata(null);
        setNotice({ tone: "success", text: t("sourceFiles.notices.absent") });
        return;
      }
      if (knownRejected(error)) {
        finish(request, controller);
        setNotice({ tone: "error", text: safeFailure(error, "delete", text) });
        return;
      }

      const reconcileController = new AbortController();
      abortRef.current = reconcileController;
      try {
        const next = await operations.retrieveSourceFile(id, { signal: reconcileController.signal });
        if (request !== requestRef.current || reconcileController.signal.aborted) return;
        setMetadata(next);
        setNotice({ tone: "warning", text: t("sourceFiles.notices.stillPresent") });
      } catch (reconcileError) {
        if (request !== requestRef.current || reconcileController.signal.aborted || isAbort(reconcileError)) return;
        if (reconcileError instanceof AgentCoreError && reconcileError.status === 404) {
          if (metadata?.id === id) setMetadata(null);
          setNotice({ tone: "success", text: t("sourceFiles.notices.deleteReconciled") });
        } else {
          setNotice({ tone: "warning", text: t("sourceFiles.notices.deleteUnknown") });
        }
      } finally {
        if (request === requestRef.current && !reconcileController.signal.aborted) setOperation(null);
      }
    }
  };

  const inspectEnvironment = async () => {
    const id = environmentId.trim();
    if (!id) return;
    const { controller, request } = start("environment");
    setEnvironment(null);
    setCopyResult(null);
    setCopyUnknown(false);
    try {
      const resource = await operations.retrieveEnvironment(id, { signal: controller.signal });
      if (!finish(request, controller)) return;
      setEnvironment(resource);
      setNotice(resource.type === "openai_hosted"
        ? isWritableBasicHostedEnvironmentResource(resource, id)
          ? {
              tone: "success",
              text: t("sourceFiles.notices.hostedReady"),
            }
          : {
              tone: "warning",
              text: t("sourceFiles.notices.hostedUnsupported"),
            }
        : {
            tone: "info",
            text: t("sourceFiles.notices.selfHosted"),
          });
    } catch (error) {
      if (!current(request, controller) || isAbort(error)) return;
      finish(request, controller);
      setNotice({ tone: "error", text: safeFailure(error, "environment", text) });
    }
  };

  const copyToEnvironment = async () => {
    if (!hosted || !sourceReady || !destinationValid || metadata.bytes > maxDestinationBytes || copyUnknown) return;
    const path = destinationPath;
    const source = metadata;
    const { controller, request } = start("copy");
    setCopyResult(null);
    setNotice({ tone: "info", text: t("sourceFiles.notices.copying") });
    try {
      const result = await operations.createEnvironmentFile(hosted.id, {
        type: "file_id",
        file_id: source.id,
        path,
      }, { signal: controller.signal });
      if (!finish(request, controller)) return;
      setCopyResult(result);
      setNotice({ tone: "success", text: t("sourceFiles.notices.copied") });
    } catch (error) {
      if (!current(request, controller) || isAbort(error)) return;
      if (knownRejected(error)) {
        finish(request, controller);
        setNotice({ tone: "error", text: safeFailure(error, "copy", text) });
        return;
      }

      const reconcileController = new AbortController();
      abortRef.current = reconcileController;
      setCopyUnknown(true);
      try {
        const page = await operations.listEnvironmentFiles(hosted.id, {
          path: parentDirectory(path),
          limit: 100,
          order: "asc",
          signal: reconcileController.signal,
        });
        if (request !== requestRef.current || reconcileController.signal.aborted) return;
        const candidate = page.data.find((file) => file.path === path && file.size_bytes === source.bytes);
        setNotice({
          tone: "warning",
          text: candidate
            ? t("sourceFiles.notices.copyCandidate")
            : t("sourceFiles.notices.copyMissing"),
        });
      } catch (reconcileError) {
        if (request !== requestRef.current || reconcileController.signal.aborted || isAbort(reconcileError)) return;
        setNotice({ tone: "warning", text: t("sourceFiles.notices.copyUnknown") });
      } finally {
        if (request === requestRef.current && !reconcileController.signal.aborted) setOperation(null);
      }
    }
  };

  return (
    <section className="source-files-panel" aria-labelledby="source-files-heading">
      <header>
        <div>
          <FileUp size={15} strokeWidth={1.5} aria-hidden="true" />
          <div><h2 id="source-files-heading">{t("sourceFiles.title")}</h2><p>{t("sourceFiles.subtitle")}</p></div>
        </div>
        <span>{t(environmentFilesEnabled ? "sourceFiles.limits" : "sourceFiles.sourceLimit")}</span>
      </header>

      <div className="source-files-grid">
        <div className="source-files-card">
          <h3>{t("sourceFiles.uploadOnce")}</h3>
          <label className="source-files-file-input">
            <span>{t("sourceFiles.localFile")}</span>
            <input key={fileInputKey} type="file" onChange={changeFile} disabled={busy} />
          </label>
          <button className="button primary" type="button" onClick={() => void upload()} disabled={busy || !selectedFile || selectedFile.size > maxSourceBytes}>
            <FileUp size={13} aria-hidden="true" />{t(operation === "upload" ? "sourceFiles.uploading" : "sourceFiles.upload")}
          </button>
          <p>{t("sourceFiles.uploadBoundary")}</p>
        </div>

        <div className="source-files-card">
          <h3>{t("sourceFiles.operate")}</h3>
          <label>
            <span>{t("sourceFiles.sourceId")}</span>
            <input
              value={fileId}
              onChange={(event) => {
                setFileId(event.target.value);
                setMetadata(null);
                setCopyResult(null);
                setCopyUnknown(false);
              }}
              placeholder="file-…"
              autoComplete="off"
              spellCheck={false}
              disabled={busy}
            />
          </label>
          <div className="source-files-actions">
            <button className="button outline" type="button" onClick={() => void retrieve()} disabled={busy || !currentId}><Search size={12} />{t("sourceFiles.retrieve")}</button>
            <button className="button outline danger" type="button" onClick={() => void deleteFile()} disabled={busy || !currentId}><Trash2 size={12} />{t("sourceFiles.deleteOnce")}</button>
          </div>
          <p>{t("sourceFiles.noDownload")}</p>
          <p>{t("sourceFiles.noList")}</p>
        </div>
      </div>

      <div className={`source-files-notice source-files-notice-${notice.tone}`} role={notice.tone === "error" ? "alert" : "status"} aria-live="polite">
        {operation ? <RefreshCw className="refresh-spinning" size={13} aria-hidden="true" /> : null}<span>{notice.text}</span>
      </div>

      {metadata ? (
        <dl className="source-files-result" aria-label={t("sourceFiles.metadata")}>
          <div><dt>ID</dt><dd><code>{metadata.id}</code></dd></div>
          <div><dt>{t("sourceFiles.filename")}</dt><dd>{metadata.filename}</dd></div>
          <div><dt>{t("sourceFiles.bytes")}</dt><dd>{formatFileSize(metadata.bytes)}</dd></div>
          <div><dt>{t("sourceFiles.status")}</dt><dd>{metadata.status} · {t("sourceFiles.storedOnly")}</dd></div>
        </dl>
      ) : null}

      {environmentFilesEnabled ? <section className="source-files-hosted" aria-labelledby="source-files-hosted-heading">
        <header>
          <div><FolderInput size={14} strokeWidth={1.5} aria-hidden="true" /><h3 id="source-files-hosted-heading">{t("sourceFiles.copyTitle")}</h3></div>
          <p>{t("sourceFiles.copyBoundary")}</p>
        </header>
        <div className="source-files-hosted-check">
          <label>
            <span>{t("sourceFiles.environmentId")}</span>
            <input
              value={environmentId}
              onChange={(event) => {
                setEnvironmentId(event.target.value);
                setEnvironment(null);
                setCopyResult(null);
                setCopyUnknown(false);
              }}
              autoComplete="off"
              spellCheck={false}
              disabled={busy}
            />
          </label>
          <button className="button outline" type="button" onClick={() => void inspectEnvironment()} disabled={busy || !environmentId.trim()}>
            <Search size={12} />{t(operation === "environment" ? "sourceFiles.checking" : "sourceFiles.checkEnvironment")}
          </button>
        </div>

        {hosted ? (
          <div className="source-files-hosted-write">
            <p className="source-files-hosted-qualified"><strong>{t("sourceFiles.qualified")}</strong> <code>{hosted.id}</code> · {hosted.status}</p>
            <label>
              <span>{t("sourceFiles.destination")}</span>
              <input
                value={destinationPath}
                onChange={(event) => {
                  setDestinationPath(event.target.value);
                  setCopyResult(null);
                  setCopyUnknown(false);
                }}
                aria-invalid={!destinationValid}
                placeholder="/workspace/input/notes.txt"
                disabled={busy}
              />
            </label>
            {!destinationValid ? <p className="source-files-validation">{t("sourceFiles.invalidPath")}</p> : null}
            {metadata && metadata.bytes > maxDestinationBytes ? <p className="source-files-validation">{t("sourceFiles.destinationLimit")}</p> : null}
            <button
              className="button primary"
              type="button"
              onClick={() => void copyToEnvironment()}
              disabled={busy || !sourceReady || !destinationValid || metadata.bytes > maxDestinationBytes || copyUnknown}
            >
              <FolderInput size={13} />{t(operation === "copy" ? "sourceFiles.copying" : copyUnknown ? "sourceFiles.outcomeUnknown" : "sourceFiles.copyById")}
            </button>
          </div>
        ) : null}

        {environment?.type === "self_hosted" ? (
          <p className="source-files-readonly">{t("sourceFiles.selfHostedReadonly")}</p>
        ) : null}
        {copyResult ? (
          <p className="source-files-copy-result"><strong>{t("sourceFiles.copyConfirmed")}</strong> <code>{copyResult.path}</code> · {formatFileSize(copyResult.size_bytes)}</p>
        ) : null}
      </section> : null}
    </section>
  );
}
