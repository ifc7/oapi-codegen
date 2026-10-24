# Code Review Rules for oapi-codegen

These rules guide automated code review for this repository. oapi-codegen is a code generator that turns OpenAPI 3.x specs into Go server stubs, clients, and types. The primary risk surface in any change is **the generated output that downstream users compile against** — review with that in mind.

## 1. Generated files (`*.gen.go`)

Files matching `*.gen.go` are produced by the code generator and committed to source control so CI can verify they are up-to-date. You do **not** need to read every generated file in a PR — spot check a representative sample.

When spot checking, look for:

- **Drift unrelated to the stated change.** If the PR claims to fix X, but a `*.gen.go` diff also shows unrelated reshuffling (renamed identifiers, reordered methods, formatting churn, removed comments, regenerated imports), call it out. This often indicates an accidental commit from a different branch, an out-of-date local toolchain, or a template change the author did not intend.
- **Diffs that look "too small" for the described change.** If the PR claims to add a new code generation feature but the only generated diff is a one-line tweak in one file, the test fixtures may not have been regenerated — flag it.
- **Diffs that look "too large" for the described change.** A small template tweak should not produce thousands of lines of churn across every fixture. If it does, the template change probably has a wider blast radius than the author realized.

You do not need to suggest stylistic improvements inside `*.gen.go` files — they are templated output, not hand-written code.

## 2. Configuration changes must update the JSON schema

Whenever `pkg/codegen/configuration.go` is modified — particularly the `Configuration`, `GenerateOptions` or `OutputOptions` structs — the corresponding JSON Schema at `configuration-schema.json` (repo root) **must** be updated to match.

Flag any PR that changes `pkg/codegen/configuration.go` without a corresponding edit to `configuration-schema.json`. The schema is consumed by IDEs and validation tooling; an out-of-sync schema silently breaks downstream users' configs.

## 3. Dependencies

- Watch out for users making `go.mod` changes which advance the Go version to a new minor release. We want these to be explicitly justified in the commit message. Maintenance version bumps are ok. Mention that the maintainers would prefer to do a minor version bump themselves.
- Watch out for unnecessary dependency changes that creep into code reviews, just because people tend to do it out of habit. Every dependency update should have a reason if it's bundled with codegen changes.

## 4. Test case placement in `internal/test/`

`internal/test/` is organized **by feature category**, not by GitHub issue. The top-level categories are:

- `aggregates/` — allOf/anyOf/oneOf composition, anonymous-schema hoisting
- `bodies/` — request/response bodies and content types
- `clients/` — client construction and options
- `events/` — webhooks and callbacks
- `extensions/` — `x-go-*`, `x-oapi-codegen-*`, `x-order`, `x-omitempty` extensions
- `naming/` — identifier generation and type-name collision handling
- `openapi31/` — OpenAPI 3.1-specific behavior
- `options/` — output-options flags (name-normalizer, filters, skip-prune, struct-tags, …)
- `parameters/` — parameter binding, styles, encoding, nil handling (incl. the `roundtrip/` harness)
- `paths/` — path-level routing edge cases: URL escaping of reserved characters and path-parameter precedence
- `references/` — external `$ref`s, import-mapping, multi-package generation, overlays
- `schemas/` — schema-to-type mapping (primitives, objects, enums, nullable, recursive, …)
- `servers/` — server codegen (routers, middleware, request and response objects)
- `spec_validation/` — pre-generation spec validation

When a PR adds a test, enforce the following:

- **Do not allow issue-numbered test directories to come back.** Flag any new directory named after a GitHub issue (`internal/test/issues/…`, `issue-1234/`, `issueNNNN/`, and similar) anywhere under `internal/test/`. The old `issues/` tree was deliberately dissolved into the categories above.
- **Prefer extending an existing test case.** If the scenario fits an existing category leaf — same OpenAPI construct, same generation config — the new schemas/operations belong in that leaf's `spec.yaml` and its `*_test.go`, marked with a provenance comment (`# From issue-NNNN: <one-line summary>`) so the issue context is not lost. Suggest the specific leaf to extend when you can identify one.
- **A new leaf is fine when nothing matches.** A new subdirectory under the right category is the correct move when the scenario needs a *different generation config* (other `generate:` targets or `output-options:`) or inherently needs its own files (multi-file external-ref layouts, or a generated client and server in separate packages, as in `parameters/roundtrip/`). It should follow the standard layout — `doc.go` (with the `//go:generate` line), `config.yaml`, `spec.yaml`, `<name>_test.go` — with a snake_case scenario-named directory (never an issue number) and the issue reference in a comment. Fixtures that generate a client and a server separately legitimately depart from this by fanning out into subpackages; do not flag that shape.
- **Regression tests for bug fixes are still expected** — they just live in the category matching the feature, not in an issue-named directory.

## 5. General

- The repo is a multi-module monorepo (the root, `internal/test` and `examples`). Cross-module changes deserve extra scrutiny.
- Generated code depends only on the standard library, plus `github.com/google/uuid`, `github.com/oapi-codegen/nullable` and `github.com/getkin/kin-openapi` when the spec or configuration calls for them. Flag a template change that adds an import beyond those.
- Generated files are committed; CI fails if `make generate` produces a diff. If a PR's generated files look stale, that will fail CI regardless — but flagging it in review saves a round-trip.
