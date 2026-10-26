#
# spec file for personal-spotify
#
Name:           personal-spotify
Version:        %{version}
Release:        1
Summary:        Lightweight native Spotify client
License:        GPL-3.0-only
URL:            https://github.com/GGGeorgiev20/personal-spotify
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  go >= 1.26
BuildRequires:  make
BuildRequires:  pkgconfig(wayland-client)
BuildRequires:  pkgconfig(wayland-egl)
BuildRequires:  pkgconfig(wayland-cursor)
BuildRequires:  pkgconfig(xkbcommon)
BuildRequires:  pkgconfig(egl)
BuildRequires:  pkgconfig(alsa)
BuildRequires:  pkgconfig(ogg)
BuildRequires:  pkgconfig(vorbis)
BuildRequires:  pkgconfig(flac)
BuildRequires:  pkgconfig(libmpg123)
Recommends:     ffmpeg
Recommends:     noto-coloremoji-fonts
# The Go binary carries no build ids worth a debuginfo package.
%global debug_package %{nil}

%description
A lightweight Spotify desktop client for Linux, drawn natively with Gio, with
go-librespot embedded for playback and Spotify Connect. Spotify Premium is
required.

%prep
%autosetup

%build
make build

%install
make install DESTDIR=%{buildroot} PREFIX=%{_prefix}

%files
%license LICENSE
%doc README.md
%{_bindir}/personal-spotify
%{_datadir}/applications/personal-spotify.desktop
%{_datadir}/icons/hicolor/scalable/apps/personal-spotify.svg

%changelog
