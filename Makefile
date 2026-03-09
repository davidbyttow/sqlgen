.PHONY: test test-v test-short lint build install generate golden clean

# Run all tests.
test:
	go test ./...

# Run all tests with verbose output.
test-v:
	go test ./... -v

# Run tests without the slow generator end-to-end tests.
test-short:
	go test ./... -short

# Run the linter.
lint:
	go vet ./...

# Build the sqlgen binary.
build:
	go build -o bin/sqlgen ./cmd/sqlgen

# Install sqlgen to $GOPATH/bin.
install:
	go install ./cmd/sqlgen

# Regenerate the basic example models.
generate:
	cd examples/basic && go run ../../cmd/sqlgen generate

# Update golden snapshot files.
golden:
	SQLGEN_UPDATE_GOLDEN=1 go test ./gen/ -run TestGoldenFiles -v

# Clear test cache and run all tests.
test-clean:
	go clean -testcache
	go test ./...

# Remove build artifacts.
clean:
	rm -rf bin/
	go clean -testcache
