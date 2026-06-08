#!/bin/sh
set -e

UPLOAD_DIR="${UPLOAD_PATH:-/app/uploads}"

mkdir -p "$UPLOAD_DIR"
chown -R appuser:appuser "$UPLOAD_DIR"

exec su-exec appuser "$@"
