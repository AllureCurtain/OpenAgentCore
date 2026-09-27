import { expect, test, type Page, type TestInfo } from "@playwright/test";
import type { SandboxReset } from "@agents-core-web/agents-client";

import { expectManagementBoundary, openConsole, setDeployment, writes } from "./console";

const deploymentURL = "**/core/v1/sandbox/deployment";
const resetURL = "**/core/v1/sandbox/deployment/reset";
const resetPath = "/core/v1/sandbox/deployment/reset";
const unavailable = { status: 503, json: { error: { type: "server_error", code: null, param: null, message: "Deployment read unavailable." } } };
const observedReset = (clear: "auto" | "force" = "auto"): SandboxReset => ({
  clear, requested_at: "2026-09-26T10:00:00Z", deadline_at: "2026-09-26T11:00:00Z",
  forced_at: clear === "force" ? "2026-09-26T11:00:01Z" : null,
  remaining: { busy: 2, idle: 1, cleanup: 1, on_offline_nodes: 2, offline_nodes: [{ node_id: "offline-owned", name: "offline-build-worker", resources: 2 }] },
});
const progress = (page: Page) => page.getByRole("region", { name: "Reset in progress" });
async function openReset(page: Page) {
  await page.getByRole("button", { name: "Reset deployment", exact: true }).click();
  return page.getByRole("dialog", { name: "Reset sandbox deployment?", exact: true });
}
async function capture(page: Page, info: TestInfo, name: string) {
  const path = info.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: true });
  await info.attach(name, { path, contentType: "image/png" });
}

test.afterEach(async ({ request }) => {
  await expectManagementBoundary(request);
  expect((await writes(request)).filter((entry) => entry.includes("/maintenance"))).toEqual([]);
});

test("auto reset requires a bounded deadline, then preserves Core progress past that deadline", async ({ page, request }, info) => {
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page.emulateMedia({ colorScheme: "light" });
  await openConsole(page, request, "nodes");
  await setDeployment(request, { resources: { allocations: 3, pending: 1 } });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  const dialog = await openReset(page);
  const deadline = dialog.getByLabel(/Force remaining work after \(seconds\)/);
  await expect(deadline).toHaveValue("3600");
  await deadline.fill("299");
  await expect(dialog.getByRole("button", { name: "Reset deployment", exact: true })).toBeDisabled();
  await deadline.fill("86401");
  await expect(dialog.getByRole("button", { name: "Reset deployment", exact: true })).toBeDisabled();
  await deadline.fill("300");
  await expect(dialog).toContainText("Archived Sessions cannot be resumed");
  await expect(dialog).toContainText("self-hosted");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await capture(page, info, "reset-confirm-en-light-1280");
  const submitted = page.waitForRequest((sent) => sent.method() === "POST" && sent.url().endsWith(resetPath));
  await dialog.getByRole("button", { name: "Reset deployment", exact: true }).click();
  expect((await submitted).postDataJSON()).toEqual({ expected_generation: 1, clear: "auto", deadline_seconds: 300 });
  await expect(progress(page)).toBeVisible();
  // These timestamps are in the past: the browser still cannot invent force or completion.
  await setDeployment(request, { resources: { allocations: 3, pending: 1 }, reset: observedReset() });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(progress(page)).toContainText("offline-build-worker");
  await expect(progress(page)).toContainText("Busy work can finish until the deadline.");
  await expect(page.getByRole("button", { name: "Add node", exact: true })).toBeDisabled();
  await expect(page.getByRole("heading", { name: "Where should sandboxes run?" })).toHaveCount(0);
  await capture(page, info, "reset-progress-en-light-1280");
  expect(await writes(request)).toEqual([`POST ${resetPath}`]);
});

test("escalation and cancellation require explicit confirmation and never undo previous clearing", async ({ page, request }) => {
  await openConsole(page, request, "nodes");
  await setDeployment(request, { resources: { allocations: 3, pending: 1 }, reset: observedReset() });
  await page.reload();
  await progress(page).getByRole("button", { name: "Force reset now" }).click();
  let confirm = page.getByRole("dialog", { name: "Force reset now?" });
  await confirm.getByRole("button", { name: "Back", exact: true }).click();
  expect(await writes(request)).toEqual([]);
  await progress(page).getByRole("button", { name: "Force reset now" }).click();
  confirm = page.getByRole("dialog", { name: "Force reset now?" });
  const force = page.waitForRequest((sent) => sent.method() === "POST" && sent.url().endsWith(resetPath));
  await confirm.getByRole("button", { name: "Force reset now", exact: true }).click();
  expect((await force).postDataJSON()).toEqual({ expected_generation: 1, clear: "force" });
  await expect(progress(page)).toContainText("Remaining hosted Sessions are being archived");
  await expect(progress(page)).toContainText("offline-build-worker");
  await expect(progress(page).getByRole("button", { name: "Force reset now" })).toHaveCount(0);
  await progress(page).getByRole("button", { name: "Cancel reset" }).click();
  confirm = page.getByRole("dialog", { name: "Cancel reset?" });
  await expect(confirm).toContainText("Sessions already archived stay archived");
  const cancel = page.waitForRequest((sent) => sent.method() === "DELETE" && sent.url().includes(resetPath));
  await confirm.getByRole("button", { name: "Cancel reset", exact: true }).click();
  expect(new URL((await cancel).url()).searchParams.get("expected_generation")).toBe("1");
  await expect(progress(page)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Reset deployment", exact: true })).toBeEnabled();
  expect(await writes(request)).toEqual([`POST ${resetPath}`, `DELETE ${resetPath}`]);
});

test("force reset requires confirmation and reconfiguration uses the completed generation once", async ({ page, request }) => {
  await openConsole(page, request, "nodes");
  await setDeployment(request, { resources: { allocations: 1, pending: 0 } });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  const dialog = await openReset(page);
  await dialog.getByLabel("Force — cancel remaining work now", { exact: true }).check();
  await expect(dialog).toContainText("Force immediately cancels remaining work");
  expect(await writes(request)).toEqual([]);
  await dialog.getByRole("button", { name: "Reset deployment", exact: true }).click();
  await expect(progress(page)).toContainText("Remaining hosted Sessions are being archived");
  // Simulate the cleanup worker's completed projection, not elapsed browser time.
  await setDeployment(request, { complete_reset: true });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await page.getByRole("button", { name: "Own machines" }).click();
  await page.getByRole("button", { name: "microsandbox Recommended" }).click();
  await page.getByRole("button", { name: /^Standard/ }).click();
  const configured = page.waitForRequest((sent) => sent.method() === "POST" && sent.url().endsWith("/sandbox/deployment"));
  await page.getByRole("button", { name: "Save configuration" }).click();
  expect((await configured).postDataJSON()).toMatchObject({ expected_generation: 2, provider: "microsandbox" });
  await expect(page.getByRole("dialog", { name: "Add node" })).toBeVisible();
  expect(await writes(request)).toEqual([`POST ${resetPath}`, "POST /core/v1/sandbox/deployment"]);
});

test("an applied reset with a lost response stays blocked through failed reads and is never replayed", async ({ page, request }) => {
  await openConsole(page, request, "nodes");
  await expect(page.getByRole("button", { name: "Reset deployment", exact: true })).toBeEnabled();
  await setDeployment(request, { resources: { allocations: 1, pending: 0 } });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(page.getByRole("button", { name: "Reset deployment", exact: true })).toBeEnabled();
  let failReads = true;
  await page.route(resetURL, async (route) => {
    await route.fetch(); // Core accepted the mutation, but its response never reached this browser.
    await route.abort("failed");
  });
  await page.route(deploymentURL, (route) => failReads ? route.fulfill(unavailable) : route.continue());
  const dialog = await openReset(page);
  await dialog.getByRole("button", { name: "Reset deployment", exact: true }).click();
  const uncertain = page.getByRole("dialog", { name: "Couldn't confirm the sandbox change" });
  await expect(uncertain).toBeVisible();
  await uncertain.getByRole("button", { name: "Refresh sandbox state" }).click();
  await expect(page.getByRole("button", { name: "Reset deployment", exact: true })).toBeDisabled();
  expect(await writes(request)).toEqual([`POST ${resetPath}`]);
  failReads = false;
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(progress(page)).toContainText("Busy work can finish until the deadline.");
  await expect(progress(page).getByRole("button", { name: "Cancel reset" })).toBeEnabled();
  expect(await writes(request)).toEqual([`POST ${resetPath}`]);
});

test("a stale reset is refused until fresh state is reviewed, with no automatic resubmission", async ({ page, request }) => {
  await openConsole(page, request, "nodes");
  await expect(page.getByRole("button", { name: "Reset deployment", exact: true })).toBeEnabled();
  await setDeployment(request, { generation: 2 });
  const dialog = await openReset(page);
  await dialog.getByRole("button", { name: "Reset deployment", exact: true }).click();
  await expect(page.getByText("Core has a newer sandbox configuration. Refresh and review it before submitting again.")).toBeVisible();
  expect(await writes(request)).toEqual([`POST ${resetPath}`]);
  await dialog.getByRole("button", { name: "Back", exact: true }).click();
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(page.getByRole("button", { name: "Reset deployment", exact: true })).toBeEnabled();
  expect(await writes(request)).toEqual([`POST ${resetPath}`]);
});

test("reset progress survives failed node and deployment reads with visible qualifications in Chinese dark mode", async ({ page, request }, info) => {
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page.emulateMedia({ colorScheme: "dark" });
  await openConsole(page, request, "nodes");
  await setDeployment(request, { resources: { allocations: 3, pending: 1 }, reset: observedReset() });
  await page.route("**/core/v1/sandbox/nodes", (route) => route.fulfill(unavailable));
  await page.reload();
  await expect(progress(page)).toContainText("offline-build-worker");
  // Node detail failure cannot block a control needing only fresh deployment state.
  await expect(progress(page).getByRole("button", { name: "Cancel reset" })).toBeEnabled();
  await page.getByRole("button", { name: "Language and appearance" }).click();
  await page.getByRole("menuitemradio", { name: "简体中文" }).click();
  const localized = page.getByRole("region", { name: "正在重置", exact: true });
  await expect(localized).toContainText("offline-build-worker");
  await expect(localized).toContainText("正在归档空闲沙箱");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await capture(page, info, "reset-progress-zh-dark-1280");
  await page.route(deploymentURL, (route) => route.fulfill(unavailable));
  await page.getByRole("button", { name: "刷新沙箱状态", exact: true }).click();
  await expect(localized).toContainText("offline-build-worker");
  await expect(localized.getByRole("button", { name: "取消重置" })).toBeDisabled();
  await expect(localized).toContainText("无法刷新重置进度。当前显示的是上次确认的数量。");
  expect(await writes(request)).toEqual([]);
});
