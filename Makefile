.PHONY: all test examples ci gofmt wasm

all: ci

ci: test examples

test:
	go test -race ./...
	go vet ./...
	./.check-gofmt.sh

examples:
	GOOS=linux go build -o /dev/null example/readline-demo/readline-demo.go
	GOOS=windows go build -o /dev/null example/readline-demo/readline-demo.go
	GOOS=darwin go build -o /dev/null example/readline-demo/readline-demo.go

gofmt:
	./.check-gofmt.sh --fix

# wasm builds for WebAssembly, and runs TestWasm in a headless Chrome, with
# wasmbrowsertest (go install github.com/agnivade/wasmbrowsertest@latest)
wasm:
	GOOS=js GOARCH=wasm go vet .
	GOOS=js GOARCH=wasm go build -o /dev/null example/readline-demo/readline-demo.go
	GOOS=js GOARCH=wasm go test -exec wasmbrowsertest -run TestWasm .
