import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Pencil, Trash2 } from "lucide-react";
import {
  harnessNames,
  product,
  useRuntimes,
  type AgentInstance,
} from "./lib/product";
import { useSkills } from "./Skills";
import { InstanceEditor } from "./InstanceEditor";
import { Button } from "./components/ui/button";
import { ErrorNotice, Help } from "./components/shared";
import { EmptyState } from "./components/ui/empty-state";
export function AgentDetail({ id }: { id: string }) {
  const query = useQuery({
    queryKey: ["instance", id],
    queryFn: () => product<AgentInstance>(`instances/${id}`),
  });
  const runtimes = useRuntimes();
  const skills = useSkills();
  const [editing, setEditing] = useState(false);
  const agent = query.data;
  const cache = useQueryClient();
  const remove = useMutation({
    mutationFn: () => product(`instances/${id}`, "DELETE"),
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: ["instances"] });
      location.hash = "/agents";
    },
  });
  return (
    <>
      <header className="flex min-h-16 items-center gap-3 border-b border-line px-6 py-3">
        <Button asChild variant="ghost" size="icon">
          <a href="#/agents" aria-label="返回 Agents">
            <ArrowLeft />
          </a>
        </Button>
        <h1 className="min-w-0 flex-1 truncate text-xl font-semibold">
          {agent?.name || "Agent"}
        </h1>
        <Button
          variant="ghost"
          size="icon"
          aria-label="删除 Agent"
          disabled={!agent || remove.isPending}
          onClick={() => {
            if (confirm("删除这个 Agent？此操作不会删除来源模板。"))
              remove.mutate();
          }}
        >
          <Trash2 />
        </Button>
        <Button
          variant="outline"
          disabled={!agent}
          onClick={() => setEditing(true)}
        >
          <Pencil />
          编辑配置
        </Button>
      </header>
      <ErrorNotice
        error={query.error || runtimes.error || skills.error || remove.error}
      />
      {agent ? (
        <div className="min-h-0 flex-1 overflow-auto">
          <div className="mx-auto max-w-4xl space-y-8 p-6 md:p-10">
            <section className="grid gap-6 rounded-xl border border-line p-5 sm:grid-cols-3">
              <div>
                <h2 className="mb-2 text-base text-fg-muted">模型</h2>
                <p className="break-words text-lg">{agent.model}</p>
              </div>
              <div>
                <h2 className="mb-2 text-base text-fg-muted">执行引擎</h2>
                <p className="text-lg">{harnessNames[agent.harness]}</p>
              </div>
              <div>
                <div className="mb-2 flex items-center gap-1">
                  <h2 className="text-base text-fg-muted">运行时</h2>
                  <Help>
                    此处展示绑定配置。托管沙箱在执行时由 Core 分配，保存 Agent
                    不会启动沙箱。
                  </Help>
                </div>
                <p className="text-lg">
                  {runtimes.data?.find((r) => r.id === agent.runtime_id)
                    ?.name || "不可用"}
                </p>
              </div>
            </section>
            <section>
              <h2 className="mb-3 text-lg font-semibold">指令</h2>
              <p className="whitespace-pre-wrap text-base leading-relaxed">
                {agent.instructions || "尚未设置指令"}
              </p>
            </section>
            <section>
              <h2 className="mb-3 text-lg font-semibold">Skills</h2>
              <div className="flex flex-wrap gap-2">
                {agent.skill_ids.map((skillId) => (
                  <span
                    key={skillId}
                    className="rounded-lg border border-line px-4 py-2 text-base"
                  >
                    {skills.data?.find((skill) => skill.id === skillId)?.name ||
                      "不可用技能"}
                  </span>
                ))}
                {!agent.skill_ids.length && (
                  <p className="text-base text-fg-muted">未绑定技能</p>
                )}
              </div>
            </section>
            <section>
              <h2 className="mb-3 text-lg font-semibold">MCP</h2>
              <div className="space-y-3">
                {agent.tools.map((tool, index) => {
                  const mcp = tool as {
                    server_label: string;
                    transport: { server_url: string };
                  };
                  return (
                    <div
                      key={index}
                      className="rounded-lg border border-line p-4"
                    >
                      <h3 className="text-base font-medium">
                        {mcp.server_label}
                      </h3>
                      <p className="mt-2 break-words text-base text-fg-muted">
                        {mcp.transport.server_url}
                      </p>
                    </div>
                  );
                })}
                {!agent.tools.length && (
                  <p className="text-base text-fg-muted">未绑定 MCP</p>
                )}
              </div>
            </section>
          </div>
        </div>
      ) : (
        !query.error && <EmptyState title="正在加载…" />
      )}
      {editing && agent && (
        <InstanceEditor instance={agent} close={() => setEditing(false)} />
      )}
    </>
  );
}
