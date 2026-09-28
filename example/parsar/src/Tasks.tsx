import { useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { ListTodo, Plus } from "lucide-react";
import { api, dateTime, taskState, taskTitle } from "./lib/api";
import { Button } from "./components/ui/button";
import { col, Ledger, LedgerHeader, LedgerRow } from "./components/ui/ledger";
import { StatusIcon } from "./components/ui/status-icon";
import { EmptyState } from "./components/ui/empty-state";
import { ErrorNotice, Help, PageHeader } from "./components/shared";
import { NewTask } from "./NewTask";

export function Tasks() {
  const [creating, setCreating] = useState(false);
  const query = useInfiniteQuery({
    queryKey: ["tasks"],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      api.listSessionsTolerant({ limit: 30, after: pageParam, signal }),
    getNextPageParam: (page) =>
      page.has_more ? page.last_id || undefined : undefined,
    refetchInterval: 5000,
  });
  const tasks = query.data?.pages.flatMap((page) => page.data) || [];
  const unknown =
    query.data?.pages.reduce(
      (count, page) => count + page.unrecognized.length,
      0,
    ) || 0;
  const open = (id: string) => {
    location.hash = `/tasks/${id}`;
  };
  return (
    <>
      <PageHeader title="任务">
        <Help>
          展示当前项目的会话。空闲表示当前没有执行中的任务；进入详情可查看最近一次执行的结果。
        </Help>
        <Button onClick={() => setCreating(true)}>
          <Plus />
          新建任务
        </Button>
      </PageHeader>
      <ErrorNotice error={query.error} retry={() => void query.refetch()} />
      {!!unknown && (
        <ErrorNotice
          error={`有 ${unknown} 条记录使用了当前示例不支持的格式，未展示。`}
        />
      )}
      {query.isPending ? (
        <EmptyState title="正在加载…" />
      ) : !tasks.length && !query.error ? (
        <EmptyState icon={ListTodo} title="还没有任务" />
      ) : (
        <Ledger
          columns={[
            col.icon(),
            col.title(),
            col.text(),
            col.meta(),
            col.age(110),
          ]}
        >
          <LedgerHeader>
            <span />
            <span>任务</span>
            <span>Agent</span>
            <span>状态</span>
            <span>创建时间</span>
          </LedgerHeader>
          <ul role="listbox" aria-label="任务" className="m-0 list-none p-0">
            {tasks.map((task) => {
              const state = taskState(task);
              return (
                <LedgerRow
                  key={task.id}
                  onClick={() => open(task.id)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      open(task.id);
                    }
                  }}
                >
                  <StatusIcon status={state.icon} />
                  <span className="truncate font-medium">
                    {taskTitle(task)}
                  </span>
                  <span className="truncate">
                    {task.agent.name || task.agent.model}
                  </span>
                  <span>{state.label}</span>
                  <span className="font-mono text-xs text-fg-muted">
                    {dateTime(task.created_at)}
                  </span>
                </LedgerRow>
              );
            })}
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
      {creating && <NewTask close={() => setCreating(false)} />}
    </>
  );
}
