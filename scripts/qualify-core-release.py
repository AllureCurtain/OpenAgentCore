#!/usr/bin/env python3
"""Reserved entrypoint for this batch's real fresh-install qualification.

The controller executes these reviewed bytes over authenticated SSH. stdin is one
JSON request with source, tree, run_id, directory, inventory, inventory_sha256,
adapter_sha256 and required_checks. The directory contains the exact downloaded
release assets. The implementation must verify their hashes before and after real
checks and freshly extract them into newly owned installation resources. stdout
must contain one JSON response echoing the identity fields and checks mapping each
required check to passed. Send only redacted diagnostics to stderr. A failed or
skipped required check must exit nonzero. Never load completion from a supplied
pass file. Resources must follow the batch's separately agreed ownership bounds.

This interface deliberately cannot pass until the actual acceptance commands are
integrated and independently reviewed. No synthetic fallback is provided.
"""

import json
import sys


if __name__ == "__main__":
    if sys.argv[1:] == ["--describe"]:
        print(json.dumps({"ready": False, "required_checks": [
            "fresh-install", "current-lifecycle", "managed-native",
            "current-generations", "node-runtime", "diagnostics-observations",
        ]}))
        sys.exit(0)
    sys.exit("Live qualification adapter is not connected; publication is blocked")
