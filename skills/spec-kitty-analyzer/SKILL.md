# Spec Kitty Analyzer

Use this skill when diagnosing a completed or troubled Spec Kitty mission from
agent harness logs. The best time to run it is after the Spec Kitty mission
retrospective, when the mission has enough history to explain decisions,
failures, retries, merges, worktree confusion, and review outcomes.

## Core Rule

Prefer structured analyzer JSON over reading raw harness logs. Use positive
Spec Kitty identifiers first: `/spec-kitty.*` slash commands, `spec-kitty ...`
CLI invocations, `spk-*` / `spec-kitty-*` skills, agent profiles, mission
scopes, Op scopes, and deterministic failure fingerprints.

## Commands

List known missions from the local cache:

```bash
spec-kitty-analyzer missions --limit 20
```

Query one mission for agent-readable JSON:

```bash
spec-kitty-analyzer query <mission-slug> --include timeline,signals,findings
```

Focus a query on known failure modes:

```bash
spec-kitty-analyzer query <mission-slug> \
  --include timeline,signals,findings \
  --failure-id branch_worktree_confusion
```

Focus a query on merge-related Spec Kitty activity:

```bash
spec-kitty-analyzer query <mission-slug> \
  --include timeline,signals,findings \
  --command merge
```

Create full human reports:

```bash
spec-kitty-analyzer analyze <mission-slug> --out spec-kitty-analyzer-report.json
```

Force a full cache rescan only when logs may have changed or the mission is
missing:

```bash
spec-kitty-analyzer query <mission-slug> --cache-bust
```

## Interpretation

- `mission:<slug>` events are inside the mission story.
- `op:<invocation_id>` events are Spec Kitty runtime operation records.
- `outside` events are surrounding harness context; treat them as lower
  confidence unless they carry a Spec Kitty command, skill, profile, or
  deterministic failure fingerprint.
- Timeline JSON is already filtered to Spec Kitty positive signals.
- Branch/worktree confusion and merge failures are included through explicit
  failure fingerprints, not broad text matching.

## Output To Use

For automation and follow-up agents, consume `query` JSON. It includes:

- `timeline`: filtered Spec Kitty events only.
- `signals`: extracted slash commands, CLI invocations, skills, profiles, and
  failures with event refs.
- `findings`: aggregated deterministic failure modes with recovery guidance.
- `cache`: cache status and mission log files used.

Use Markdown, HTML, or PDF reports only for human review.
