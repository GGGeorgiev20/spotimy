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

# appimage builds the AppImage into build/dist: the AppDir in a Debian 12
# container (this machine's Go and module cache mounted in), then
# appimagetool (APPIMAGETOOL, the AppImage project's tool) packs it with
# the AppImage runtime (APPIMAGE_RUNTIME, else it downloads one).
APPIMAGETOOL ?= appimagetool
# openSUSE's Go keeps its sources under /usr/share/go, linked from GOROOT.
GOROOT := $(shell go env GOROOT)
appimage:
	mkdir -p build/dist
	docker run --rm -e OWNER=$(shell id -u):$(shell id -g) -v $(CURDIR):/src:ro -v $(CURDIR)/build/dist:/out \
		-v $(GOROOT):$(GOROOT):ro -v /usr/share/go:/usr/share/go:ro -e GOROOT=$(GOROOT) \
		-v $(shell go env GOMODCACHE):/gomod:ro debian:bookworm sh /src/packaging/appimage/build.sh
	ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 $(APPIMAGETOOL) --no-appstream $(if $(APPIMAGE_RUNTIME),--runtime-file $(APPIMAGE_RUNTIME)) build/dist/AppDir build/dist/personal-spotify-$(VERSION)-x86_64.AppImage

.PHONY: build run test vet install dist rpm arch appimage
