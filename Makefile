.PHONY: test test-race testx lint bench cover

test:
	go test -skip=TestCacheRace_ ./...

test-race:
	go test ./... -run=TestCacheRace_ -count=1 -race

testx:
	go test ./... -v -failfast

lint:
	golangci-lint run

cover:
	go test -timeout 2000s -coverprofile=cover.out -coverpkg=./... -skip=TestCacheRace_ ./...
	go tool cover -html=cover.out -o cover.html
