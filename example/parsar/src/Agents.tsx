import { useState } from "react";
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { Bot, Plus } from "lucide-react";
import type {
  CoreHarnessKind,
  SavedAgent,
} from "@agents-core-web/agents-client";
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
import {
  col,
  Ledger,
  LedgerHeader,
  LedgerRow,
  InitialTile,
} from "./components/ui/ledger";
import { EmptyState } from "./components/ui/empty-state";
import { ErrorNotice, Field, Help, PageHeader } from "./components/shared";

export function useAgents() {
  return useInfiniteQuery({
    queryKey: ["agents"],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      api.listAgents({ limit: 100, after: pageParam, signal }),
    getNextPageParam: (page) =>
      page.has_more ? page.last_id || undefined : undefined,
  });
}

export function Agents() {
  const query = useAgents();
  const [editing, setEditing] = useState<SavedAgent | "new" | null>(null);
  const agents = query.data?.pages.flatMap((page) => page.data) || [];
  return (
    <>
      <PageHeader title="Agents">
        <Help>
          配置可重复使用的 Agent。模型和运行引擎需要在 Core 部署中可用。
        </Help>
        <Button onClick={() => setEditing("new")}>
          <Plus />
          新建 Agent
        </Button>
      </PageHeader>
      <ErrorNotice error={query.error} retry={() => void query.refetch()} />
      {query.isPending ? (
        <EmptyState title="正在加载…" />
      ) : agents.length === 0 && !query.error ? (
        <EmptyState icon={Bot} title="还没有 Agent" />
      ) : (
        <Ledger columns={[col.tile(), col.title(), col.text(), col.text()]}>
          <LedgerHeader>
            <span />
            <span>名称</span>
            <span>模型</span>
            <span>运行引擎</span>
          </LedgerHeader>
          <ul role="listbox" aria-label="Agents" className="m-0 list-none p-0">
            {agents.map((agent) => (
              <LedgerRow
                key={agent.id}
                onClick={() => setEditing(agent)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setEditing(agent);
                  }
                }}
              >
                <InitialTile name={agent.name || "A"} />
                <span className="truncate font-medium">
                  {agent.name || "未命名 Agent"}
                </span>
                <span className="truncate">{agent.model}</span>
                <span>{agent.x_agents_core?.harness || "部署默认"}</span>
              </LedgerRow>
            ))}
          </ul>
        </Ledger>
      )}
      {query.hasNextPage && (
        <div className="border-t border-line p-3">
          <Button
            variant="outline"
            disabled={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
          >
            加载更多
          </Button>
        </div>
      )}
      {editing && (
        <AgentEditor
          agent={editing === "new" ? undefined : editing}
          close={() => setEditing(null)}
        />
      )}
    </>
  );
}

function AgentEditor({
  agent,
  close,
}: {
  agent?: SavedAgent;
  close: () => void;
}) {
  const cache = useQueryClient();
  const [name, setName] = useState(agent?.name || "");
  const [model, setModel] = useState(agent?.model || "");
  const [instructions, setInstructions] = useState(agent?.instructions || "");
  const [harness, setHarness] = useState<CoreHarnessKind | "">(
    agent?.x_agents_core?.harness || "",
  );
  const save = useMutation({
    mutationFn: () => {
      const input = {
        name: name.trim(),
        model: model.trim(),
        instructions,
        ...(harness ? { x_agents_core: { harness } } : {}),
      };
      return agent
        ? api.updateAgent(agent.id, input)
        : api.createAgent({
            ...input,
            metadata: { application },
            multi_agent: { enabled: false },
            text: { verbosity: "medium" },
            tools: [{ type: "web_search", mode: "disabled" }],
          });
    },
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: ["agents"] });
      close();
    },
  });
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) close();
      }}
    >
      <DialogContent
        aria-describedby={undefined}
        className="max-h-[90dvh] overflow-y-auto"
      >
        <DialogHeader>
          <DialogTitle>{agent ? "编辑 Agent" : "新建 Agent"}</DialogTitle>
        </DialogHeader>
        <form
          id="agent-form"
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          <Field label="名称" id="agent-name">
            <Input
              id="agent-name"
              required
              maxLength={80}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field label="模型" id="agent-model">
            <Input
              id="agent-model"
              required
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder="输入部署支持的模型名称"
            />
          </Field>
          <Field label="运行引擎" id="agent-harness">
            <Select
              id="agent-harness"
              value={harness}
              onValueChange={(v) => setHarness(v as CoreHarnessKind)}
            >
              <SelectOption value="" disabled={!!agent?.x_agents_core?.harness}>
                部署默认
              </SelectOption>
              <SelectOption value="codex">Codex</SelectOption>
              <SelectOption value="claude_sdk">Claude Code</SelectOption>
              <SelectOption value="mcode">MiniMax Code</SelectOption>
            </Select>
          </Field>
          <Field label="指令" id="agent-instructions">
            <Textarea
              id="agent-instructions"
              rows={5}
              value={instructions}
              onChange={(e) => setInstructions(e.target.value)}
            />
          </Field>
          <ErrorNotice error={save.error} />
        </form>
        <DialogFooter>
          <Help>
            保存不会启动执行。修改只影响新任务，已有任务继续使用创建时的配置。模型凭据由
            Core 部署提供。
          </Help>
          <Button
            type="submit"
            form="agent-form"
            disabled={save.isPending || !name.trim() || !model.trim()}
          >
            {save.isPending ? "保存中…" : "保存"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
