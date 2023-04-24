.PHONY: test testx lint bench cover

test:
	go test ./... -race

testx:
	go test ./... -v -failfast

lint:
	golangci-lint run

cover:
	go test -race -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html
