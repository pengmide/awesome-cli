#!/usr/bin/env bash

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly ROWCOUNT_BIN="${SCRIPT_DIR}/dist/parquet-rowcount"
readonly DATE_DIR_PREFIX="inc_day="
readonly BATCH_SIZE=200

usage() {
  cat <<'EOF'
Usage:
  count_event_rows.sh <root_dir>
  count_event_rows.sh <root_dir> <start_yyyymmdd> <end_yyyymmdd>

Output columns (TSV):
  day	parquet_files	row_count
EOF
}

is_gnu_date() {
  date --version >/dev/null 2>&1
}

normalize_yyyymmdd() {
  local input="$1"

  if [[ ! "$input" =~ ^[0-9]{8}$ ]]; then
    return 1
  fi

  if is_gnu_date; then
    date -d "${input:0:4}-${input:4:2}-${input:6:2}" "+%Y-%m-%d" 2>/dev/null
  else
    date -j -f "%Y%m%d" "$input" "+%Y-%m-%d" 2>/dev/null
  fi
}

next_day() {
  local input="$1"

  if is_gnu_date; then
    date -d "$input +1 day" "+%Y-%m-%d"
  else
    date -j -v+1d -f "%Y-%m-%d" "$input" "+%Y-%m-%d"
  fi
}

extract_total_rows() {
  local output="$1"
  local total

  total="$(printf '%s\n' "$output" | awk -F '\t' '$2 == "total" { print $1; exit }')"
  if [[ -z "$total" || ! "$total" =~ ^[0-9]+$ ]]; then
    echo "failed to parse row count output: $output" >&2
    exit 1
  fi

  printf '%s\n' "$total"
}

run_batch() {
  local batch_output
  local batch_rows

  if [[ $# -eq 0 ]]; then
    printf '0\n'
    return
  fi

  batch_output="$("$ROWCOUNT_BIN" -total-only "$@")"
  batch_rows="$(extract_total_rows "$batch_output")"

  printf '%s\n' "$batch_rows"
}

collect_day_stats() {
  local day_dir="$1"
  local file_count=0
  local row_count=0
  local -a batch=()
  local batch_rows
  local file_path

  if [[ ! -d "$day_dir" ]]; then
    printf '0\t0\n'
    return
  fi

  while IFS= read -r -d '' file_path; do
    file_count=$((file_count + 1))
    batch+=("$file_path")

    if [[ ${#batch[@]} -ge $BATCH_SIZE ]]; then
      batch_rows="$(run_batch "${batch[@]}")"
      row_count=$((row_count + batch_rows))
      batch=()
    fi
  done < <(find "$day_dir" -type f -name '*.parquet' -print0)

  batch_rows="$(run_batch "${batch[@]}")"
  row_count=$((row_count + batch_rows))

  printf '%s\t%s\n' "$file_count" "$row_count"
}

print_day_line() {
  local day="$1"
  local stats="$2"
  printf '%s\t%s\n' "$day" "$stats"
}

main() {
  local root_dir
  local start_date=""
  local end_date=""
  local current_date=""
  local day_dir=""
  local stats=""
  local file_count=0
  local row_count=0
  local total_files=0
  local total_rows=0
  local matched=0
  local entry=""
  local day=""

  if [[ $# -ne 1 && $# -ne 3 ]]; then
    usage >&2
    exit 1
  fi

  root_dir="$1"

  if [[ ! -d "$root_dir" ]]; then
    echo "root directory does not exist: $root_dir" >&2
    exit 1
  fi

  if [[ ! -x "$ROWCOUNT_BIN" ]]; then
    echo "row count binary is missing or not executable: $ROWCOUNT_BIN" >&2
    exit 1
  fi

  printf 'day\tparquet_files\trow_count\n'

  if [[ $# -eq 3 ]]; then
    start_date="$(normalize_yyyymmdd "$2")" || {
      echo "invalid start date: $2" >&2
      exit 1
    }
    end_date="$(normalize_yyyymmdd "$3")" || {
      echo "invalid end date: $3" >&2
      exit 1
    }

    if [[ "$start_date" > "$end_date" ]]; then
      echo "start date must be earlier than or equal to end date" >&2
      exit 1
    fi

    current_date="$start_date"
    while [[ "$current_date" < "$end_date" || "$current_date" == "$end_date" ]]; do
      day_dir="$root_dir/${DATE_DIR_PREFIX}${current_date}"
      stats="$(collect_day_stats "$day_dir")"
      file_count="${stats%%$'\t'*}"
      row_count="${stats#*$'\t'}"

      print_day_line "$current_date" "$stats"

      total_files=$((total_files + file_count))
      total_rows=$((total_rows + row_count))
      current_date="$(next_day "$current_date")"
    done
  else
    while IFS= read -r entry; do
      matched=1
      day="${entry#${DATE_DIR_PREFIX}}"
      stats="$(collect_day_stats "$root_dir/$entry")"
      file_count="${stats%%$'\t'*}"
      row_count="${stats#*$'\t'}"

      print_day_line "$day" "$stats"

      total_files=$((total_files + file_count))
      total_rows=$((total_rows + row_count))
    done < <(
      find "$root_dir" -mindepth 1 -maxdepth 1 -type d -name "${DATE_DIR_PREFIX}????-??-??" -exec basename {} \; | sort
    )

    if [[ $matched -eq 0 ]]; then
      :
    fi
  fi

  printf 'TOTAL\t%s\t%s\n' "$total_files" "$total_rows"
}

main "$@"
