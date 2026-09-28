import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  Plus,
  Settings2,
  MessageSquare,
  ChevronRight,
} from "lucide-react";
import { product, useSessions, type AgentProfile } from "./lib/product";
import { dateTime } from "./lib/api";
import { AgentEditor } from "./Agents";
import { NewSession } from "./NewSession";
import { Button } from "./components/ui/button";
import { EmptyState } from "./components/ui/empty-state";
import { ErrorNotice, Help, PageHeader } from "./components/shared";
export function AgentDetail({ id }: { id: string }) {
  const [editing, setEditing] = useState(false);
  const [creating, setCreating] = useState(false);
  const query = useQuery({
    queryKey: ["agent", id],
    queryFn: () => product<AgentProfile>(`agents/${id}`),
  });
  const sessions = useSessions();
  const rows = sessions.data?.filter((row) => row.agent_id === id) || [];
  return (
    <>
      <PageHeader title={query.data?.name || "Agent"}>
        <Button asChild variant="ghost" size="icon">
          <a href="#/agents" aria-label="返回 Agents">
            <ArrowLeft />
          </a>
        </Button>
        <Help>每个会话独立保存工作区和历史。修改 Agent 配置只影响新会话。</Help>
        <Button
          variant="outline"
          disabled={!query.data}
          onClick={() => setEditing(true)}
        >
          <Settings2 />
          配置
        </Button>
        <Button disabled={!query.data} onClick={() => setCreating(true)}>
          <Plus />
          开始会话
        </Button>
      </PageHeader>
      <ErrorNotice error={query.error || sessions.error} />
      <div className="min-h-0 flex-1 overflow-auto px-6 py-6">
        <h2 className="mb-4 text-lg font-semibold">会话</h2>
        <div className="mx-auto max-w-4xl divide-y divide-line rounded-xl border border-line">
          {rows.map((row) => (
            <a
              key={row.id}
              href={`#/sessions/${row.id}`}
              className="flex items-center gap-4 px-5 py-5 hover:bg-surface-subtle"
            >
              <MessageSquare className="h-5 w-5 shrink-0 text-fg-muted" />
              <span className="min-w-0 flex-1 truncate text-lg">
                {row.name}
              </span>
              <span className="text-base text-fg-muted">
                {row.core_session_id ? dateTime(row.created_at) : "待恢复"}
              </span>
              <ChevronRight className="h-4 w-4 shrink-0" />
            </a>
          ))}
        </div>
        {!sessions.isPending && !rows.length && (
          <EmptyState title="开始第一个会话" />
        )}
      </div>
      {editing && query.data && (
        <AgentEditor value={query.data} close={() => setEditing(false)} />
      )}
      {creating && query.data && (
        <NewSession agent={query.data} close={() => setCreating(false)} />
      )}
    </>
  );
}
