# Performance profile and improvement plan

Measured on the Stripe spec (6.6 MB, 431 paths, 612 operations) with `genbench`, generating server, client and models (37.9 MB of Go). The libopenapi engine is the only one, `GOMAXPROCS=8`, Go 1.27.3. Re-run with:

```bash
go run ./cmd/genbench -spec <stripe spec> -cpuprofile cpu.prof -memprofile mem.prof
go tool pprof -top -cum cpu.prof
```

## Results

| Phase | Time |
| --- | --- |
| Load (libopenapi) | 0.94 s |
| Generate (includes goimports) | 10.4 s |
| Total, median of 12 | 10.3 s (mean 11.3 s ± 1.7 s; one 19 s outlier) |

Loading is under 10% of the run. Everything worth doing is in generation. The shares below are of total CPU time in the profile (182 s over 13 runs, 133% of wall time because the garbage collector runs on other cores). Nested entries overlap, so they do not add up to 100%.

| Where the time goes | CPU share | Notes |
| --- | --- | --- |
| Garbage collection | 33% | 86 GB allocated across the 13 runs, about 6.6 GB per run for a 38 MB output |
| `generateUnions` → `closeUnionVariants` → `variantKeys` | 23% | Builds every union variant's complete Go schema again, only to read its JSON keys. 44% of all bytes allocated |
| goimports (`imports.Process`, of which `go/format` is half) | 19% | Parses and prints a 38 MB file |
| `OperationDefinitions` | 9% | Builds each operation's parameter and body types |
| `Schema.HasCustomMarshalJSON` | 9% | Calls `GenerateGoSchema` again each time a template asks |
| `text/template` evaluation | 11% | Reflection-based calls into methods such as the one above |
| `structFieldTags` | 6% of allocation | Runs a template for every struct field's tags |
| `strings.NewReplacer` in `template_helpers.go` | 9% of allocation | A replacer is built on every call to a helper |
| `GenerateEnums` | 4% | |

## Progress

Same spec and machine, 6 samples after 1 warmup round each. Output is the same Go code at every step, checked by the golden files and, for each step that rewrites a pass, by a test that runs the old and new paths side by side.

| After | Median | Allocations per run | Allocated per run |
| --- | --- | --- | --- |
| Baseline | 10.27 s | 75.6 million | 6,611 MB |
| Steps 1 to 3 (replacer, marshal check, struct tags) | 7.76 s | 56.2 million | 4,819 MB |
| Step 4 (`variantKeys` from the spec for plain objects) | 6.01 s | 35.0 million | 3,105 MB |
| Step 6 (format in parallel, `pkg/codegen/format.go`) | 3.72 s | 32.9 million | 2,792 MB |
| Loader (one parse for documents that declare `openapi`) | 2.85 s | 29.1 million | 2,611 MB |
| Format without a whole-file parse (`cutPoints`, `formatPiece`) | 2.5 s (3.1 s on the same run, same load) | 28.9 million | 2,568 MB |

The loader had parsed the 6.6 MB document two more times, in `declaredSwagger` and `withOpenAPIVersion`, to look for the `swagger` and `openapi` keys. A document that declares an OpenAPI 3 version is now read with the one parse libopenapi makes, and the other paths (Swagger, no version) are unchanged. Load fell from 0.93 s to 0.43 s.

## The pattern behind most of it

The generator derives the same facts from a schema several times, by running the whole schema-to-Go-type step again:

- the type's own generation,
- `variantKeys`, with a fresh context per variant (the cache is only per `*model.Schema`, and each variant still walks its entire subtree),
- `generatesMarshalJSON`, with a call to `GenerateGoSchema` per template question,
- `aliasChain`, which generates each level of an alias again.

Together the first three re-derivations account for about a third of CPU, and more of the allocation. They exist because there is no stored Go-level result to read. This is the shape the target architecture removes: lower each schema once into a Go type graph keyed by canonical ID, and have every later question read that graph.

## Plan

Each step must leave the golden files unchanged. Gains are estimates from the profile shares, not measurements.

### Phase 1: remove repeated work without changing structure (about 1 to 2 days each)

1. **Hoist the `strings.NewReplacer`** in `template_helpers.go:344` to a package variable. Trivial, and removes about 9% of allocation.
2. **Compute `generatesMarshalJSON` once per schema.** Store the answer on the `Schema` when `GenerateGoSchema` builds it (it already knows its properties, unions and additional properties) and make `HasCustomMarshalJSON` a field read. Expected: up to 9% of CPU and the GC pressure that comes with it.
3. **Compile struct tags once per field shape.** `structFieldTags` runs a template per field; most fields use the default tag set. Cache the compiled output by (tag config, field name, required, omit rules), or generate the default tags with plain string building and keep the template only for user-defined tags. Expected: about 6% of allocation, and part of the template share.
4. **Make `variantKeys` read the spec, not generate a type.** The keys a variant marshals as come from its property names, its allOf members and the `json` tag rules in `fieldKey`. Walk the model schema directly, with a cache keyed by `*model.Schema`, and drop the fresh `genContext` and `aliasChain`. Check equivalence by running both paths over all fixtures and Stripe and comparing keys before removing the old one. Expected: most of the 23%, and up to 44% of allocation.

### Phase 2: stop paying for goimports (about 19%)

5. **Skip the second parse.** `imports.Process` parses and prints the generated file to fix imports and formatting. The generator already knows its imports (`GenerateImports` builds them from what the output uses). Options, cheapest first:
   - run `go/format` only (about half the cost) and let `GenerateImports` emit the exact set of imports, then check in a test that `imports.Process` would add or remove none;
   - emit already-formatted code from the templates for the large regular parts (the type declarations), so the formatter has less to fix.
6. **Format in parallel.** Split the output at top-level declaration boundaries, format the chunks concurrently, and join them. `go/format` is per file, so this is a self-contained change at the end of `Generate`. Expected on 8 cores: most of the remaining formatting time.

### Phase 3: architecture, using the freedom the kin removal gave us

These follow `docs/target-architecture.md`; the profile says which steps pay first.

7. **Lower each schema once, keyed by canonical ID** (document path plus JSON pointer from the libopenapi index). Cache the resulting Go schema. Every consumer (`variantKeys`, marshal checks, enum collection, imports gathering, operation definitions) reads it. This removes the repeated generation by construction and is what makes steps 2 and 4 permanent instead of patched. It needs the context-dependence of today's `GenerateGoSchema` (name hints, `globalState`) made explicit first, which is step 8.
8. **Replace `globalState` with a context value** (about 100 reads). A prerequisite for step 7 and for running anything in parallel.
9. **Generate operations and component types in parallel.** After step 8 the passes over components, operations, client, server and boilerplate only read shared state; run them as separate goroutines and join the output in the existing order. The output order is fixed, so determinism is unaffected. Expected: wall time falls toward the slowest pass, not the sum.
10. **Reduce allocation in the hot path.** `generateGoSchema` itself accounts for 12% of allocation (`slices.Clone` of path slices, per-call struct copies). Pass paths as a persistent linked list or an interned ID instead of cloning slices. Do this after step 7, when the call count has dropped, and measure first.

### Phase 4: loader and tooling

11. **Load.** 0.9 s is fine. The remaining cost is libopenapi's schema building (about 2% of allocation). Revisit only if steps 1 to 9 leave it as the largest piece.
12. **Keep the benchmark honest.** Add the Stripe and Twilio specs to a scheduled CI job (not the pull request path) that records `genbench` results, so regressions show up. Report allocation per run next to time.

## Suggested order

Do steps 1 and 2 first: they are small and local. Then step 4, the largest single item, with the equivalence check. Then step 5 or 6 for the formatter. Re-profile after each, because the shares will move (garbage collection falls with allocation). Steps 7 to 9 are the larger refactor; start them once the profile after step 6 shows what is left.

A realistic target, from the profile alone: Stripe generation from about 10 s to 4 to 5 s after steps 1 to 6, and lower again after steps 7 to 9. These are estimates; each step should be measured with the benchmark before it is committed.

## Where it stands, and what is left

Stripe now takes a median of 2.7 s, from 10.3 s. Splitting that wall time (`genbench -skip-fmt` times everything but the formatter):

| Part | Wall time | Notes |
| --- | --- | --- |
| Load | 0.4 s | libopenapi parse and build, then conversion to the model |
| Generation, without formatting | 1.2 s | One thread |
| Formatting | 1.0 s | Half of it is one thread: parsing the whole file to find declaration boundaries and the imports |

The CPU profile adds: garbage collection is 24% of CPU time, `go/parser` is 11% of CPU and 22% of allocation, and `GenerateEnums` is 8%. Steps 1 to 6 of the plan are done. Most of what the plan said about generating schemas again (step 7) is now done by the targeted caches; `generateGoSchema` is 2.5% of CPU.

Rejected: raising `GOGC`. 400 gives about 7% (2.62 s to 2.44 s) for 400 MB more memory, and 800 gives nothing more. Not worth a default.

### Remaining steps, in the order I would take them

Estimates are from the profile, to be measured as before.

1. **Format without parsing the whole file (about 0.5 to 0.8 s). Done: 3.1 s to 2.5 s in an interleaved A/B on a busy machine (the formatter alone: 0.93 s to 0.4 s).**
   - `Generate` already holds the types, client, server and boilerplate as separate strings, so format each in parallel and skip the serial parse.
   - The imports come from a scan of the output for `pkg.Name` selectors; `go/scanner` is much cheaper than `go/parser`.
   - It also removes the second parse of each piece inside `format.Source`, which is most of the 22% of allocation in the parser.
   - The existing test, which compares the bytes with `imports.Process`, is the check.
2. **Enums (about 0.3 s).**
   - `GenerateEnums` recomputes each enum's constant names several times (`GetValues` in the conflict passes and again in the template) and converts names with `ToCamelCase` without caching.
   - Compute the names once per enum and memoize the name conversion.
3. **Run the independent passes at the same time (about 0.4 to 0.5 s).**
   - After operations are built, the component types, operation types, boilerplate, server, client and client-with-responses passes only read shared state. Their output is joined in a fixed order, so the result stays deterministic.
   - First, the state they write lazily has to be made safe: the marshal-JSON cache, `componentNames`, `typeMappingImports`, `usedFormatTypes`, `warnedFieldNameCollisions` and the struct-tag generator. Build them before the fan-out, or guard the few that must change, as the struct-tag cache already is.
   - Replacing `globalState` as a whole (step 8) is the larger change and is not needed for this.
4. **Loader (about 0.1 to 0.2 s).** Converting libopenapi's model into the generator's model is 0.5 s of CPU and 8% of allocation. Convert lazily, or in parallel by path.
5. **Allocation in the schema walk, and canonical IDs (steps 7 and 10).** Leave until the profile after steps 1 to 4 shows them as the largest remaining part.

If steps 1 to 3 give what the profile suggests, Stripe would be about 1.6 to 1.8 s.

### Notes on the formatting step

- The first version of the cut scan only recognised a `func` or `type` straight after the blank line. Nearly every declaration has a doc comment, so the scan found almost no cuts in the client and types sections: the 46 pieces included ones of 12.9, 7.3 and 4.3 MB, and the run was no faster. Skipping comment lines before the keyword gave 139 pieces, the largest 1.1 MB. Look at the piece sizes, not only the total, when a parallel step doesn't speed up.
- A wrong cut cannot slip through: a piece cut inside a declaration doesn't parse, and the whole source then goes to `imports.Process`. The test requires that no fixture falls back.
- Wall-clock timings here need a quiet machine. Compare binaries interleaved in one session, as `ab.sh`-style loops do, and check the load average.
