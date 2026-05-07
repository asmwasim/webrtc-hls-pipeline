#!/bin/bash
# PreToolUse(Bash) — refuse any `git push` that would update main.
#
# Why this exists:
#   The repo is on GitHub's free plan, which doesn't expose branch protection
#   for private repos. This is the client-side substitute. It catches accidents
#   from Claude Code's Bash tool but is NOT a security boundary — a determined
#   human can bypass.
#
# Patterns blocked (matched against the SHELL-LEVEL command, after stripping
# heredoc bodies and quoted string contents — so a commit message that
# *mentions* `git push origin main` as documentation is not a false positive):
#   - git push origin main
#   - git push origin main:main
#   - git push origin HEAD:main
#   - git push origin <branch>:main
#   - bare `git push` (or `git push origin`) while currently on main
#   - git push --all / git push --mirror
#
# Patterns allowed:
#   - git push origin <feature>     (working branch → its own remote)
#   - git push origin dev           (dev integration push)
#   - any non-push command

INPUT=$(cat)

exec python3 - <<'PY' "$INPUT" "${CLAUDE_PROJECT_DIR:-$PWD}"
import json
import os
import re
import subprocess
import sys

raw, project_dir = sys.argv[1], sys.argv[2]

try:
    payload = json.loads(raw)
    cmd = (payload.get("tool_input") or {}).get("command") or ""
except Exception:
    sys.exit(0)

if not cmd:
    sys.exit(0)


def strip_heredocs(s: str) -> str:
    """Remove heredoc bodies so their contents don't trip the regex."""
    pattern = re.compile(
        r"<<-?\s*'?\"?(\w+)\"?'?[^\n]*\n.*?(?:^|\n)\s*\1\s*(?=\n|$)",
        re.DOTALL | re.MULTILINE,
    )
    while True:
        m = pattern.search(s)
        if not m:
            return s
        s = s[: m.start()] + s[m.end():]


def strip_quoted(s: str) -> str:
    """Remove single- and double-quoted string contents (keep delimiters)."""
    s = re.sub(r"'[^']*'", "''", s)
    s = re.sub(r'"[^"]*"', '""', s)
    return s


cmd_clean = strip_quoted(strip_heredocs(cmd))

WORKFLOW_HINT = (
    "Branch via /start-work <type>/<name> from dev, then PR to dev. "
    "See .claude/rules/git-workflow.md."
)


def block(msg: str) -> None:
    print(f"BLOCKED: {msg}", file=sys.stderr)
    sys.exit(2)


# Pattern 1: explicit ref ending in `:main` or just `main`
if re.search(r"git\s+push\s+\S+\s+(\S+:)?main(\s|;|&|\||$)", cmd_clean):
    block(f"Direct push to main is not allowed. {WORKFLOW_HINT}")

# Pattern 2: bare `git push` or `git push origin` (no ref) while currently on main
try:
    cur_branch = subprocess.check_output(
        ["git", "branch", "--show-current"],
        cwd=project_dir,
        text=True,
        stderr=subprocess.DEVNULL,
    ).strip()
except Exception:
    cur_branch = ""

if cur_branch == "main":
    bare_push = re.search(
        r"(^|;|&&|\|\|)\s*git\s+push(\s+(origin|--[a-z-]+))*\s*(;|&|\||$)",
        cmd_clean,
    )
    if bare_push:
        block(
            "You're on main and trying to push. Direct push to main is not allowed. "
            + WORKFLOW_HINT
        )

# Pattern 3: --all or --mirror would touch main
if re.search(r"git\s+push\s+(\S+\s+)*(--all|--mirror)(\s|$|;|&|\|)", cmd_clean):
    block("'git push --all' / '--mirror' would update main. Use a feature branch and PR to dev.")

sys.exit(0)
PY
