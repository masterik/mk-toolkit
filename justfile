# Run `just` for the list, `just <recipe>` to run one.

# build, vet, test, lint — what CI runs, in one shot
ci: build vet test lint

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run


# exercise the front-end contract, e.g. `just run version --json`
run *ARGS:
	go run ./cmd/mkit {{ARGS}}

# bump, commit, tag and push a release after a confirmation — auto (default), patch|minor|major or X.Y.Z
release BUMP="auto":
	tools/release.sh {{BUMP}}
