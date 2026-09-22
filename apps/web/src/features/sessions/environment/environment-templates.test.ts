import { describe, expect, it } from "vitest";

import { AgentCoreError } from "@agents-core-web/agents-client";
import type { AgentCore, EnvironmentTemplate } from "@agents-core-web/agents-client";

import {
  environmentTemplateLabel,
  environmentTemplatesUnsupported,
  hostedNetworkNarrowingBlocker,
  loadEnvironmentTemplateCatalog,
} from "./environment-templates";

const templateId = "3f9c1d2e-4b5a-4c7d-8e9f-0a1b2c3d4e5f";

function template(overrides: Partial<EnvironmentTemplate> = {}): EnvironmentTemplate {
  return {
    id: templateId,
    object: "agent.environment.template",
    name: "Restricted outbound access",
    network: { access: "disabled", allowed_domains: [] },
    capability_directories: [],
    packages: { npm: [], python: [], system: [] },
    files: [],
    plugins: [],
    skills: [],
    created_at: 1_700_000_000,
    updated_at: 1_700_000_000,
    ...overrides,
  };
}

function core(listEnvironmentTemplates: AgentCore["listEnvironmentTemplates"]): AgentCore {
  return { listEnvironmentTemplates } as unknown as AgentCore;
}

describe("loadEnvironmentTemplateCatalog", () => {
  it("returns the listed Templates when Core exposes the resource", async () => {
    const catalog = await loadEnvironmentTemplateCatalog(core(async () => ({
      object: "list",
      data: [template()],
      has_more: false,
      first_id: templateId,
      last_id: templateId,
    })));

    expect(catalog).toEqual({ state: "ready", templates: [template()] });
  });

  it("treats a Core build without the resource as unsupported, never as empty configuration", async () => {
    for (const status of [400, 404, 405, 501]) {
      const catalog = await loadEnvironmentTemplateCatalog(core(async () => {
        throw new AgentCoreError("Invalid resource identifier or request limits.", status, "invalid_request");
      }));
      expect(catalog).toEqual({ state: "unsupported" });
    }
  });

  it("keeps an authorization or server failure explicit", async () => {
    const catalog = await loadEnvironmentTemplateCatalog(core(async () => {
      throw new AgentCoreError("Unauthorized.", 401, "invalid_api_key");
    }));

    expect(catalog).toEqual({ state: "failed", message: "Unauthorized." });
  });

  it("propagates an aborted read instead of reporting a capability", async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(loadEnvironmentTemplateCatalog(
      core(async () => { throw new DOMException("Aborted", "AbortError"); }),
      controller.signal,
    )).rejects.toBeInstanceOf(Error);
  });
});

describe("Template presentation and narrowing", () => {
  it("labels a Template with its saved network policy", () => {
    expect(environmentTemplateLabel(template())).toBe("Restricted outbound access · network disabled");
    expect(environmentTemplateLabel(template({ name: null }))).toBe("Unnamed Template · network disabled");
  });

  it("blocks widening a disabled Template and allows inheriting or narrowing", () => {
    expect(hostedNetworkNarrowingBlocker(template(), "enabled")).toContain("never widen");
    expect(hostedNetworkNarrowingBlocker(template(), "disabled")).toBeNull();
    expect(hostedNetworkNarrowingBlocker(template(), null)).toBeNull();
    expect(hostedNetworkNarrowingBlocker(template({ network: { access: "enabled", allowed_domains: [] } }), "disabled")).toBeNull();
    expect(hostedNetworkNarrowingBlocker(null, "enabled")).toBeNull();
  });

  it("recognizes only Core error statuses as an absent resource", () => {
    expect(environmentTemplatesUnsupported(new AgentCoreError("gone", 404))).toBe(true);
    expect(environmentTemplatesUnsupported(new AgentCoreError("boom", 500))).toBe(false);
    expect(environmentTemplatesUnsupported(new Error("network down"))).toBe(false);
  });
});
