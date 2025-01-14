.PHONY: build build-linux build-mac build-all clean install

# build for current plataform (development)
build:
	go build -o core-rss ./cmd/core-rss

# build for specific platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o dist/core-rss-linux-amd64 ./cmd/core-rss
	GOOS=linux GOARCH=arm64 go build -o dist/core-rss-linux-arm64 ./cmd/core-rss

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o dist/core-rss-darwin-amd64 ./cmd/core-rss
	GOOS=darwin GOARCH=arm64 go build -o dist/core-rss-darwin-arm64 ./cmd/core-rss

# build for all platforms
build-all: build-linux build-mac

# install locally (development)
install: build
	sudo mv core-rss /usr/local/bin/
	sudo chmod +x /usr/local/bin/core-rss

# clean build artifacts
clean:
	rm -f core-rss
	rm -rf dist/

# create dist directory
dist:
	mkdir -p dist
