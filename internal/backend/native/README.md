# Native backend

The native backend runs an explicitly configured local build command. The
command is an executable plus arguments, never a shell expression. It receives
the selected profile as its final argument and these environment variables:

- `ROUTERCTL_DEVICE`
- `ROUTERCTL_PROFILE`
- `ROUTERCTL_OUTPUT_DIR`

The output directory must contain one or more regular artifact files when the
command exits successfully. `SHA256SUMS` is treated as metadata and is not
returned as an artifact; every returned artifact is hashed by `routerctl`.
