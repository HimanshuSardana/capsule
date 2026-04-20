all: build

build:
	go build -o capsule ./cmd/capsule

build-release:
	go build -ldflags="-s -w" -o capsule ./cmd/capsule
	upx --best --lzma capsule

clean:
	rm capsule

