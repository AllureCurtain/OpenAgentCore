import type { SandboxDeployment } from "@agents-core-web/agents-client";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { node } from "../overview/test-fixtures";
import { SandboxRolloutSummary } from "./SandboxRolloutSummary";
import { NodeRolloutStatus } from "./NodeRolloutStatus";
import { SandboxDeploymentSettings } from "./SandboxDeploymentSettings";

const deployment: SandboxDeployment = { installation_id: "i", owner_epoch: 1, generation: 4, provider: "docker", mode: "nodes", core_url: "https://core.example", resources: { allocations: 8, pending: 2 }, reset: null, suspension: null,
  rollout: { state: "settled", previous_generation_sandboxes: 8, nodes: { ready: 1, preparing: 0, failed: 2, update_required: 3, unknown: 4 } } };

describe("authoritative configuration rollout", () => {
  it("shows settled with retained work and failed/unknown nodes without claiming readiness", () => {
    const html = renderToStaticMarkup(<SandboxRolloutSummary deployment={deployment} />);
    expect(html).toContain("No active preparation");
    expect(html).toContain("Previous-generation sandboxes</dt><dd>8");
    expect(html).toContain("Preparation failed</dt><dd>2");
    expect(html).toContain("Node update required</dt><dd>3");
    expect(html).toContain("Target readiness unknown</dt><dd>4");
    expect(html).not.toContain("Core is preparing the target configuration.");
    expect(html).toContain("Review affected nodes");
  });

  it("qualifies stale preparation and keeps routine compact summaries quiet", () => {
    const preparing = renderToStaticMarkup(<SandboxRolloutSummary deployment={{ ...deployment, rollout: { ...deployment.rollout, state: "preparing" } }} stale />);
    expect(preparing).toContain("last confirmed observations");
    expect(preparing).toContain("Core is preparing the target configuration.");
    const quiet = renderToStaticMarkup(<SandboxRolloutSummary deployment={{ ...deployment, rollout: { state: "settled", previous_generation_sandboxes: 0, nodes: { ready: 2, preparing: 0, failed: 0, update_required: 0, unknown: 0 } } }} compact />);
    expect(quiet).toContain("Target generation</dt><dd>4");
    expect(quiet).not.toContain("Previous-generation sandboxes");
    expect(quiet).not.toContain("Preparation failed");
  });

  it("never promotes an offline durable serving pin into ready target status", () => {
    const pinned = { ...node("offline"), online: false, rollout: { state: "ready" as const, ready_generation: 4 } };
    const html = renderToStaticMarkup(<NodeRolloutStatus node={pinned} />);
    expect(html).toContain("Target readiness unknown");
    expect(html).not.toContain("Ready for target");
    expect(renderToStaticMarkup(<NodeRolloutStatus node={{ ...pinned, online: true }} stale />)).toContain("Target readiness unknown");
  });

  it("allows same-provider online editing with held resources but keeps reset active as a boundary", () => {
    const render = (value: SandboxDeployment) => renderToStaticMarkup(<SandboxDeploymentSettings deployment={value} fresh disabled={false} onReset={async () => true} onCancelReset={async () => true} onUpdate={async () => {}} />);
    const html = render(deployment);
    expect(html).toMatch(/<button[^>]*class="button outline"[^>]*>Change resources<\/button>/);
    expect(html).not.toMatch(/<button[^>]*disabled[^>]*>Change resources<\/button>/);
    expect(html).not.toContain("Add the nodes again after saving");
    const reset = { clear: "force" as const, requested_at: "2026-09-27T10:00:00Z", deadline_at: null, forced_at: "2026-09-27T10:00:00Z", remaining: { busy: 0, idle: 0, cleanup: 1, on_offline_nodes: 0, offline_nodes: [] } };
    expect(render({ ...deployment, reset })).not.toContain("Change resources");
  });
});
