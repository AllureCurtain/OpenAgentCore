import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { homedir } from "node:os";
import assert from "node:assert/strict";
const base = process.env.OAC_EXAMPLE_LIVE_URL;
const model = process.env.OAC_EXAMPLE_LIVE_MODEL;
if (!base || !model)
  throw new Error("Set OAC_EXAMPLE_LIVE_URL and OAC_EXAMPLE_LIVE_MODEL.");
const directory = join(homedir(), ".oac", "tests", "parsar-agent-live");
await mkdir(directory, { recursive: true });
const browser = await chromium.launch({ channel: "chrome" });
const page = await browser.newPage({
  viewport: { width: 1440, height: 960 },
  reducedMotion: "reduce",
});
const suffix = Date.now().toString().slice(-6);
const modelName = `Kimi ${suffix}`,
  runtimeName = `开发沙箱 ${suffix}`,
  mcpName = `DeepWiki ${suffix}`,
  skillName = `code-review-${suffix}`,
  templateName = `审查助手 ${suffix}`,
  agentName = `代码助手 ${suffix}`;
const save = async () => {
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
};
const go = async (name) =>
  page.getByRole("link", { name, exact: true }).click();
try {
  await page.goto(`${base}/#/models`);
  await page.getByRole("button", { name: "添加模型", exact: true }).click();
  await page.getByLabel("显示名称").fill(modelName);
  await page.getByLabel("模型 ID").fill(model);
  await save();
  await go("运行时");
  await page.getByRole("button", { name: "添加运行时" }).click();
  await page.getByLabel("名称", { exact: true }).fill(runtimeName);
  await save();
  await go("MCP");
  await page.getByRole("button", { name: "添加MCP" }).click();
  await page.getByLabel("名称", { exact: true }).fill(mcpName);
  await page.getByLabel("服务标识").fill("deepwiki");
  await page.getByLabel("MCP 地址").fill("https://mcp.deepwiki.com/mcp");
  await save();
  await go("Skills");
  await page.getByRole("button", { name: "创建 Skill" }).click();
  await page.getByLabel("技能名称").fill(skillName);
  await page.getByLabel("用途").fill("检查代码修改，找出有依据的问题");
  await page
    .getByLabel("执行方法")
    .fill(
      "Read changed files. Report actionable correctness issues with file locations and explain the verification performed.",
    );
  await save();
  await go("模板");
  await page.getByRole("button", { name: "新建模板" }).click();
  await page.getByLabel("模板名称").fill(templateName);
  await page.getByLabel("模型", { exact: true }).click();
  await page.getByRole("option", { name: modelName, exact: true }).click();
  await page
    .getByLabel("指令", { exact: true })
    .fill(
      "先理解目标，再检查代码。复用已绑定的技能；需要公开仓库背景时使用 DeepWiki。给出明确结论和验证依据。",
    );
  await page.getByLabel(skillName, { exact: true }).check();
  await page.getByLabel(mcpName, { exact: true }).check();
  await save();
  await page
    .getByRole("article")
    .filter({
      has: page.getByRole("heading", { name: templateName, exact: true }),
    })
    .getByRole("button", { name: "创建 Agent", exact: true })
    .click();
  await page.getByLabel("Agent 名称").fill(agentName);
  await page.getByLabel("运行时", { exact: true }).click();
  await page.getByRole("option", { name: runtimeName, exact: true }).click();
  await page.getByRole("button", { name: "创建", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: agentName, exact: true }),
  ).toBeVisible({ timeout: 30000 });
  await page.reload();
  await expect(page.getByText(skillName, { exact: true })).toBeVisible();
  const records = await (
    await page.request.get(`${base}/app/instances`)
  ).json();
  const instance = records.find((row) => row.name === agentName);
  assert.ok(instance.environment.environment_template_id);
  assert.equal(instance.model, model);
  const core = await (
    await page.request.get(`${base}/v1/agents/${instance.core_agent_id}`)
  ).json();
  assert.equal(core.model, model);
  assert.equal(
    core.tools.find((tool) => tool.type === "mcp").transport.server_url,
    "https://mcp.deepwiki.com/mcp",
  );
  await page.screenshot({ path: join(directory, "agent-light.png") });
  await page.getByRole("button", { name: "切换深色" }).click();
  await page.locator("img").evaluate((image) => image.decode());
  await page.screenshot({ path: join(directory, "agent-dark.png") });
  await writeFile(
    join(directory, "result.json"),
    JSON.stringify(
      {
        passed: true,
        url: page.url(),
        instance_id: instance.id,
        core_agent_id: instance.core_agent_id,
        environment_template_id: instance.environment.environment_template_id,
        checks: [
          "model-catalog",
          "runtime-binding",
          "mcp-binding",
          "real-skill-upload",
          "template-copy",
          "core-agent-save",
          "reload",
        ],
      },
      null,
      2,
    ),
  );
  console.log("Live Agent workbench flow passed. No Session was started.");
} catch (error) {
  await page.screenshot({ path: join(directory, "failure.png") });
  await writeFile(
    join(directory, "failure.txt"),
    await page.locator("body").innerText(),
  );
  throw error;
} finally {
  await browser.close();
}
