all: build

build:
	go build -o capsule main.go

build-release:
	go build -ldflags="-s -w" -o capsule
	upx --best --lzma capsule

clean:
	rm capsule

