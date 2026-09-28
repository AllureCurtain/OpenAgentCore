import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { join } from "node:path";

const baseURL = process.env.OAC_EXAMPLE_LIVE_URL;
const model = process.env.OAC_EXAMPLE_LIVE_MODEL;
if (!baseURL || !model)
  throw new Error(
    "Set OAC_EXAMPLE_LIVE_URL and OAC_EXAMPLE_LIVE_MODEL; the example server must use a dedicated live Project.",
  );
const output = join(
  process.env.OAC_DEV_HOME || join(homedir(), ".oac"),
  "tests",
  "parsar-example-live",
);
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ channel: "chrome", headless: true });
const page = await browser.newPage({
  viewport: { width: 1440, height: 960 },
  baseURL,
});
const name = `Live acceptance ${Date.now()}`;
try {
  await page.goto("/#/agents");
  await page.getByRole("button", { name: "新建 Agent", exact: true }).click();
  await page.getByLabel("名称", { exact: true }).fill(name);
  await page.getByLabel("模型", { exact: true }).fill(model);
  await page.getByLabel("运行引擎", { exact: true }).click();
  await page
    .getByRole("option", {
      name: process.env.OAC_EXAMPLE_LIVE_HARNESS || "Claude Code",
      exact: true,
    })
    .click();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("link", { name: "任务", exact: true }).click();
  await page.getByRole("button", { name: "新建任务" }).click();
  await page.getByLabel("任务名称").fill(name);
  await page.getByLabel("Agent", { exact: true }).click();
  await page.getByRole("option", { name, exact: true }).click();
  await page
    .getByLabel("任务内容")
    .fill(
      "Compute 19 + 23. Reply with PARSAR_LIVE_OK and the result. Remember the result for this conversation.",
    );
  await page.getByRole("button", { name: "开始任务" }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible({
    timeout: 40_000,
  });
  process.stdout.write("Live task admitted through the example UI.\n");
  await expect(page.getByText(/^(已完成|失败)$/)).toBeVisible({
    timeout: 180_000,
  });
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  await expect(page.getByRole("article", { name: "Agent 回复" })).toContainText(
    "PARSAR_LIVE_OK",
  );
  await expect(page.getByRole("article", { name: "Agent 回复" })).toContainText(
    "42",
  );
  await page.reload();
  await expect(page.getByRole("article", { name: "Agent 回复" })).toContainText(
    "PARSAR_LIVE_OK",
  );
  await page.screenshot({
    path: join(output, "live-task.png"),
    fullPage: true,
  });
  process.stdout.write("Live model output and persisted reload passed.\n");
  await page
    .getByLabel("继续对话")
    .fill(
      "Which number did I ask you to remember? Reply only REMEMBERED and that number.",
    );
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(
    page
      .getByRole("article", { name: "Agent 回复" })
      .filter({ hasText: "REMEMBERED" }),
  ).toContainText("42", { timeout: 180_000 });
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  process.stdout.write(
    "Completed continuation with remembered context passed.\n",
  );
  await page
    .getByLabel("继续对话")
    .fill(
      "Use the shell tool to run sleep 60, then tell me the number I asked you to remember. Do not respond until the command finishes.",
    );
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(page.getByRole("button", { name: "取消执行" })).toBeVisible({
    timeout: 30_000,
  });
  await page.getByRole("button", { name: "取消执行" }).click();
  await expect(page.getByText("已取消", { exact: true })).toBeVisible({
    timeout: 90_000,
  });
  await page.reload();
  await expect(page.getByText("已取消", { exact: true })).toBeVisible();
  await writeFile(
    join(output, "result.json"),
    JSON.stringify(
      {
        passed: true,
        task: page.url(),
        checks: [
          "agent-save",
          "hosted-creation",
          "real-model-output",
          "history-reload",
          "completed-continuation",
          "durable-cancel",
        ],
      },
      null,
      2,
    ),
  );
  process.stdout.write("Follow-up and durable cancellation passed.\n");
} catch (error) {
  await page.screenshot({ path: join(output, "failure.png"), fullPage: true });
  await writeFile(
    join(output, "failure.txt"),
    await page.locator("body").innerText(),
  );
  throw error;
} finally {
  await browser.close();
}
