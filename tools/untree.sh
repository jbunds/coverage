#!/usr/bin/env bash

# useful for validating doc/structure.md, and dirty workspace checks:
#
#   diff <(./untree.sh) <(git ls-tree -r --name-only HEAD)
#   diff <(./untree.sh) <(./untree.sh | xargs git ls-files)

shopt -s lastpipe # enable lastpipe so the `lines` array persists outside the pipeline
shopt -s extglob  # enable extended globbing for trimming whitespace

grep -E '^[│├└]' doc/structure.md | sed 's/\xc2\xa0/ /g' | mapfile -t lines

paths=()

for line in "${lines[@]}"; do
  name=$(echo "$line" | sed -E 's/^([│[:space:]]*├──|[│[:space:]]*└──|[│[:space:]]+)//')
  name="${name##+([[:space:]])}"
  name="${name%%+([[:space:]])}"

  prefix="${line%%[├──└──]*}"
  depth=$((${#prefix} / 4))

  paths=("${paths[@]:0:$depth}")
  paths+=("$name")

  full_path=$(IFS=/; echo "${paths[*]}")

  if [[ -f "$full_path" ]]; then
    echo "$full_path"
  fi
done
