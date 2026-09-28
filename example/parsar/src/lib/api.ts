import {
  OpenAIAgentsClient,
  type ListPage,
  type PageOptions,
} from "@agents-core-web/agents-client";

export const api = new OpenAIAgentsClient();

export async function readHistory<T>(
  read: (options: PageOptions) => Promise<ListPage<T>>,
  signal: AbortSignal,
) {
  const data: T[] = [];
  let after: string | undefined;
  for (let pageNumber = 0; pageNumber < 100; pageNumber++) {
    const page = await read({ limit: 100, order: "asc", after, signal });
    data.push(...page.data);
    if (!page.has_more) return data;
    if (!page.last_id || page.last_id === after)
      throw new Error("Core 返回了无效的历史分页游标。");
    after = page.last_id;
  }
  throw new Error("历史超过 10,000 条，当前示例无法完整展示。");
}

export const errorText = (error: unknown) =>
  error instanceof Error
    ? error.message
    : typeof error === "string"
      ? error
      : "请求失败，请重试。";
