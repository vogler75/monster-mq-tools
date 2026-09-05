# GraphQL contract tool

Developer tooling owned by the tools repository. This Go program parses and
validates broker SDL, generates documentation/introspection snapshots, validates
CLI operations, and detects source or live-schema drift. It is not part of the
broker runtime or the normal `mmq` runtime. The adjacent shell script is only a
launcher; it resolves checkout paths and runs this standalone Go module.

From the tools repository:

```bash
./scripts/graphql-contract.sh --main-root ../main --edge-root ../edge --cli-root .
./scripts/graphql-contract.sh --main-root ../main --edge-root ../edge --cli-root . --check
(cd scripts/graphql-contract && go test ./...)
```

`--main-root` defaults to the sibling `main` checkout. Edge generation and CLI validation
are optional and require their respective flags. Generated broker artifacts live
in `main/doc/graphql/main/` and `main/doc/graphql/edge/`; `--output-dir` can redirect those artifacts to a temporary or external directory.
`--cli-root` validates operations and adapters without writing anything into the CLI.

CLI tests read `MMQ_CONTRACT_DIR` (test-only) when supplied, or generate schemas
from sibling broker checkouts in a temporary directory that is removed after loading.
No schema copies are stored in the CLI repository.

Go is used to reuse a validated GraphQL parser and the CLI's existing Go toolchain.
A different implementation language would be possible, but a shell copy script
alone would not provide schema validation or compatibility checks.
