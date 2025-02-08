build:
	go build -ldflags "-X main.Version=$$(git describe --tags --always)" ./cmd/...

test: build
	go test -v ./...

format:
	go fmt ./...

clean:
	rm -f ymlt
