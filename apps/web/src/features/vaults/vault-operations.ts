import type { AgentCore, Vault } from "@agents-core-web/agents-client";

import type { VaultMetadata } from "./vault-metadata";
import i18n from "../../i18n";

function metadataMatches(actual: VaultMetadata, expected: VaultMetadata): boolean {
  return Object.keys(actual).length === Object.keys(expected).length &&
    Object.entries(expected).every(([key, value]) => (
      Object.hasOwn(actual, key) && actual[key] === value
    ));
}

/**
 * Performs exactly one Vault create request and verifies the returned public
 * metadata. Reconciliation after an uncertain result remains App-owned.
 */
export async function requestVaultCreate(
  core: Pick<AgentCore, "createVault">,
  name: string,
  metadata: VaultMetadata,
): Promise<Vault> {
  const created = await core.createVault({ name, metadata });
  if (created.name !== name || !metadataMatches(created.metadata, metadata)) {
    throw new Error(i18n.t("errors.mismatchedMetadata", { ns: "vaults" }));
  }
  return created;
}
