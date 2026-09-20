version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# 빌드 결과는 bin/gofer. 재빌드 시 TCC(자동화 권한) 재승인을 막기 위해
# 로컬 자체서명 인증서(gofer-codesign)로 서명한다.
build:
    go build -ldflags "-X main.version={{version}}" -o bin/gofer .
    codesign --force --sign "gofer-codesign" --identifier "local.gofer" bin/gofer

test:
    go test ./...

lint:
    go vet ./...
