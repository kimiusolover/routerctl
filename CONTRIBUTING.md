# Contributing

This is a Human-AI collaborative project. Read [the AI contribution policy](docs/development/ai-contribution-policy.md) and [the Git trailer policy](docs/development/git-trailer-policy.md) before submitting changes.

Never bypass hardware, regulatory evidence, or release review gates. In particular, an AX23V discovery fixture is not flashable and must not be applied to a device.

For usage questions and proposals, use GitHub Discussions according to the
[Discussions policy](docs/community/github-discussions-policy.ja.md). A
Discussion is not authorization for flashing, RF transmission, safety decisions,
or releases; use Issues and pull requests for changes that need formal tracking.

Run the relevant tests and `routerctl verify commit` before requesting review.
GitHub Actions runs the same check over every pull-request commit under the
`commit-trailer-policy` status name; configure that status as required in the
protected-branch rules.
