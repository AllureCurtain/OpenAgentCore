export type HostShell = "posix" | "powershell";
export type ExecutorInstall = { kind: "unavailable" } | { kind: "ready"; commands: Record<HostShell, string> };

/** Quote Core-owned values as literal arguments; credential contents never enter a command. */
function quote(value: string, shell: HostShell): string {
  return "'" + value.replaceAll("'", shell === "powershell" ? "''" : "'\\''") + "'";
}

/** Native installation needs Session connection facts, not console installer assets. */
export function executorInstall({ environmentId, remoteUrl, workspaceDirectory }: {
  environmentId: string; remoteUrl: string; workspaceDirectory: string;
}): ExecutorInstall {
  if (!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(environmentId) || !workspaceDirectory || /[\x00-\x1f\x7f]/.test(workspaceDirectory + remoteUrl)) return { kind: "unavailable" };
  try {
    const url = new URL(remoteUrl);
    const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname);
    if (url.username || url.password || url.hash || !(url.protocol === "wss:" || (url.protocol === "ws:" && loopback))) return { kind: "unavailable" };
  } catch { return { kind: "unavailable" }; }
  const command = (shell: HostShell) => `${shell === "powershell" ? ".\\oac-daemon.exe" : "./oac-daemon"} install --interactive --remote ${quote(remoteUrl, shell)} --environment-id ${quote(environmentId, shell)} --workspace ${quote(workspaceDirectory, shell)}`;
  return { kind: "ready", commands: { posix: command("posix"), powershell: command("powershell") } };
}
