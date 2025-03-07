#!/bin/bash

set -e

for file in e2e/*_out.yaml; do
  base="${file%_out.yaml}"
  echo "Testing file: ${base}_in.yaml"
  temp_in=$(mktemp)
  if [[ -f "${base}_defaults.yaml" ]]; then
    ymlt --defaults "${base}_defaults.yaml" "${base}_in.yaml" > "$temp_in"
  else
    ymlt "${base}_in.yaml" > "$temp_in"
  fi
  diff "$temp_in" "$file"
  rm "$temp_in"
done

