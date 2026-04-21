all: build

build:
	go build -o capsule ./cmd/capsule
	go build -o api ./cmd/api

build-release:
	go build -ldflags="-s -w" -o capsule ./cmd/capsule
	upx --best --lzma capsule
	go build -ldflags="-s -w" -o api ./cmd/api
	upx --best --lzma api

clean:
	rm capsule && rm api
