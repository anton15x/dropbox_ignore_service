#!/bin/sh

set -e
set -x

go mod download

go test ./...

npm install
npm run spellcheck

golangci-lint run ./...

# skip: lint would error if windows line endings are used
# npm run lint

echo "local test ok"
