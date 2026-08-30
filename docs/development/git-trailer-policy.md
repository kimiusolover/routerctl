# Git trailer policy

The project uses these four provenance trailers with the exact values below:

```text
AI-Assisted-By: OpenAI ChatGPT
Generated-By: routerctl sync
Reviewed-By: <human>
Automation-Actor: router-os-bot[bot]
```

`AI-Assisted-By` records material OpenAI ChatGPT / Codex assistance. `Generated-By` records that `routerctl sync` generated the commit. `Reviewed-By` records an actual named human review. `Automation-Actor` records the actor only when `router-os-bot[bot]` performed the GitHub operation.

Ordinary generation does not require `Reviewed-By`. A verified promotion, regulatory value change, device application, or public release does. The commit verifier classifies the repository's regulatory/profile/device/release paths as review-required and fails if their commit lacks `Reviewed-By`.

Use the local checks:

```sh
routerctl verify commit
routerctl git sync --ai-assisted --reviewed-by "Yuta Nakano" --automation-actor 'router-os-bot[bot]'
```

`Generated-By: routerctl sync` without `Automation-Actor` is a warning, not a failure: local interactive sync is allowed, but an automated GitHub operation must identify its bot actor. The verifier rejects duplicate policy trailers, wrong fixed values, and non-human values for `Reviewed-By`.

GitHub Actions runs `routerctl verify commit` for every commit in a pull request
as the `commit-trailer-policy` status check. Protect the target branch by
requiring that check; direct writes and releases remain separately governed by
repository permissions.
