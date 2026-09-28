import { test, expect, type Page } from "@playwright/test";
async function navigate(page: Page, name: string) {
  await page.getByRole("link", { name, exact: true }).click();
}
async function resources(page: Page) {
  await page.goto("/#/models");
  await page.getByRole("button", { name: "添加模型", exact: true }).click();
  await page.getByLabel("显示名称").fill("Kimi");
  await page.getByLabel("模型 ID").fill("kimi-k2.6");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await navigate(page, "运行时");
  await page.getByRole("button", { name: "添加运行时" }).click();
  await page.getByLabel("名称", { exact: true }).fill("开发沙箱");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await navigate(page, "MCP");
  await page.getByRole("button", { name: "添加MCP" }).click();
  await page.getByLabel("名称", { exact: true }).fill("文档服务");
  await page.getByLabel("服务标识").fill("docs");
  await page.getByLabel("MCP 地址").fill("https://mcp.example/docs");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await navigate(page, "Skills");
  await page.getByRole("button", { name: "创建 Skill" }).click();
  await page.getByLabel("技能名称").fill("code-review");
  await page.getByLabel("用途").fill("检查代码质量");
  await page
    .getByLabel("执行方法")
    .fill("Inspect changed files and report actionable issues.");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
}
async function template(page: Page) {
  await navigate(page, "模板");
  await page.getByRole("button", { name: "新建模板" }).click();
  await page.getByLabel("模板名称").fill("审查助手");
  await page.getByLabel("模型", { exact: true }).click();
  await page.getByRole("option", { name: "Kimi", exact: true }).click();
  await page
    .getByLabel("指令", { exact: true })
    .fill("Read the code and report findings.");
  await page.getByLabel("code-review", { exact: true }).check();
  await page.getByLabel("文档服务", { exact: true }).check();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
}
async function instance(page: Page) {
  await page.getByRole("button", { name: "创建 Agent", exact: true }).click();
  await page.getByLabel("Agent 名称").fill("Ada");
  await page.getByLabel("运行时", { exact: true }).click();
  await page.getByRole("option", { name: "开发沙箱", exact: true }).click();
  await page.getByRole("button", { name: "创建", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Ada", exact: true }),
  ).toBeVisible();
}
test.beforeEach(async ({ request }) => {
  for (const kind of ["instances", "templates", "models", "mcps", "runtimes"]) {
    const rows = await (await request.get(`/app/${kind}`)).json();
    for (const row of rows)
      await request.delete(`/app/${kind}/${row.id}`, {
        headers: { origin: "http://127.0.0.1:18180" },
      });
  }
  await request.post("http://127.0.0.1:18181/reset");
});
test("resources to template to independent Agent, with real Core-shaped bindings", async ({
  page,
  request,
}, testInfo) => {
  await resources(page);
  await template(page);
  await instance(page);
  await expect(page.getByText("kimi-k2.6", { exact: true })).toBeVisible();
  await expect(page.getByText("code-review", { exact: true })).toBeVisible();
  await expect(
    page.getByText("https://mcp.example/docs", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Ada", exact: true }),
  ).toBeVisible();
  const core = await (
    await request.get("http://127.0.0.1:18181/counts")
  ).json();
  expect(core.sessions).toBe(0);
  expect(core.agents[0].tools[1].server_label).toBe("docs");
  expect(core.templates[0].skills[0].skill_id).toBe(core.skills[0].id);
  await page.screenshot({ path: testInfo.outputPath("agent-light.png") });
  await page.getByRole("button", { name: "切换深色" }).click();
  await page
    .locator("img")
    .evaluate((image: HTMLImageElement) => image.decode());
  await page.screenshot({ path: testInfo.outputPath("agent-dark.png") });
  await navigate(page, "模板");
  await page.getByRole("button", { name: "编辑 审查助手" }).click();
  await page
    .getByLabel("指令", { exact: true })
    .fill("New template instructions");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await navigate(page, "Agents");
  await page.getByRole("link", { name: /Ada/ }).click();
  await expect(
    page.getByText("Read the code and report findings.", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "编辑配置" }).click();
  await page
    .getByLabel("指令", { exact: true })
    .fill("Independent Agent instructions");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(
    page.getByText("Independent Agent instructions", { exact: true }),
  ).toBeVisible();
  page.on("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "删除 Agent", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Agents", exact: true }),
  ).toBeVisible();
  expect(
    (await (await request.get("http://127.0.0.1:18181/counts")).json()).agents,
  ).toHaveLength(0);
});
test("model edits persist and bound resources cannot be removed", async ({
  page,
}) => {
  await resources(page);
  await template(page);
  await navigate(page, "模型");
  await page.getByRole("button", { name: "编辑 Kimi" }).click();
  await page.getByLabel("显示名称").fill("常用 Kimi");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("heading", { name: "常用 Kimi" })).toBeVisible();
  await page.reload();
  page.on("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "移除 常用 Kimi" }).click();
  await expect(page.getByRole("alert")).toContainText("仍被引用");
});
test("Skill version upload and default selection", async ({ page }) => {
  await resources(page);
  await page.getByRole("button", { name: /code-review v1/ }).click();
  await page.getByLabel("上传 Skill 新版本").setInputFiles({
    name: "review.zip",
    mimeType: "application/zip",
    buffer: Buffer.from("fixture bundle"),
  });
  await expect(
    page.getByRole("dialog").getByText("v2", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "设为默认" }).click();
  await expect(
    page.getByRole("dialog").getByText("v1", { exact: true }).locator(".."),
  ).toContainText("默认版本");
});
test("mobile navigation, help and single creation action", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/#/agents");
  await expect(
    page.getByRole("button", { name: "创建 Agent", exact: true }),
  ).toHaveCount(1);
  await page.getByRole("button", { name: "创建 Agent", exact: true }).focus();
  await page.keyboard.press("Shift+Tab");
  await expect(page.getByRole("tooltip")).toBeVisible();
  await page.keyboard.press("Escape");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("mobile.png") });
});
