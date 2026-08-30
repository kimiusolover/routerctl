# Contributing

This is a Human-AI collaborative project. Read [the AI contribution policy](docs/development/ai-contribution-policy.md) and [the Git trailer policy](docs/development/git-trailer-policy.md) before submitting changes.

Never bypass hardware, regulatory evidence, or release review gates. In particular, an AX23V discovery fixture is not flashable and must not be applied to a device.

Run the relevant tests and `routerctl verify commit` before requesting review.
GitHub Actions runs the same check over every pull-request commit under the
`commit-trailer-policy` status name; configure that status as required in the
protected-branch rules.
