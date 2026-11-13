#!/usr/bin/env bash
# Fake `claude -p --input-format stream-json`. For every line read on stdin it
# prints the fixture for $FAKE_CLAUDE_SCENARIO (testdata/<scenario>.jsonl).
# Scenarios: normal, tooluse, badjson, hang (reads, never answers),
# crash (first process dies mid-turn, later ones behave like "normal").
# $FAKE_CLAUDE_LOG gets one line of arguments per spawn.
dir="$(cd "$(dirname "$0")" && pwd)"
scenario="${FAKE_CLAUDE_SCENARIO:-normal}"
[ -n "$FAKE_CLAUDE_LOG" ] && echo "$*" >>"$FAKE_CLAUDE_LOG"
while IFS= read -r line; do
  case "$scenario" in
    hang) continue ;;
    crash)
      marker="${FAKE_CLAUDE_LOG}.crashed"
      if [ ! -e "$marker" ]; then
        touch "$marker"
        cat "$dir/crash.jsonl"
        exit 1
      fi
      cat "$dir/normal.jsonl" ;;
    *) cat "$dir/$scenario.jsonl" ;;
  esac
done
