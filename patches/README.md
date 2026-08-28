# Patch series

Create one directory per profile, for example `patches/ax23v/`. Patch files are
applied in lexical order to an isolated worktree by `packaging/build.sh`.

Do not apply this series directly in `upstream/firmware`; that directory is a
clean, pinned submodule checkout.
