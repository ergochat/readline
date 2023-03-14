.PHONY: all test examples ci gofmt

all: ci

ci: test examples

test:
	go test -race ./...
	go vet ./...

examples:
	go build -o /dev/null example/readline-demo/readline-demo.go
	GOOS=windows go build -o /dev/null example/readline-demo/readline-demo.go

gofmt:
	./.check-gofmt.sh --fix
