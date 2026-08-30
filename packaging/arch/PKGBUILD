# Maintainer: routerctl contributors

_pkgname=routerctl
pkgname=${_pkgname}-git
pkgver=r10.g92f1088
pkgrel=1
pkgdesc='Host-side orchestrator for router OS manifests, releases, and regulatory evidence'
arch=('x86_64' 'aarch64')
url='https://github.com/kimiusolover/routerctl'
license=('MIT')
depends=('glibc')
makedepends=('git' 'go')
checkdepends=('imagemagick' 'tesseract')
provides=("${_pkgname}=${pkgver}")
conflicts=("${_pkgname}")
source=("git+${url}.git")
b2sums=('SKIP')

pkgver() {
  cd "${srcdir}/${_pkgname}"
  printf 'r%s.g%s' "$(git rev-list --count HEAD)" "$(git rev-parse --short=7 HEAD)"
}

prepare() {
  cd "${srcdir}/${_pkgname}"
  go mod download
}

build() {
  cd "${srcdir}/${_pkgname}"
  local build_date
  build_date=$(date -u -d "@${SOURCE_DATE_EPOCH:-$(git log -1 --format=%ct)}" +%Y-%m-%dT%H:%M:%SZ)
  go build -trimpath -buildmode=pie -mod=readonly \
    -ldflags="-X main.version=${pkgver} -X main.commit=$(git rev-parse --short=12 HEAD) -X main.date=${build_date}" \
    -o routerctl ./cmd/routerctl
}

check() {
  cd "${srcdir}/${_pkgname}"
  go test ./...
}

package() {
  cd "${srcdir}/${_pkgname}"
  install -Dm755 routerctl "${pkgdir}/usr/bin/routerctl"
  install -Dm644 LICENSE "${pkgdir}/usr/share/licenses/${_pkgname}/LICENSE"
  install -Dm644 README.md "${pkgdir}/usr/share/doc/${_pkgname}/README.md"
}
