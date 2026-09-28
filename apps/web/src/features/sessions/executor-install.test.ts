import { describe, expect, it } from "vitest";
import { executorInstall } from "./executor-install";
const input = { environmentId: "5b1e2c3d-0000-4000-8000-000000000001", remoteUrl: "wss://core.example/api/v1/agent-daemon/ws", workspaceDirectory: "/srv/work" };
describe("native host installation", () => {
  it("uses Session facts without installer assets, credentials or automatic start", () => {
    const result = executorInstall(input);
    expect(result.kind).toBe("ready");
    if (result.kind !== "ready") return;
    expect(result.commands.posix).toBe(`./oac-daemon install --interactive --remote '${input.remoteUrl}' --environment-id '${input.environmentId}' --workspace '/srv/work'`);
    expect(result.commands.powershell).toContain(".\\oac-daemon.exe install --interactive");
    expect(result.commands.posix).not.toMatch(/python|docker|token|start/);
  });
  it("quotes shell metacharacters in workspace paths as literal values", () => {
    const result = executorInstall({ ...input, workspaceDirectory: "/srv/it's $HOME `work`" });
    if (result.kind !== "ready") throw new Error("expected command");
    expect(result.commands.posix).toContain("--workspace '/srv/it'\\''s $HOME `work`'");
    expect(result.commands.powershell).toContain("--workspace '/srv/it''s $HOME `work`'");
  });
  it.each(["ws://localhost:8091/ws", "ws://127.0.0.1:8091/ws", "ws://[::1]:8091/ws"])("accepts native loopback connection %s", remoteUrl => {
    expect(executorInstall({ ...input, remoteUrl }).kind).toBe("ready");
  });
  it.each([{ remoteUrl: "ws://core.example/ws" }, { remoteUrl: "https://core.example" }, { remoteUrl: "wss://user:secret@core.example" }, { environmentId: "" }, { workspaceDirectory: "" }, { workspaceDirectory: "/srv/\nwork" }])("withholds commands with invalid facts %j", change => {
    expect(executorInstall({ ...input, ...change })).toEqual({ kind: "unavailable" });
  });
});
