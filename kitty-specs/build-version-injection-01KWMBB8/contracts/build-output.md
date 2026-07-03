# Contract — Build Provenance in Output

Specification-by-example. Concrete before/after for every surface that exposes version.

## CLI: `version` command

**Before**
```
$ spec-kitty-analyzer version
spec-kitty-analyzer 0.2.0
```

**After — release build**
```
$ spec-kitty-analyzer version
spec-kitty-analyzer 0.3.0 (commit a1b2c3d, built 2026-07-03T18:00:00Z)
```

**After — local/dev build**
```
$ go run ./cmd/spec-kitty-analyzer version
spec-kitty-analyzer dev (commit none, built unknown)
```

## JSON: `analyze` report / `query` result / `missions` index

The `build` object appears at the top of each output. The former top-level `version` key is **removed**.

**Before (pre-0.3.0)**
```json
{
  "version": "0.2.0",
  "generated_at": "2026-07-03T18:00:00Z",
  "...": "..."
}
```

**After (0.3.0+) — release build**
```json
{
  "build": {
    "version": "0.3.0",
    "commit": "a1b2c3d",
    "build_date": "2026-07-03T18:00:00Z"
  },
  "generated_at": "2026-07-03T18:00:00Z",
  "...": "..."
}
```

**After — local/dev build**
```json
{
  "build": { "version": "dev", "commit": "none", "build_date": "unknown" },
  "...": "..."
}
```

## Breaking-change contract (FR-005)

| Consumer expectation | Pre-0.3.0 | 0.3.0+ |
|----------------------|-----------|--------|
| Read analyzer version | `report.version` | `report.build.version` |
| Read build commit | (unavailable) | `report.build.commit` |
| Read build date | (unavailable) | `report.build.build_date` |

**Migration for JSON consumers**: replace any read of top-level `.version` with `.build.version`. This is the documented breaking change shipping in 0.3.0.

## Acceptance assertions (tests)

- `CurrentBuild()` with no injection returns `{dev, none, unknown}`.
- Marshaled `Report`, `missions`, and `query` JSON each contain `build.version`, `build.commit`, `build.build_date`.
- Marshaled JSON for all three contains **no** top-level `version` key (guards FR-005).
