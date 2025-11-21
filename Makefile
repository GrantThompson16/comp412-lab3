APP      := 412alloc
PKG_MAIN := ./cmd/412alloc

.PHONY: all build clean

all: build

build:
	GOFLAGS= GOTOOLCHAIN=local go build -trimpath -ldflags="-s -w" -o $(APP) $(PKG_MAIN)
	chmod +x $(APP)

clean:
	rm -f $(APP)
