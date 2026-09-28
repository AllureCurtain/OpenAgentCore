import { useState } from "react";
import { Bot, Plus } from "lucide-react";
import { useInstances, useRuntimes } from "./lib/product";
import { InstanceEditor } from "./InstanceEditor";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import { ErrorNotice, Help, PageHeader } from "./components/shared";
import { EmptyState } from "./components/ui/empty-state";
export function Agents() {
  const query = useInstances();
  const runtimes = useRuntimes();
  const [creating, setCreating] = useState(false);
  const [search, setSearch] = useState("");
  const rows =
    query.data?.filter((agent) =>
      `${agent.name} ${agent.model}`
        .toLowerCase()
        .includes(search.toLowerCase()),
    ) || [];
  return (
    <>
      <PageHeader title="Agents">
        <Help>
          每个 Agent 拥有独立的模型、Skills、MCP
          和运行时绑定。从模板创建后，可继续单独调整配置。
        </Help>
        <Button onClick={() => setCreating(true)}>
          <Plus />
          创建 Agent
        </Button>
      </PageHeader>
      <ErrorNotice error={query.error || runtimes.error} />
      <div className="px-6 py-4">
        <Input
          aria-label="搜索 Agents"
          placeholder="搜索 Agent"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-6 pb-6">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {rows.map((agent) => (
            <a
              href={`#/agents/${agent.id}`}
              key={agent.id}
              className="rounded-xl border border-line p-5 transition-colors hover:bg-surface-subtle focus-visible:ring-2 focus-visible:ring-accent"
            >
              <div className="flex items-center gap-3">
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-surface-muted">
                  <Bot className="h-6 w-6" />
                </div>
                <h2 className="truncate text-lg font-semibold">{agent.name}</h2>
              </div>
              <p className="mt-5 break-words text-base">{agent.model}</p>
              <p className="mt-2 text-base text-fg-muted">
                {runtimes.data?.find((r) => r.id === agent.runtime_id)?.name ||
                  "运行时不可用"}
              </p>
              <div className="mt-5 flex gap-3 border-t border-line pt-4 text-base text-fg-muted">
                <span>{agent.skill_ids.length} Skills</span>
                <span>{agent.mcp_ids.length} MCP</span>
              </div>
            </a>
          ))}
        </div>
        {!query.isPending && !rows.length && (
          <EmptyState
            icon={Bot}
            title={search ? "没有匹配的 Agent" : "从模板创建你的第一个 Agent"}
          />
        )}
      </div>
      {creating && <InstanceEditor close={() => setCreating(false)} />}
    </>
  );
}
