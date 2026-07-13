# Data Model — Build Version & Metadata Injection

## Entity: `Build`

The provenance of a compiled binary. A single cohesive value object; one source of truth for "what am I and where did I come from."

| Field | JSON key | Type | Source | Default (local build) |
|-------|----------|------|--------|-----------------------|
| Version | `version` | string | `-ldflags -X ...analyzer.Version` (= git tag minus `v`) | `dev` |
| Commit | `commit` | string | `-ldflags -X ...analyzer.Commit` (= `git rev-parse --short HEAD`) | `none` |
| BuildDate | `build_date` | string (UTC ISO-8601) | `-ldflags -X ...analyzer.BuildDate` (= `date -u +%Y-%m-%dT%H:%M:%SZ`) | `unknown` |

### Go representation (illustrative)

```go
// internal/analyzer/types.go
var (
    Version   = "dev"     // overridden at release build via -ldflags -X
    Commit    = "none"
    BuildDate = "unknown"
)

type Build struct {
    Version   string `json:"version"`
    Commit    string `json:"commit"`
    BuildDate string `json:"build_date"`
}

func CurrentBuild() Build { return Build{Version, Commit, BuildDate} }
```

### Invariants

- **INV-1**: The three package vars are the sole source; `CurrentBuild()` is the only constructor. No call site sets these fields independently.
- **INV-2**: A binary reports either a full release triple (all three injected) or the full sentinel triple — never a real version with `none`/`unknown` provenance from a release build.
- **INV-3**: `Build` is emitted as a nested object under the `build` key; `version` never appears as a top-level JSON field (breaking contract vs. pre-0.3.0).

### Consumers (nest the `build` object under `json:"build"`, as the FIRST field)

Each emitter replaces its current first field `Version string json:"version"` with `Build Build json:"build"`, keeping `Build` first so the `build` object leads the marshaled JSON (R8):

- `analyzer.Report` — `Version` → `Build` (first field).
- `missions` result struct (`cmd/spec-kitty-analyzer/main.go`) — same replacement.
- `query.QueryResult` — same replacement; sourced from the report's `Build`.

`Build` is a **named** field (`Build Build json:"build"`), not an embedded/anonymous field — embedding would promote `version`/`commit`/`build_date` back to the top level, which is exactly the flat shape C1 rejects.

### State transitions

None — `Build` is immutable, fixed at link time.
