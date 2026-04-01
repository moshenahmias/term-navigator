.PHONY: build clean test

build:
	go build -o bin/termnav ./cmd/termnav

clean:
	rm -rf bin/

test:
	go test ./cmd/termnav

run:
	./bin/termnav

VERSION := $(shell git describe --tags --always --dirty)

dist-macos-amd64:
	mkdir -p dist/darwin-amd64/
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/darwin-amd64/termnav ./cmd/termnav
	cd dist && zip -j termnav-darwin-amd64-$(VERSION).zip darwin-amd64/termnav


dist-macos-arm64:
	mkdir -p dist/darwin-arm64/
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/darwin-arm64/termnav ./cmd/termnav
	cd dist && zip -j termnav-darwin-arm64-$(VERSION).zip darwin-arm64/termnav

dist-linux-amd64:
	mkdir -p dist/linux-amd64/
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/linux-amd64/termnav ./cmd/termnav
	cd dist && zip -j termnav-linux-amd64-$(VERSION).zip linux-amd64/termnav

dist-linux-arm64:
	mkdir -p dist/linux-arm64/
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/linux-arm64/termnav ./cmd/termnav
	cd dist && zip -j termnav-linux-arm64-$(VERSION).zip linux-arm64/termnav

dist-linux-arm:
	mkdir -p dist/linux-arm/
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-X main.Version=$(VERSION)" -o dist/linux-arm/termnav ./cmd/termnav
	cd dist && zip -j termnav-linux-arm-$(VERSION).zip linux-arm/termnav

dist-linux-386:
	mkdir -p dist/linux-386/
	GOOS=linux GOARCH=386 go build -ldflags "-X main.Version=$(VERSION)" -o dist/linux-386/termnav ./cmd/termnav
	cd dist && zip -j termnav-linux-386-$(VERSION).zip linux-386/termnav
