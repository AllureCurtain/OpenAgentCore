import { test, expect, type Page } from "@playwright/test";

async function createAgent(page: Page) {
  await page.goto("/#/agents");
  await page.getByRole("button", { name: "新建 Agent", exact: true }).click();
  await page.getByLabel("名称", { exact: true }).fill("代码助手");
  await page.getByLabel("模型", { exact: true }).fill("fixture-model");
  await page
    .getByLabel("指令", { exact: true })
    .fill("先检查，再修改。说明验证结果。");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("option", { name: /代码助手/ })).toBeVisible();
}

async function newTask(
  page: Page,
  prompt = "检查登录流程，补充测试，并给出修改总结。",
) {
  await page.getByRole("link", { name: "任务", exact: true }).click();
  await page.getByRole("button", { name: "新建任务" }).click();
  await page.getByLabel("任务名称").fill("检查登录流程");
  await page.getByLabel("Agent", { exact: true }).click();
  await page.getByRole("option", { name: "代码助手" }).click();
  await page.getByLabel("任务内容").fill(prompt);
  await page.getByRole("button", { name: "开始任务" }).click();
}

test.beforeEach(async ({ request }) => {
  await request.post("http://127.0.0.1:18181/reset");
});

test("oversized creation and continuation remain editable after rejection", async ({
  page,
}) => {
  await createAgent(page);
  await newTask(page, "x".repeat(1024 * 1024));
  await expect(page.getByRole("alert")).toContainText("Request too large");
  await expect(page.getByLabel("任务内容")).toBeEditable();
  await page.getByLabel("任务内容").fill("Shorter request");
  await page.getByRole("button", { name: "开始任务" }).click();
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  await page.getByLabel("继续对话").fill("x".repeat(1024 * 1024));
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("exceeds 1 MiB");
  await expect(page.getByLabel("继续对话")).toBeEditable();
  await page.getByLabel("继续对话").fill("Shorter follow-up");
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(page.getByRole("button", { name: "取消执行" })).toBeVisible();
});

test("Agent, task, history recovery, follow-up and cancellation through the server", async ({
  page,
}, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await createAgent(page);
  await newTask(page);
  await expect(
    page.getByRole("heading", { name: "检查登录流程", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("12 项测试通过。")).toBeVisible();
  await page.reload();
  await expect(page.getByText("12 项测试通过。")).toBeVisible();
  await page.getByText("npm test", { exact: true }).click();
  await expect(page.getByText("12 tests passed")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("task-light.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "切换深色" }).click();
  await expect(page.locator('img[src="/parsar-mark-dark.png"]')).toBeVisible();
  await page
    .locator('img[src="/parsar-mark-dark.png"]')
    .evaluate((image: HTMLImageElement) => image.decode());
  await page.screenshot({
    path: testInfo.outputPath("task-dark.png"),
    fullPage: true,
  });
  await page.getByLabel("继续对话").fill("继续检查移动端");
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(page.getByRole("button", { name: "取消执行" })).toBeVisible();
  await page.getByRole("button", { name: "取消执行" }).click();
  await expect(page.getByText("已取消", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "任务详情", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("fixture-model");
  expect(errors).toEqual([]);
});

test("a lost creation response can be recovered after reload without duplicating a task", async ({
  page,
  request,
}) => {
  await createAgent(page);
  await request.post("http://127.0.0.1:18181/lose-creation");
  await newTask(page);
  await expect(page.getByRole("alert")).toContainText("creation response lost");
  await page.reload();
  await page.getByRole("button", { name: "新建任务" }).click();
  await expect(page.getByLabel("任务内容")).toBeDisabled();
  await page.getByRole("button", { name: "重试", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "检查登录流程", exact: true }),
  ).toBeVisible();
  expect(
    await (await request.get("http://127.0.0.1:18181/counts")).json(),
  ).toMatchObject({ sessions: 1 });
});

test("compact navigation, one primary action and keyboard-accessible help", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createAgent(page);
  await page.getByRole("link", { name: "任务", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "任务", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "新建任务" })).toHaveCount(1);
  await page.getByRole("button", { name: "新建任务" }).focus();
  await page.keyboard.press("Shift+Tab");
  await expect(page.getByRole("button", { name: "了解更多" })).toBeFocused();
  await expect(page.getByRole("tooltip")).toBeVisible();
  await page.keyboard.press("Escape");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("mobile.png"),
    fullPage: true,
  });
});
