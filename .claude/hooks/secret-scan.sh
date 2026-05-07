#!/bin/bash
# UserPromptSubmit — refuse the turn if the prompt looks like it leaks credentials.

INPUT=$(cat)
PROMPT=$(printf '%s' "$INPUT" | python3 -c 'import json,sys
try:
    d = json.load(sys.stdin)
    print(d.get("prompt") or d.get("user_prompt") or "")
except Exception:
    pass' 2>/dev/null)

if [ -z "$PROMPT" ]; then
    exit 0
fi

# Credential patterns:
# - OpenAI / Anthropic API keys
# - GitHub PAT (classic & fine-grained)
# - AWS access key IDs
# - Google API keys
# - Stripe live/test secret keys
# - JWT tokens (3-part base64 dot-separated)
# - Connection strings with embedded passwords (postgres://, redis://, mongodb://)
# - Generic password= / secret= assignments
if printf '%s' "$PROMPT" | grep -qE 'sk-(ant-)?[A-Za-z0-9_-]{20,}|ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{82}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|sk_(live|test)_[A-Za-z0-9]{24,}|eyJ[A-Za-z0-9_-]{20,}\.eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}|(postgres|postgresql|redis|mongodb)://[^[:space:]]+:[^[:space:]@]+@|(password|secret|token)[[:space:]]*=[[:space:]]*['\''"]?[A-Za-z0-9_-]{8,}'; then
    echo "BLOCKED: Your prompt looks like it contains a credential. Remove it (or rotate it) before resubmitting." >&2
    exit 2
fi

exit 0
