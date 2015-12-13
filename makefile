srcFiles := $(shell find . -name "*.go" ! -name "*_test.go")

all: gitgone gitgone-tests
build:gitgone
gitgone:$(srcFiles)
	go build main/$@.go

test:
	go test -v --cover ./...
