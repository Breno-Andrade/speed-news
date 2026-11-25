#!/usr/bin/env bash
# Memory used by a running Speed News, per process group (Linux).
#
#   ./scripts/measure-memory.sh            # installed app (/usr/bin/speed-news) or `make dev`
#   ./scripts/measure-memory.sh <pid>      # a specific shell process
#
# PSS (proportional set size) splits shared pages between the processes using them, so the
# groups add up to what the app really costs. Measure in three moments to compare builds:
# popover open, 2 minutes after closing it (webview destroyed), and after a day of uptime.
set -euo pipefail

shell_pid="${1:-$(pgrep -o -x speed-news || true)}"
if [[ -z "${shell_pid}" ]]; then
  echo "Speed News is not running" >&2
  exit 1
fi

# Every descendant of the shell: WebKit processes and the Go sidecar.
descendant_pids() {
  local parent_pid="$1"
  local child_pid
  for child_pid in $(pgrep -P "${parent_pid}" || true); do
    echo "${child_pid}"
    descendant_pids "${child_pid}"
  done
}

pss_kib() {
  awk '/^Pss:/ { print $2 }' "/proc/$1/smaps_rollup" 2>/dev/null || echo 0
}

declare -A group_kib=([shell]=0 [webview]=0 [sidecar]=0 [other]=0)
group_kib[shell]=$(pss_kib "${shell_pid}")
for process_pid in $(descendant_pids "${shell_pid}"); do
  process_name="$(cat "/proc/${process_pid}/comm" 2>/dev/null || echo gone)"
  case "${process_name}" in
    WebKit*) group_name=webview ;;
    news-api*) group_name=sidecar ;;
    *) group_name=other ;;
  esac
  group_kib[${group_name}]=$(( group_kib[${group_name}] + $(pss_kib "${process_pid}") ))
done

total_kib=0
printf '%-26s %10s\n' "group" "PSS (MiB)"
for group_name in shell webview sidecar other; do
  total_kib=$(( total_kib + group_kib[${group_name}] ))
  case "${group_name}" in
    shell) label="shell (Rust/Tauri)" ;;
    webview) label="webview (WebKitGTK)" ;;
    sidecar) label="sidecar (Go news-api)" ;;
    other) label="other children" ;;
  esac
  awk -v label="${label}" -v kib="${group_kib[${group_name}]}" 'BEGIN { printf "%-26s %10.1f\n", label, kib / 1024 }'
done
awk -v kib="${total_kib}" 'BEGIN { printf "%-26s %10.1f\n", "total", kib / 1024 }'
