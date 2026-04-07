#!/usr/bin/env bash

set -euo pipefail

FILE="UpdateLog.md"

if [[ ! -f "${FILE}" ]]; then
    echo "❌ File does not exist: ${FILE}" >&2
    exit 1
fi

UPDATE_LOGS=$(awk '
  /^## v/ {
    if (found) exit;
    found=1
  }
  found
' "${FILE}")

if [[ -z "${UPDATE_LOGS}" ]]; then
    echo "⚠️ No update logs."
    exit 0
fi

echo "✅ New update logs be like:"
echo "----------------------------------------"
echo "${UPDATE_LOGS}"
echo "----------------------------------------"

if [[ -n "${GITHUB_ENV:-}" ]]; then
  {
    echo "UPDATE_LOGS<<EOF"
    echo "${UPDATE_LOGS}"
    echo "EOF"
  } >> "${GITHUB_ENV}"
  echo "✅ Environment variable UPDATE_LOGS is appended to Github environments."
fi
