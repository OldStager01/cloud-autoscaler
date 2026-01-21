#!/bin/bash

set -e

echo "Running database migrations..."
go run cmd/autoscaler/main.go --migrate

echo "Done."