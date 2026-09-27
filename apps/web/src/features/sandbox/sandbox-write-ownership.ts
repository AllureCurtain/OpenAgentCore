import { queryOptions, type QueryClient } from "@tanstack/react-query";

export interface SandboxWriteOwnership {
  phase: "idle" | "pending" | "reconcile";
  attempt: number;
  /** A GET must begin after settlement, not merely after submission. */
  readAfter: number;
}
const idle: SandboxWriteOwnership = { phase: "idle", attempt: 0, readAfter: 0 };
let nextAttempt = 0;

/** In-memory, connection-scoped ownership; QueryClient.clear() discards it on logout. */
export const sandboxWriteOwnershipQuery = queryOptions<SandboxWriteOwnership>({
  queryKey: ["sandbox-write-ownership"], queryFn: () => idle,
  initialData: idle, enabled: false, gcTime: Infinity,
});

export function beginSandboxWrite(cache: QueryClient): number | null {
  const current = cache.getQueryData(sandboxWriteOwnershipQuery.queryKey);
  if (current && current.phase !== "idle") return null;
  const attempt = ++nextAttempt;
  cache.setQueryData(sandboxWriteOwnershipQuery.queryKey, { phase: "pending", attempt, readAfter: performance.now() });
  return attempt;
}

export function ownsSandboxWrite(cache: QueryClient, attempt: number): boolean {
  return cache.getQueryData(sandboxWriteOwnershipQuery.queryKey)?.attempt === attempt;
}

export function settleSandboxWrite(cache: QueryClient, attempt: number): boolean {
  if (!ownsSandboxWrite(cache, attempt)) return false;
  cache.setQueryData(sandboxWriteOwnershipQuery.queryKey, { phase: "reconcile", attempt, readAfter: performance.now() });
  return true;
}

/** A read during a pending request, or begun before it settled, proves no outcome. */
export function confirmSandboxRead(cache: QueryClient, readStartedAt: number): void {
  const current = cache.getQueryData(sandboxWriteOwnershipQuery.queryKey);
  if (current?.phase === "reconcile" && readStartedAt > current.readAfter) {
    cache.setQueryData(sandboxWriteOwnershipQuery.queryKey, { ...current, phase: "idle" });
  }
}
