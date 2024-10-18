.PHONY: test test-race-pool test-race-nopool testx lint bench cover

test:
	go test -skip=TestCacheRace_ ./...

test-race-pool:
	go test ./... -run=TestCacheRace_EntryPool -count=1

test-race-nopool:
	go test ./... -run=TestCacheRace_NoPool -count=1 -race

testx:
	go test ./... -v -failfast

lint:
	golangci-lint run

cover:
	go test -timeout 2000s -race -coverprofile=cover.out -coverpkg=./... -skip=TestCacheRace_ ./...
	go tool cover -html=cover.out -o cover.html
