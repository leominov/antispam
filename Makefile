.PHONY: run linux

run:
	@go run *.go

linux:
	@mkdir -p bin/
	@export GOOS=linux && export GOARCH=amd64 && go build -o bin/antispam -v
