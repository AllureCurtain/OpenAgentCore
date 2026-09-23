import { isLoopbackHostname, isValidDirectCoreBaseUrl } from "../../lib/connection";

export function sandboxCoreOrigin(value: string): string | null {
  const candidate = value.trim();
  if (!/^https?:\/\/[^/?#\\\s]+\/?$/i.test(candidate) || !isValidDirectCoreBaseUrl(candidate)) return null;
  const url = new URL(candidate);
  if (url.protocol === "http:" && url.hostname.endsWith(".localhost")) return null;
  return url.origin;
}

export function sandboxSetupOrigin(value: string): string | null {
  const origin = sandboxCoreOrigin(value);
  if (!origin) return null;
  const url = new URL(origin);
  return url.protocol === "https:" && !isLoopbackHostname(url.hostname.replace(/\.$/, "")) ? origin : null;
}
