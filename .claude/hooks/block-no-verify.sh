#!/bin/bash
# PreToolUse(Bash) — block git --no-verify.
# Force-push blocking is handled by block-direct-push-main.sh.
# Hook input arrives on stdin as JSON: { tool_name, tool_input: { command, ... }, ... }

INPUT=$(cat)
CMD=$(printf '%s' "$INPUT" | python3 -c 'import json,sys
try:
    d = json.load(sys.stdin)
    print((d.get("tool_input") or {}).get("command") or "")
except Exception:
    pass' 2>/dev/null)

if [ -z "$CMD" ]; then
    exit 0
fi

if printf '%s' "$CMD" | grep -qE -- '(^|[[:space:]])--no-verify([[:space:]]|$)'; then
    echo "BLOCKED: --no-verify is not allowed. Fix the underlying hook failure instead." >&2
    exit 2
fi

exit 0
