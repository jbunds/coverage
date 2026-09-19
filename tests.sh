#!/usr/bin/env bash

# list Go functions and possible corresponding (1:*) tests in a two-column table

shopt -s extglob

funcs=()
tests_list=()

func_len=0
tests_len=0

for func in $(sed -nE 's/^[[:space:]]*func[[:space:]]+(\([^)]*\)[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*(.*)/\2/p' !(*_test).go); do
  matching_tests=$(grep -hE "^\s*func\s+Test${func^}" *_test.go \
    | sed -nE 's/^[[:space:]]*func[[:space:]]+([A-Za-z_][A-Za-z0-9_]*).*/\1/p')
  funcs+=("$func")
  tests_list+=("$matching_tests")
	if (( ${#func} > func_len )); then
    func_len=${#func}
  fi
  while IFS= read -r t; do
    if (( ${#t} > tests_len )); then
      tests_len=${#t}
    fi
  done <<< "$matching_tests"
done

divider=$(          printf '%*s' $((func_len  + 1)) '' | tr ' ' '─')
divider=${divider}┼
divider=${divider}$(printf '%*s' $((tests_len + 1)) '' | tr ' ' '─')
divider=$(          printf '%s\n' $divider)

printf '%-*s │ %s\n' $func_len "func" "test(s)"
echo $divider

for i in "${!funcs[@]}"; do
  func="${funcs[$i]}"
  tests="${tests_list[$i]}"

  if [[ -z $tests ]]; then
    printf '%-*s │\n' $func_len $func
    if (( $i < ${#funcs[@]} - 1 )); then
      echo $divider
    fi
    continue
  fi

  first=$(head -1 <<< "$tests")
  printf '%-*s │ %s\n' $func_len $func $first

  rest=$(tail -n +2 <<< "$tests")
  if [[ -n $rest ]]; then
    while IFS= read -r t; do
      printf '%*s │ %s\n' $func_len '' $t
    done <<< "$rest"
  fi
  if (( $i < ${#funcs[@]} - 1 )); then
    echo $divider
  fi
done
