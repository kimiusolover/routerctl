# Git trailer policy

The project uses these provenance trailers:

```text
AI-Assisted-By: <AI system actually used>
Generated-By: routerctl sync
Reviewed-By: <human>
Automation-Actor: router-os-bot[bot]
```

## Meaning of each trailer

| Trailer | Meaning |
|---------|---------|
| `AI-Assisted-By` | Development provenance: an AI system materially assisted implementation, design, docs, or analysis. Not authorship and not a human review. |
| `Generated-By` | The commit message or change set was produced by `routerctl sync`. Fixed value only. |
| `Reviewed-By` | A named human performed review. Required for verified promotion, regulatory values, device application, or public release paths. |
| `Automation-Actor` | GitHub automation that performed the operation. Fixed value `router-os-bot[bot]` only, and only when that bot acted. |

## AI-Assisted-By

If AI systems materially assisted a change, record one or more `AI-Assisted-By` trailers naming the systems actually used. There is **no vendor whitelist**.

Examples:

```text
AI-Assisted-By: Grok
AI-Assisted-By: OpenAI ChatGPT
AI-Assisted-By: Claude Code
AI-Assisted-By: Gemini
```

Multiple systems:

```text
AI-Assisted-By: Grok
AI-Assisted-By: OpenAI ChatGPT
```

Rules:

- Optional: ordinary commits need not include it.
- Do not claim assistance from a system that was not used.
- Value must be non-empty, single-line, and at most 128 characters.
- Do not use `router-os-bot[bot]` as an AI system name.

`Generated-By`, `Reviewed-By`, and `Automation-Actor` still use fixed or human-only values as above.

## Repository Profiles

Each repository runs `routerctl verify commit --profile <profile>` in its `commit-trailer-policy` status check:

- `routerctl` -> `--profile policy-source`
- `router-firmware` -> `--profile firmware`
- `router-platform` -> `--profile platform`
- `router-infra` -> `--profile infrastructure`
- `router-upstream` -> `--profile upstream`
- `router-packages` -> `--profile package`
- `certificateDB` -> `--profile regulatory`

## Local checks

```sh
routerctl verify commit --profile policy-source
routerctl git sync --ai-assisted-by Grok --reviewed-by "Yuta Nakano" --automation-actor 'router-os-bot[bot]'
```

`Generated-By: routerctl sync` without `Automation-Actor` is a warning, not a failure: local interactive sync is allowed, but an automated GitHub operation must identify its bot actor. The verifier rejects invalid trailer shapes, wrong fixed values for `Generated-By` / `Automation-Actor`, and non-human values for `Reviewed-By`.

GitHub Actions runs `routerctl verify commit --profile <profile>` for every commit in a pull request
as the `commit-trailer-policy` status check. Protect the target branch by
requiring that check; direct writes and releases remain separately governed by
repository permissions.
