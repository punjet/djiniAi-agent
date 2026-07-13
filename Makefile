.PHONY: build test lint run-evaluate run-test-apply

# Default target
all: build

# Build the career-ops binary
build:
	go build -v -o career-ops ./cmd/career-ops

# Run all tests
test:
	go test -v ./...

# Run standard Go vet as the linter
lint:
	go vet ./...

# Run the evaluate command with a custom JD (e.g. make run-evaluate JD="text")
run-evaluate:
	@if [ -z "$(JD)" ]; then \
		echo "Error: JD is required. Usage: make run-evaluate JD=\"text\""; \
		exit 1; \
	fi
	go run ./cmd/career-ops evaluate --jd "$(JD)"

# Run the test-apply pipeline (diagnostic/test tool)
run-test-apply:
	go run ./cmd/career-ops pipeline test-apply