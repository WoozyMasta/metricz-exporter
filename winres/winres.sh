#!/usr/bin/env bash
set -euo pipefail

# Args:
#   $1 - binary name (without .exe).
#   $2 - base winres.json  (default: winres/winres.json)
#   $3 - output json       (default: winres/winres.current.json)

: "${BIN_NAME:=${1:?}}"
: "${WINRES_BASE_FILE:=${2:-./winres/winres.json}}"
: "${WINRES_UPDATED_FILE:=${3:-./winres/winres.current.json}}"

tag="$(git describe --tags --abbrev=0 2>/dev/null || echo 0.0.0)"
tag="${tag#v}"
tag="${tag%%[-+]*}"
IFS='.' read -r -a parts <<< "$tag"
v_major="${parts[0]:-0}"
v_minor="${parts[1]:-0}"
v_patch="${parts[2]:-0}"

# use commit count (<=65535)
build_num="$(git rev-list --count HEAD 2>/dev/null || echo 0)"
[ "$build_num" -gt 65535 ] && build_num=$((build_num % 65535))

v_windows="$v_major.$v_minor.$v_patch.$build_num"
commit_short="$(git rev-parse --short HEAD 2>/dev/null || echo 0000000)"
v_string="$v_major.$v_minor.$v_patch+$commit_short"

jq -er \
  --arg tag "$tag" \
  --arg v_windows   "$v_windows" \
  --arg v_string    "$v_string" \
  --arg bin_name    "$BIN_NAME.exe" '
  .RT_MANIFEST["#1"]["0409"].identity.version = $tag
  | .RT_VERSION["#1"]["0000"].fixed.file_version    = $v_windows
  | .RT_VERSION["#1"]["0000"].fixed.product_version = $v_windows
  | .RT_VERSION["#1"]["0000"].info["0409"].FileVersion    = $v_string
  | .RT_VERSION["#1"]["0000"].info["0409"].ProductVersion = $v_string
  | .RT_VERSION["#1"]["0000"].info["0409"].OriginalFilename = $bin_name
' "$WINRES_BASE_FILE" > "$WINRES_UPDATED_FILE"
