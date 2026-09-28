import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AgentCoreError,
  createIdempotencyKey,
  type CreateSessionInput,
} from "@agents-core-web/agents-client";
import { useAgents } from "./Agents";
import { api, application } from "./lib/api";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import { Textarea } from "./components/ui/textarea";
import { Select, SelectOption } from "./components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./components/ui/dialog";
import { ErrorNotice, Field, Help } from "./components/shared";

const pendingKey = "oac-example-pending-task";
type PendingTask = { key: string; input: CreateSessionInput };
function pendingTask(): PendingTask | null {
  try {
    return JSON.parse(
      sessionStorage.getItem(pendingKey) || "null",
    ) as PendingTask | null;
  } catch {
    return null;
  }
}

export function NewTask({ close }: { close: () => void }) {
  const agents = useAgents();
  const cache = useQueryClient();
  const [pending, setPending] = useState(pendingTask);
  const [title, setTitle] = useState(pending?.input.metadata?.title || "");
  const [prompt, setPrompt] = useState(
    typeof pending?.input.input === "string" ? pending.input.input : "",
  );
  const [agentId, setAgentId] = useState(pending?.input.agent_id || "");
  const [environment, setEnvironment] = useState<string>(
    pending?.input.environment.type || "openai_hosted",
  );
  const mutation = useMutation({
    mutationFn: async () => {
      const operation = pending || {
        key: createIdempotencyKey(),
        input: {
          agent_id: agentId,
          environment: { type: environment as "none" | "openai_hosted" },
          input: prompt,
          metadata: { application, title: title.trim() },
        },
      };
      sessionStorage.setItem(pendingKey, JSON.stringify(operation));
      setPending(operation);
      return api.createSession(operation.input, operation.key);
    },
    onSuccess: (session) => {
      sessionStorage.removeItem(pendingKey);
      void cache.invalidateQueries({ queryKey: ["tasks"] });
      location.hash = `/tasks/${session.id}`;
      close();
    },
    onError: (error) => {
      if (
        error instanceof AgentCoreError &&
        [400, 401, 403, 404, 413, 422].includes(error.status)
      ) {
        sessionStorage.removeItem(pendingKey);
        setPending(null);
      }
    },
  });
  const rows = agents.data?.pages.flatMap((page) => page.data) || [];
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) close();
      }}
    >
      <DialogContent
        aria-describedby={undefined}
        className="max-h-[90dvh] overflow-y-auto"
      >
        <DialogHeader>
          <DialogTitle>新建任务</DialogTitle>
        </DialogHeader>
        <form
          id="task-form"
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            mutation.mutate();
          }}
        >
          <Field label="任务名称" id="task-title">
            <Input
              id="task-title"
              required
              maxLength={100}
              disabled={!!pending}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </Field>
          <Field label="Agent" id="task-agent">
            <Select
              id="task-agent"
              disabled={!!pending}
              value={agentId}
              onValueChange={setAgentId}
            >
              <SelectOption value="">选择 Agent</SelectOption>
              {rows.map((agent) => (
                <SelectOption key={agent.id} value={agent.id}>
                  {agent.name || agent.model}
                </SelectOption>
              ))}
            </Select>
          </Field>
          <ErrorNotice
            error={agents.error}
            retry={() => void agents.refetch()}
          />
          {!rows.length && !agents.isPending && !agents.error && (
            <a
              href="#/agents"
              className="inline-block underline underline-offset-4"
              onClick={close}
            >
              前往 Agents 配置
            </a>
          )}
          {agents.hasNextPage && (
            <Button
              type="button"
              variant="outline"
              onClick={() => void agents.fetchNextPage()}
            >
              加载更多 Agent
            </Button>
          )}
          <Field label="运行环境" id="task-environment">
            <Select
              id="task-environment"
              disabled={!!pending}
              value={environment}
              onValueChange={setEnvironment}
            >
              <SelectOption value="openai_hosted">Core 托管环境</SelectOption>
              <SelectOption value="none">纯文本环境</SelectOption>
            </Select>
          </Field>
          <Field label="任务内容" id="task-prompt">
            <Textarea
              id="task-prompt"
              rows={5}
              required
              disabled={!!pending}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="希望 Agent 完成什么？"
            />
          </Field>
          <ErrorNotice error={mutation.error} />
          {pending && !mutation.isPending && (
            <p className="text-base">
              请求内容已保留。重试会查询或提交同一次创建，不会新建第二个任务。
            </p>
          )}
        </form>
        <DialogFooter>
          <Help>
            托管环境支持编码和工具，需要 Core
            已配置节点及模型。纯文本环境不提供工作区。任务内容会随创建一起提交。
          </Help>
          <Button
            type="submit"
            form="task-form"
            disabled={
              mutation.isPending || !title.trim() || !prompt.trim() || !agentId
            }
          >
            {mutation.isPending ? "提交中…" : pending ? "重试" : "开始任务"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
