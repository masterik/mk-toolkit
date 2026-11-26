# Run `just` for the list, `just <recipe>` to run one.

# build, vet, test, lint, bats, theme — what CI runs, in one shot
ci: build vet test lint bats theme

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run

# the bundled scripts' tests (plugin/skills/_shared/scripts)
bats:
	bats tests/

# the catppuccin theme plugin: validate the mod and run its tests (needs the `claude` CLI)
theme:
	claude plugin validate catppuccin
	claude plugin test catppuccin


# exercise the front-end contract, e.g. `just run version --json`
run *ARGS:
	go run ./cmd/mkit {{ARGS}}

# bump, commit, tag and push a release after a confirmation — auto (default), patch|minor|major or X.Y.Z
release BUMP="auto":
	tools/release.sh {{BUMP}}
