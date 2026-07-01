# Public API Inventory Tooling

Status: open

The Board/API coverage work has checked-in Rust and TypeScript inventory artifacts under
`docs/maintainer/tickets/board-api-coverage/`, but the regeneration path must not depend
on ad-hoc scripts in `/tmp` or any other local scratch path.

Build checked-in maintainer tooling for regenerating the public API inventories used by
Board coverage work.

Current inputs to replace with reproducible tooling:

- `docs/maintainer/tickets/board-api-coverage/api_inventory_rust.txt`
- `docs/maintainer/tickets/board-api-coverage/api_inventory_ts.txt`

Requirements:

- The command lives in normal repo tooling, such as `xtask`, rather than a temporary local
  script.
- The command regenerates the same inventory files deterministically from the current
  tree.
- The command records enough provenance in the generated files for a future maintainer to
  know what tree/date/tool produced them.
- The command does not turn inventory generation into API adjudication. Inventories are
  mechanical inputs; Board coverage still has to be reconciled by humans and tests.

Done means:

- A future agent can regenerate the inventory files from a clean checkout using a
  checked-in command.
- The command is documented wherever the Board/API coverage work tells agents to refresh
  inventories.
