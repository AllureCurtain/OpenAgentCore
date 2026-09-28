import { expect, it, vi } from "vitest";
import type { AgentSession, AgentTurn } from "@agents-core-web/agents-client";
import { readHistory, taskState } from "./api";

it("reads all history pages in chronological order and rejects a broken cursor", async () => {
  const read = vi
    .fn()
    .mockResolvedValueOnce({ data: [1], has_more: true, last_id: "one" })
    .mockResolvedValueOnce({ data: [2], has_more: false });
  const signal = new AbortController().signal;
  expect(await readHistory(read, signal)).toEqual([1, 2]);
  expect(read.mock.calls[1]?.[0]).toEqual({
    after: "one",
    limit: 100,
    order: "asc",
    signal,
  });
  await expect(
    readHistory(
      async () => ({ data: [], has_more: true, last_id: "same" }),
      signal,
    ),
  ).rejects.toThrow("分页游标");
});

it("does not equate idle with success and uses durable cancellation evidence", () => {
  const idle = { status: "idle" } as AgentSession;
  expect(taskState(idle).label).toBe("空闲");
  expect(taskState(idle, { status: "cancelled" } as AgentTurn).label).toBe(
    "已取消",
  );
  expect(
    taskState(
      { status: "in_progress" } as AgentSession,
      { status: "completed" } as AgentTurn,
    ).label,
  ).toBe("执行中");
});
