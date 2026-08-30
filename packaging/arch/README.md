# Arch Linux package

`PKGBUILD` produces the development package `routerctl-git`. It installs the
executable as `/usr/bin/routerctl`, so the command remains `routerctl`.

Build and install it from this directory:

```sh
makepkg -si
```

The source is fetched from the configured GitHub repository. `pkgver()` derives
the package version from the checked-out commit, so updating the source does not
require manually editing `pkgver`.
