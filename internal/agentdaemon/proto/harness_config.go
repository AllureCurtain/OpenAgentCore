package proto

// HarnessConfig is the shared JSON-object wire shape for native model parameters.
// The selected Harness owns its field semantics; no cross-Harness translation is
// implied. Omission and {} mean empty configuration; null, arrays and encoded
// objects larger than 16 KiB of serialized UTF-8 bytes are invalid. Unknown and reserved fields must fail
// before model input, without echoing submitted keys or values. Connection,
// authentication, execution policy and lifecycle are not configurable here.
// Runtime applies a copied Session snapshot only after modeltransport has prepared
// the final provider connection. The Executor retains the logical configuration
// across Turns; local proxy addresses and credentials may be regenerated on rebuild.
type HarnessConfig = map[string]any
