.PHONY: test lint bench cover

test:
	go test ./... -race

lint:
	golangci-lint run

bench:
	go test -bench=. -run=^$ -benchmem

cover:
	go test -race -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html
