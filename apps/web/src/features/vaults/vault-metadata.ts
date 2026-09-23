import i18n from "../../i18n";

const tv = (key: string) => i18n.t(key as never, { ns: "vaults" });

export const VAULT_METADATA_MAX_BYTES = 64 * 1024;

export type VaultMetadata = Record<string, string>;

export type VaultMetadataValidationCode = "invalid_json" | "invalid_shape" | "too_large";

export class VaultMetadataValidationError extends Error {
  readonly code: VaultMetadataValidationCode;

  constructor(code: VaultMetadataValidationCode, message: string) {
    super(message);
    this.name = "VaultMetadataValidationError";
    this.code = code;
  }
}

function encodedBytes(value: string): number {
  return new TextEncoder().encode(value).length;
}

/**
 * Parses the public metadata accepted while creating a Vault.
 *
 * Core exposes metadata as a string-to-string map. Blank or omitted input is
 * therefore the empty map; every other JSON root and every non-string value is
 * rejected rather than silently coerced. The size limit applies to the JSON
 * value sent to Core, not editor whitespace.
 */
export function parseVaultMetadata(source?: string): VaultMetadata {
  if (source === undefined || source.trim() === "") return {};

  let parsed: unknown;
  try {
    parsed = JSON.parse(source);
  } catch {
    throw new VaultMetadataValidationError(
      "invalid_json",
      tv("errors.metadataJson"),
    );
  }

  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new VaultMetadataValidationError(
      "invalid_shape",
      tv("errors.metadataShape"),
    );
  }

  const entries = Object.entries(parsed);
  if (entries.some(([, value]) => typeof value !== "string")) {
    throw new VaultMetadataValidationError(
      "invalid_shape",
      tv("errors.metadataStrings"),
    );
  }

  const metadata = parsed as VaultMetadata;
  if (encodedBytes(JSON.stringify(metadata)) > VAULT_METADATA_MAX_BYTES) {
    throw new VaultMetadataValidationError(
      "too_large",
      tv("errors.metadataLarge"),
    );
  }

  return metadata;
}
