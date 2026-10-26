# Gio needs these tags here: Wayland only (no xkbcommon-x11) and GL only (no
# Vulkan headers).
TAGS := nox11,novulkan
GOFLAGS_BUILD := -tags $(TAGS) -trimpath -ldflags "-s -w -X gioui.org/app.ID=personal-spotify"

VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//')
PREFIX ?= /usr/local
DESTDIR ?=

build:
	go build $(GOFLAGS_BUILD) -o build/bin/personal-spotify .

# install puts the binary, the desktop entry (which is also how the window
# finds its icon: Gio's app id matches its name) and the icon under PREFIX.
install:
	install -Dm755 build/bin/personal-spotify $(DESTDIR)$(PREFIX)/bin/personal-spotify
	install -Dm644 packaging/personal-spotify.desktop $(DESTDIR)$(PREFIX)/share/applications/personal-spotify.desktop
	install -Dm644 packaging/personal-spotify.svg $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/personal-spotify.svg

# dist is the source tarball the packages build from.
dist:
	mkdir -p build/dist
	git archive --format=tar.gz --prefix=personal-spotify-$(VERSION)/ -o build/dist/personal-spotify-$(VERSION).tar.gz HEAD

# rpm builds the openSUSE package into build/dist.
rpm: dist
	rpmbuild -bb packaging/opensuse/personal-spotify.spec \
		--define "_sourcedir $(CURDIR)/build/dist" --define "_rpmdir $(CURDIR)/build/dist" \
		--define "_builddir $(CURDIR)/build/rpm" --define "version $(VERSION)"

# arch builds the Arch package into build/dist with makepkg in an Arch
# container, using this machine's Go module cache (no module downloads).
arch: dist
	cp packaging/arch/PKGBUILD build/dist/PKGBUILD
	docker run --rm -v $(CURDIR)/build/dist:/pkg -v $(shell go env GOMODCACHE):/gomod:ro archlinux:latest bash -euc '\
		pacman -Syu --noconfirm --needed base-devel go wayland libxkbcommon libglvnd alsa-lib libogg libvorbis flac mpg123 >/dev/null; \
		useradd -m build; cp -r /pkg /home/build/pkg; chown -R build /home/build/pkg; \
		cd /home/build/pkg && sed -i "s/^pkgver=.*/pkgver=$(VERSION)/" PKGBUILD && \
		su build -c "GOMODCACHE=/gomod GOPROXY=off makepkg --nodeps --noconfirm" && \
		cp *.pkg.tar.zst /pkg/'

run:
	go run -tags $(TAGS) .

test:
	go test -tags $(TAGS) ./...

vet:
	go vet -tags $(TAGS) ./...

.PHONY: build run test vet install dist rpm arch
