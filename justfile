version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# 빌드 결과는 bin/gofer
build:
    go build -ldflags "-X main.version={{version}}" -o bin/gofer .

test:
    go test ./...

lint:
    go vet ./...
