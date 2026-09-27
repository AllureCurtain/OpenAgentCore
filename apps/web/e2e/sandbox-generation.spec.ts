import { expect, test, type Locator, type Page } from "@playwright/test";
import type { SandboxAllocation, SandboxDeployment, SandboxNode } from "@agents-core-web/agents-client";

import { expectManagementBoundary, openConsole, setDeployment, setNode, writes } from "./console";

const deploymentPath = "/core/v1/sandbox/deployment";
const rollout = (page: Page) => page.getByRole("region", { name: "Configuration rollout", exact: true });
const fact = (scope: Locator, label: string) => scope.locator("dt").filter({ hasText: new RegExp(`^${label}$`) }).locator("..").locator("dd");
const deploymentRead = (page: Page) => read<SandboxDeployment>(page, deploymentPath);
async function read<T>(page: Page, path: string): Promise<T> {
  const response = await page.request.get(new URL(path, page.url()).href);
  expect(response.ok()).toBe(true);
  return response.json() as Promise<T>;
}
async function inventory(page: Page) {
  const nodes = (await read<{ data: SandboxNode[] }>(page, "/core/v1/sandbox/nodes")).data;
  const allocations = (await Promise.all(nodes.map((node) => read<{ data: SandboxAllocation[] }>(page, `/core/v1/sandbox/nodes/${node.id}/allocations`)))).flatMap((result) => result.data);
  return { nodes, allocations };
}
async function saveConfiguration(page: Page) {
  const sent = page.waitForRequest((request) => request.method() === "PUT" && request.url().endsWith(deploymentPath));
  const response = page.waitForResponse((response) => response.request().method() === "PUT" && response.url().endsWith(deploymentPath));
  await page.getByRole("button", { name: "Save configuration", exact: true }).click();
  return { input: (await sent).postDataJSON(), response: await response };
}
async function editE2B(page: Page, key?: string) {
  await page.getByRole("button", { name: "Change resources", exact: true }).click();
  await expect(page.getByLabel("E2B API key", { exact: true })).toHaveValue("");
  if (key !== undefined) await page.getByLabel("E2B API key", { exact: true }).fill(key);
  await page.getByRole("button", { name: "Next", exact: true }).click();
}

test.afterEach(async ({ request }) => expectManagementBoundary(request));

test("online configuration retains existing resources and stops rapid polling when preparation settles", async ({ page, request }, info) => {
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "light" });
  const pending = new Set<unknown>();
  page.on("request", (sent) => { if (sent.method() === "GET" && sent.url().endsWith(deploymentPath)) pending.add(sent); });
  page.on("requestfinished", (sent) => pending.delete(sent));
  page.on("requestfailed", (sent) => pending.delete(sent));
  await openConsole(page, request, "nodes");
  await expect(rollout(page)).toBeVisible();
  const before = await inventory(page);
  expect(before.allocations.length).toBeGreaterThan(0);
  await setDeployment(request, { resources: { allocations: before.allocations.length, pending: 0 } });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await page.getByRole("button", { name: "Change resources", exact: true }).click();
  await page.getByRole("button", { name: /^Large/ }).click();
  const saved = await saveConfiguration(page);
  expect(saved.response.ok()).toBe(true);
  expect(saved.input).toMatchObject({ provider: "docker", expected_generation: 1 });
  const current = await saved.response.json() as SandboxDeployment;
  expect(current.generation).toBe(2);
  expect(current.resources.allocations).toBe(before.allocations.length);
  await expect(rollout(page)).toContainText("Core is preparing the target configuration.");
  await expect(fact(rollout(page), "Previous-generation sandboxes")).toHaveText(String(before.allocations.length));
  const after = await inventory(page);
  expect(after.nodes.map((node) => node.id)).toEqual(before.nodes.map((node) => node.id));
  expect(after.allocations).toEqual(before.allocations);
  expect(after.nodes.map((node) => node.rollout.ready_generation)).toEqual(before.nodes.map((node) => node.rollout.ready_generation));

  // Explicit Core observations settle preparation; old Session ownership remains.
  await setNode(request, { id: "node-local", rollout: { state: "ready", ready_generation: 2 } });
  await setNode(request, { id: "node-gpu", rollout: { state: "failed", ready_generation: 1, diagnostic: "runtime_image_unavailable" } });
  await setNode(request, { id: "node-edge", rollout: { state: "unknown", ready_generation: 1 } });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(rollout(page)).toContainText("No active preparation");
  await expect(fact(rollout(page), "Previous-generation sandboxes")).toHaveText(String(before.allocations.length));
  await expect(fact(rollout(page), "Preparation failed")).toHaveText("1");
  await expect(fact(rollout(page), "Target readiness unknown")).toHaveText("1");
  await expect(page.getByRole("button", { name: "Change resources", exact: true })).toBeEnabled();
  await rollout(page).screenshot({ path: info.outputPath("generation-settled-with-old-resources.png") });
  // Watch longer than the five-second preparation interval. Retained resources
  // are not a reason to keep that interval alive; ordinary thirty-second reads remain.
  await expect.poll(() => pending.size).toBe(0);
  const rapidRead = await page.waitForRequest((sent) => sent.method() === "GET" && sent.url().endsWith(deploymentPath), { timeout: 6500 }).then(() => true, () => false);
  expect(rapidRead).toBe(false);
  expect(await writes(request)).toEqual([`PUT ${deploymentPath}`]);
});

test("unknown target preparation preserves live old-generation service while an offline pin stays offline", async ({ page, request }) => {
  await openConsole(page, request, "nodes", { fresh: true });
  await setDeployment(request, { generation: 2 });
  await setNode(request, { id: "node-local", online: true, provider_ready: true, rollout: { state: "unknown", ready_generation: 1 } });
  await setNode(request, { id: "node-edge", online: false, provider_ready: true, rollout: { state: "unknown", ready_generation: 1 } });
  await setNode(request, { id: "node-gpu", rollout: { state: "update_required", ready_generation: 1 } });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(fact(rollout(page), "Node update required")).toHaveText("1");
  const serving = page.getByRole("row", { name: /core-01/ });
  await expect(serving).toContainText("Available");
  await expect(serving).toContainText("Target readiness unknown");
  const row = page.getByRole("row", { name: /edge-03/ });
  await expect(row).toContainText("Offline");
  await expect(row).not.toContainText("Ready for target");
  await page.getByRole("button", { name: "Open edge-03", exact: true }).click();
  await expect(page.getByRole("heading", { name: "edge-03", exact: true })).toBeVisible();
  const facts = page.locator(".resource-facts");
  await expect(facts.locator("dt").filter({ hasText: /^Serving generation/ }).locator("..").locator("dd")).toHaveText("1");
  await expect(facts.locator("dt").filter({ hasText: /^Target preparation/ }).locator("..").locator("dd")).toContainText("Target readiness unknown");
  await expect(facts.locator("dt").filter({ hasText: /^Status$/ }).locator("..")).toContainText("Offline");
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  const setup = page.getByRole("region", { name: "Getting started" }).getByRole("listitem").filter({ hasText: "Get sandboxes ready" });
  await expect(setup).toContainText("Done");
  expect(await writes(request)).toEqual([]);
});

test("a generation-only change preserves old allocation evidence through failed inventory reads", async ({ page, request }) => {
  await openConsole(page, request, "nodes?id=node-local");
  const table = page.getByRole("table", { name: "Sandbox allocations", exact: true });
  await expect(table).toBeVisible();
  await expect(table.getByRole("columnheader", { name: "Configuration generation", exact: true })).toBeVisible();
  const rows = table.getByRole("row");
  const count = await rows.count();
  expect(count).toBeGreaterThan(1);
  const original = await table.getByRole("rowheader").allTextContents();
  await page.route("**/core/v1/sandbox/nodes", (route) => route.fulfill({ status: 503, json: { error: { type: "server_error", code: null, message: "Node inventory unavailable.", param: null } } }));
  await setDeployment(request, { generation: 2 });
  await page.getByRole("button", { name: "Refresh sandbox state", exact: true }).click();
  await expect(page.getByText("Node state could not be read", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "core-01", exact: true })).toBeVisible();
  await expect(rows).toHaveCount(count);
  expect(await table.getByRole("rowheader").allTextContents()).toEqual(original);
  for (const row of (await rows.all()).slice(1)) await expect(row.getByRole("cell").first()).toHaveText("1");
  expect(await writes(request)).toEqual([]);
});

test("E2B omitted-key updates keep the saved key while explicit same-key replacements each advance generation", async ({ page, request }) => {
  await openConsole(page, request, "nodes", { sandbox: "e2b" });
  await expect(rollout(page)).toBeVisible();
  const initial = await deploymentRead(page);
  await editE2B(page);
  const omitted = await saveConfiguration(page);
  expect(omitted.input).toMatchObject({ provider: "e2b", expected_generation: 1, e2b: { template: initial.e2b!.template } });
  expect(omitted.input.e2b).not.toHaveProperty("api_key");
  expect((await omitted.response.json()).generation).toBe(1);
  await page.getByRole("button", { name: "Cancel editing", exact: true }).click();
  const key = "fixture-same-team-key";
  for (const generation of [1, 2]) {
    await editE2B(page, key);
    const explicit = await saveConfiguration(page);
    expect(explicit.input).toMatchObject({ provider: "e2b", expected_generation: generation, e2b: { template: initial.e2b!.template, api_key: key } });
    const current = await explicit.response.json() as SandboxDeployment;
    expect(current.generation).toBe(generation + 1);
    expect(current.resources).toEqual(initial.resources);
    await expect(fact(rollout(page), "Target generation")).toHaveText(String(generation + 1));
  }
  const storage = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
  expect(storage).not.toContain(key);
  expect(await page.content()).not.toContain(key);
  expect(await writes(request)).toEqual(Array(3).fill(`PUT ${deploymentPath}`));
});

test("a different E2B team leaves the committed configuration intact and requires a deliberate reset", async ({ page, request }) => {
  await openConsole(page, request, "nodes", { sandbox: "e2b" });
  await expect(rollout(page)).toBeVisible();
  const before = await deploymentRead(page);
  await editE2B(page, "fixture-other-team-key");
  const rejected = await saveConfiguration(page);
  expect(rejected.response.status()).toBe(409);
  await expect(page.locator(".wizard-rejection")).toContainText("Reset before changing teams.");
  await expect(page.locator(".wizard-rejection")).toContainText("the saved configuration is unchanged");
  expect(await deploymentRead(page)).toEqual(before);
  await expect(page.getByRole("button", { name: "Save configuration", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Keep saved key", exact: true })).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(await writes(request)).toEqual([`PUT ${deploymentPath}`]);
  await page.getByRole("button", { name: "Cancel editing", exact: true }).click();
  await page.getByRole("button", { name: "Reset deployment", exact: true }).click();
  const reset = page.getByRole("dialog", { name: "Reset sandbox deployment?", exact: true });
  await expect(reset).toContainText("Archived Sessions cannot be resumed");
  expect(await writes(request)).toEqual([`PUT ${deploymentPath}`]);
  await reset.getByRole("button", { name: "Back", exact: true }).click();
  await editE2B(page);
  expect(await writes(request)).toEqual([`PUT ${deploymentPath}`]);
});
