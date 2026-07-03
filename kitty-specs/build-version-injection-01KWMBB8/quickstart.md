# Quickstart — Verifying Build Metadata Injection

## Local (dev) build — expect sentinels

```bash
go run ./cmd/spec-kitty-analyzer version
# spec-kitty-analyzer dev (commit none, built unknown)
```

## Simulate a release build locally — expect injected values

```bash
PKG=github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer
VERSION=0.3.0
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

go build -trimpath \
  -ldflags="-s -w -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.BuildDate=${DATE}" \
  -o /tmp/ska ./cmd/spec-kitty-analyzer

/tmp/ska version
# spec-kitty-analyzer 0.3.0 (commit <sha>, built <ts>)
```

**Footgun check (C-002)**: if `version` still prints `dev` after injecting, the `-X` symbol path is wrong — confirm it is the lowercase module path `github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer`.

## Verify the JSON build object + breaking change

```bash
/tmp/ska missions --limit 1 | python3 -c "import sys,json; d=json.load(sys.stdin); \
  assert 'version' not in d, 'top-level version must be gone'; \
  print('build:', d['build'])"
# build: {'version': '0.3.0', 'commit': '<sha>', 'build_date': '<ts>'}
```

## Tests

```bash
go test ./...   # must stay green (NFR-001)
```

## CI verification (release.yml)

After the ldflags change lands:
- A **tag-triggered** build's `version` output (and any emitted report) must show the tag-derived version + commit + date. A build-step assertion greps the binary's `version` output and fails if it still reads `dev`.
- A **manual `workflow_dispatch`** run (non-tag ref) must leave the sentinels (`dev`/`none`/`unknown`) — it must NOT stamp the branch name (C-006). Stamping is gated on `GITHUB_REF_TYPE == 'tag'`.

## Release-time note (FR-006)

When cutting 0.3.0, publish the release body from the curated `release-notes-0.3.0.md` (`gh release edit v0.3.0 --notes-file …`) — it carries the `.version` → `.build.version` breaking-change migration. Do NOT rely on the workflow's auto-generated notes, which would omit the warning.
