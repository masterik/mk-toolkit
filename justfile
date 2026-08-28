# Run `just` for the list, `just <recipe>` to run one.

# build, vet, test, lint, shell suite — what CI runs, in one shot
ci: build vet test lint shtest

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run

# node --test + bats over the shell layer (tests/run.sh)
shtest:
	tests/run.sh

# exercise the front-end contract, e.g. `just run version --json`
run *ARGS:
	go run ./cmd/mkit {{ARGS}}
