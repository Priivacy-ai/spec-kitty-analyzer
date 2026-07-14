# Phase 1 Data Model: Curated CHANGELOG & Release Notes Pipeline

The `tools/release` program is stateless text/CLI tooling; the "data model" is the set of value
types it parses and the invariants over them. No persistent storage, no schemas beyond `CHANGELOG.md`
structure and git tags.

## Entities / value types

### Version
- **Fields**: `Major int`, `Minor int`, `Patch int`, `Stage {alpha|beta|rc|stable}`, `StageNum int`.
- **Parse from**: `X.Y.Z`, `X.Y.Z(a|b|rc)N` (compact only; dotted `-rc.N` is rejected — mirrors the
  reference grammar).
- **Ordering key**: `(Major, Minor, Patch, stageRank, StageNum)` with
  `stageRank = {alpha:0, beta:1, rc:2, stable:3}`.
- **Invariants**: a parsed Version round-trips to its canonical string; `Unreleased` is NOT a Version
  (it is a distinct sentinel heading).

### ChangelogSection
- **Fields**: `Heading (Version | Unreleased)`, `Date string?` (`YYYY-MM-DD`, released only),
  `Body []string` (lines between this heading and the next `## [...]` heading, link-ref lines
  excluded).
- **Derived**: `IsPopulated` = `Body` has ≥1 non-blank line after trimming.
- **Invariants**: sections appear newest-first; exactly one `[Unreleased]` (if present) and it is the
  topmost; the topmost **released** section is "the version being prepared/released".

### ReleaseTag
- **Fields**: `Raw string` (e.g. `v0.2.0`), `Version` (parsed from `Raw` minus leading `v`).
- **Source**: `git tag --list 'v*.*.*'`, filtered to `v` + valid release version.
- **Invariants**: the "latest" tag is `max` by the Version ordering key (tuple sort, not string sort);
  in tag mode the tag under release is excluded from the set before computing "latest".

### ValidationResult
- **Fields**: `OK bool`, `Issues []string` (human-readable, each naming the offending value).
- **Invariants**: `OK == false` ⟺ `len(Issues) > 0`; the program exits non-zero iff `!OK`.

## Relationships

```
CHANGELOG.md ──parsed──▶ [ChangelogSection...] ──top released──▶ Version ─┐
                                                                          ├─▶ ValidationResult
git tags ────parsed──▶ [ReleaseTag...] ──latest (excl. self in tag)──▶ Version ┘
                                                                          │
tag arg / $GITHUB_REF_NAME ──parsed──▶ Version ───(tag-mode parity)───────┘
```

## Validation rules (mapped to FRs)

| Rule | Mode | Source FR |
|------|------|-----------|
| Top released heading parses as a valid Version | branch + tag | FR-004a, FR-005 |
| Top released section `IsPopulated` | branch + tag | FR-004b |
| Branch state-aware monotonicity: `V>T` OK (release-prep) · `V==T` OK (inter-release) · `V<T` error | branch | FR-004 |
| `V` `>` latest tag excluding the released tag (strict) | tag | FR-004d |
| `tag == "v" + topReleasedVersion` (canonical compact) | tag | FR-004c |
| `Unreleased` / link-ref lines never parsed as a released Version | branch + tag | FR-005 |
| Bracketed `## [...]` heading that is neither `Unreleased` nor a valid version → **hard error** | branch + tag | FR-005 |
| Extract returns section body, or default text if absent (exit 0) | (extract) | FR-003 |

Where `V` = top released changelog version, `T` = latest existing `v*.*.*` tag (by the Version
ordering key). Prerelease forms are **compact only** (`rcN`/`aN`/`bN`); dotted `-rc.N` is not a Version.

## State transitions

None. Each invocation is a pure function of `CHANGELOG.md` + git tags + args. No mutable state, no
lifecycle, no persistence.
