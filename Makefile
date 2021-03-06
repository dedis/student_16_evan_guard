install:
	go get -t ./...

test_fmt:
	@echo Checking correct formatting of files
	@{ \
	files=$$( go fmt ./... ); \
	if [ -n "$$files" ]; then \
		echo "Files not properly formatted: $$files"; \
		exit 1; \
	fi; \
	if ! go vet ./...; then \
		exit 1; \
	fi \
	}

test_lint:
	@echo Checking linting of files
	@{ \
	go get github.com/golang/lint/golint; \
	exclude="protocols/byzcoin|_test.go"; \
	lintfiles=$$( golint ./... | egrep -v "($$exclude)" ); \
	if [ -n "$$lintfiles" ]; then \
		echo "Lint errors:"; \
		echo "$$lintfiles"; \
		exit 1; \
	fi \
	}

test_go:
	go test -race -p=1 ./...

test: test_fmt test_lint test_go

all: install test
