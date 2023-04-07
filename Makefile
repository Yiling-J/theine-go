.PHONY: test lint bench

test:
	go test ./... -race

lint:
	golangci-lint run

bench:
	go test -bench=. -run=^$ -benchmem
