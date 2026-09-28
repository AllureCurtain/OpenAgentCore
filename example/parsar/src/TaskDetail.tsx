import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Info } from "lucide-react";
import { api, dateTime, readHistory, taskState, taskTitle } from "./lib/api";
import { Button } from "./components/ui/button";
import { EmptyState } from "./components/ui/empty-state";
import { StatusIcon } from "./components/ui/status-icon";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "./components/ui/dialog";
import { Property, PropertyList } from "./components/ui/property-list";
import { ErrorNotice } from "./components/shared";
import { Activity, Thinking } from "./components/Activity";
import { Composer } from "./Composer";

export function TaskDetail({ id }: { id: string }) {
  const [info, setInfo] = useState(false);
  const viewport = useRef<HTMLDivElement>(null);
  const followOutput = useRef(true);
  const query = useQuery({
    queryKey: ["task", id],
    queryFn: async ({ signal }) => {
      const [session, items, turns] = await Promise.all([
        api.retrieveSession(id, { signal }),
        readHistory((options) => api.listItems(id, options), signal),
        api.listTurns(id, { limit: 20, order: "desc", signal }),
      ]);
      return { session, items, turns: turns.data };
    },
    refetchInterval: 2000,
  });
  const data = query.data;
  useEffect(() => {
    if (viewport.current && followOutput.current)
      viewport.current.scrollTop = viewport.current.scrollHeight;
  }, [data?.items]);
  const session = data?.session;
  const state = session ? taskState(session, data?.turns[0]) : undefined;
  return (
    <>
      <header className="flex h-16 shrink-0 items-center gap-3 border-b border-line px-6">
        <Button asChild variant="ghost" size="icon">
          <a href="#/tasks" aria-label="返回任务列表">
            <ArrowLeft />
          </a>
        </Button>
        <h1 className="min-w-0 flex-1 truncate text-xl font-semibold">
          {session ? taskTitle(session) : "任务"}
        </h1>
        {state && (
          <span className="flex shrink-0 items-center gap-2 text-base">
            <StatusIcon status={state.icon} />
            {state.label}
          </span>
        )}
        <Button
          variant="ghost"
          size="icon"
          aria-label="任务详情"
          disabled={!session}
          onClick={() => setInfo(true)}
        >
          <Info />
        </Button>
      </header>
      <ErrorNotice error={query.error} retry={() => void query.refetch()} />
      {session?.error && <ErrorNotice error={session.error} />}
      {data?.turns[0]?.error && !session?.error && (
        <ErrorNotice error={data.turns[0].error.message} />
      )}
      {query.isPending ? (
        <EmptyState title="正在加载…" />
      ) : (
        data && (
          <>
            <div
              ref={viewport}
              onScroll={(event) => {
                const element = event.currentTarget;
                followOutput.current =
                  element.scrollHeight -
                    element.scrollTop -
                    element.clientHeight <
                  80;
              }}
              className="min-h-0 flex-1 overflow-y-auto"
              aria-label="任务过程"
            >
              <div className="mx-auto max-w-3xl px-6 py-6">
                {data.items.map((item) => (
                  <Activity key={item.id} item={item} />
                ))}
                {!data.items.length && (
                  <EmptyState title="等待执行记录" size="compact" />
                )}
                {data.session.status === "in_progress" && <Thinking />}
                {data.session.status === "requires_action" && (
                  <div role="status" className="py-4 text-base">
                    此任务需要外部操作，当前示例暂不支持处理。可取消本次执行。
                    <details className="mt-2">
                      <summary className="cursor-pointer">查看所需操作</summary>
                      <pre className="overflow-auto py-2 text-sm">
                        {JSON.stringify(data.session.required_actions, null, 2)}
                      </pre>
                    </details>
                  </div>
                )}
              </div>
            </div>
            <Composer
              key={`${id}:${data.turns[0]?.id || "initial"}`}
              session={data.session}
            />
          </>
        )
      )}
      {session && (
        <Dialog open={info} onOpenChange={setInfo}>
          <DialogContent aria-describedby={undefined}>
            <DialogHeader>
              <DialogTitle>任务详情</DialogTitle>
            </DialogHeader>
            <PropertyList>
              <Property label="Agent">
                {session.agent.name || "未命名 Agent"}
              </Property>
              <Property label="模型">{session.agent.model}</Property>
              <Property label="运行引擎">
                {session.agent.x_agents_core?.harness || "部署默认"}
              </Property>
              <Property label="环境">{session.environment.type}</Property>
              <Property label="创建时间">
                {dateTime(session.created_at)}
              </Property>
              <Property label="Session ID" mono>
                {session.id}
              </Property>
              <Property label="Tokens" mono>
                {session.usage?.total_tokens ?? "未报告"}
              </Property>
            </PropertyList>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
