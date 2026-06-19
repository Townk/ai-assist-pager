#!/usr/bin/env bash
#
# demo.sh — drive ai-assist-pager standalone so you can see the streaming UI
# without wiring up the zellij broker.
#
# It builds the pager from the working tree (so you always see your current
# code), then feeds it a stream that exercises every new feature:
#
#   1. an initial pause            → the launch "working…" spinner is visible
#   2. streamed markdown           → text appears incrementally, view tails it
#   3. DLE thinking records        → a bottom spinner re-arms with a label…
#   4. a second thinking record    → …whose label swaps while the timer keeps
#                                     running (no flicker, no reset)
#   5. more text                   → clears the spinner
#   6. the sample, dripped in      → streaming + auto-follow over real content
#
# While it's up: Space = hint labels on code buttons, ? = help modal,
# h/l = horizontal scroll on wide code, q = quit.
#
# Usage:   ./demo.sh [harness-label] [thinking-label]
# Example: ./demo.sh "Sample" "Working…"
#
# Override the binary with AI_ASSIST_PAGER_BIN=/path/to/ai-assist-pager to skip
# the build and run a specific binary instead.

set -u

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SAMPLE="$DIR/testdata/sample.md"
HARNESS="${1:-Sample}"
LABEL="${2:-Working…}"

if [[ ! -f "$SAMPLE" ]]; then
  echo "demo.sh: sample not found at $SAMPLE" >&2
  exit 1
fi

# Resolve the pager binary: an explicit override, otherwise build the working
# tree to a temp binary and clean it up on exit.
PAGER_BIN="${AI_ASSIST_PAGER_BIN:-}"
if [[ -z "$PAGER_BIN" ]]; then
  PAGER_BIN="$(mktemp -t ai-assist-pager-demo)"
  trap 'rm -f "$PAGER_BIN"' EXIT
  echo "demo.sh: building pager from $DIR …" >&2
  if ! go build -o "$PAGER_BIN" "$DIR"; then
    echo "demo.sh: build failed" >&2
    exit 1
  fi
fi

# think emits a DLE-bracketed thinking control record: DLE 't' <label> DLE.
# DLE is 0x10; printf's \x10 stops after two hex digits, so the 't' is literal.
think() { printf '\x10t%s\x10' "$1"; }

# The producer writes the stream to the pager's stdin. Each printf/sleep/cat is
# a distinct write, so the pager reads them as separate chunks (real streaming);
# keys still work because the pager reads them from /dev/tty, not this pipe.
producer() {
  sleep 1.5                                              # see the launch spinner
  printf '# Streaming demo\n\nThis paragraph streamed in.\n'
  sleep 1
  think "Searching the web…"; sleep 1.5                  # bottom spinner re-arms (timer 0)
  think "Reading 12 files…"; sleep 1.5                   # label swaps, timer keeps running
  printf '\n\nDone thinking — here is the sample:\n\n'   # text clears the spinner
  sleep 0.8
  while IFS= read -r line; do                            # drip the sample, line by line
    printf '%s\n' "$line"
    sleep 0.03
  done < "$SAMPLE"
}

producer | "$PAGER_BIN" --harness "$HARNESS" --thinking-label "$LABEL"
