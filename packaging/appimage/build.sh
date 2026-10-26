#!/bin/sh
# Builds the AppDir inside a debian:bookworm container (see `make appimage`):
# an old enough glibc (2.36) that the AppImage runs on most distros. The
# audio and keyboard libraries are bundled; glibc, the graphics stack
# (EGL, Wayland) and ALSA come from the system, as AppImages normally do.
set -eu
apt-get update -qq
apt-get install -y -qq --no-install-recommends gcc libc6-dev pkg-config make \
	libwayland-dev libxkbcommon-dev libegl-dev libgles-dev \
	libasound2-dev libogg-dev libvorbis-dev libflac-dev libmpg123-dev >/dev/null

cd /src
export PATH=$GOROOT/bin:$PATH GOMODCACHE=/gomod GOPROXY=off GOTOOLCHAIN=local GOCACHE=/tmp/gocache
app=/out/AppDir
rm -rf "$app"
mkdir -p "$app/usr/bin" "$app/usr/lib"
# The bundled libraries are found next to the binary through its runpath.
go build -tags nox11,novulkan -trimpath \
	-ldflags '-s -w -X gioui.org/app.ID=spotimy -extldflags "-Wl,-rpath,$ORIGIN/../lib"' \
	-o "$app/usr/bin/spotimy" .
for lib in libxkbcommon.so.0 libmpg123.so.0 libFLAC.so.12 libvorbisenc.so.2 libvorbis.so.0 libogg.so.0; do
	cp -L "/usr/lib/x86_64-linux-gnu/$lib" "$app/usr/lib/"
done
cp packaging/spotimy.desktop packaging/spotimy.svg "$app/"
ln -s spotimy.svg "$app/.DirIcon"
ln -s usr/bin/spotimy "$app/AppRun"
chown -R "$OWNER" /out/AppDir
