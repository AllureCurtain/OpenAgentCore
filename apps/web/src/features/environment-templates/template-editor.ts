import { AgentCoreError, type EnvironmentTemplate, type OpenAIHostedNetworkAccess, type UpdateEnvironmentTemplateInput } from "@agents-core-web/agents-client";
import i18n from "../../i18n";

const tt = (key: string) => i18n.t(key as never, { ns: "templates" });

export interface TemplateDraft { name: string; access: OpenAIHostedNetworkAccess }

export function templatePatch(template: EnvironmentTemplate, draft: TemplateDraft): UpdateEnvironmentTemplateInput {
  const name = draft.name.trim() || null;
  if ([...(name ?? "")].length > 256) throw new Error(tt("form.nameTooLong"));
  const patch: UpdateEnvironmentTemplateInput = {};
  if (draft.name !== (template.name ?? "")) patch.name = name;
  if (draft.access !== template.network.access) {
    if (template.network.allowed_domains.length !== 0) {
      throw new Error(tt("errors.policyPreserved"));
    }
    patch.network = { access: draft.access };
  }
  return patch;
}

export function templateName(template: EnvironmentTemplate): string {
  return template.name?.trim() || tt("unnamed");
}

export function templateFailure(error: unknown): string {
  if (error instanceof AgentCoreError) {
    if (error.status === 401 || error.status === 403) return tt("errors.forbidden");
    if (error.status === 404) return tt("errors.missing");
    if (error.status === 400 || error.status === 422) return tt("errors.invalid");
  }
  return tt("errors.uncertain");
}

export type TemplateWriteResult =
  | { kind: "ignored" }
  | { kind: "saved" | "saved-refresh-failed" }
  | { kind: "failed"; message: string };

/** One write at a time; disposal fences both late UI updates and catalog refreshes. */
export function createTemplateWriteScope() {
  let active = true;
  let pending = false;
  let controller: AbortController | null = null;
  return {
    dispose() { active = false; controller?.abort(); },
    async run(write: (signal: AbortSignal) => Promise<unknown>, refresh: () => Promise<void>): Promise<TemplateWriteResult> {
      if (!active || pending) return { kind: "ignored" };
      pending = true;
      controller = new AbortController();
      try {
        await write(controller.signal);
        if (!active) return { kind: "ignored" };
        try {
          await refresh();
          return active ? { kind: "saved" } : { kind: "ignored" };
        } catch {
          return active ? { kind: "saved-refresh-failed" } : { kind: "ignored" };
        }
      } catch (error) {
        return active ? { kind: "failed", message: templateFailure(error) } : { kind: "ignored" };
      } finally { pending = false; controller = null; }
    },
  };
}
