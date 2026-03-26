#!/bin/bash

set -euo pipefail

cat <<< '
go 1.26

use (
	./
	./jsonschema/generator
)' > go.work

go run ./jsonschema/generator
rm -f go.work go.work.sum
