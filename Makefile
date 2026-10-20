# Gio needs these tags here: Wayland only (no xkbcommon-x11) and GL only (no
# Vulkan headers).
TAGS := nox11,novulkan
GOFLAGS_BUILD := -tags $(TAGS) -trimpath -ldflags "-s -w -X gioui.org/app.ID=personal-spotify"

build:
	go build $(GOFLAGS_BUILD) -o build/bin/personal-spotify .

run:
	go run -tags $(TAGS) .

test:
	go test -tags $(TAGS) ./...

vet:
	go vet -tags $(TAGS) ./...

.PHONY: build run test vet
