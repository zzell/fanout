lint:
	@golangci-lint run

integration:
	@go test -tags integration -covermode=count

test:
	@go test -covermode=count -count=1 $$(go list -e ./... | grep -v .*/mock)
