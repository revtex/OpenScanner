#!/usr/bin/env bash
# Human-in-the-loop reproduction loop.
# Copy this file, edit the steps below, and run it.
# The agent runs the script; the user follows prompts in their terminal.
#
# Usage:
#   bash hitl-loop.template.sh
#
# Two helpers:
#   step "<instruction>"          → show instruction, wait for Enter
#   capture VAR "<question>"      → show question, read response into VAR
#
# At the end, captured values are printed as KEY=VALUE for the agent to parse.

set -euo pipefail

step() {
  printf '\n>>> %s\n' "$1"
  read -r -p "    [Enter when done] " _
}

capture() {
  local var="$1" question="$2" answer
  printf '\n>>> %s\n' "$question"
  read -r -p "    > " answer
  printf -v "$var" '%s' "$answer"
}

# --- edit below ---------------------------------------------------------

# Example: a real-device check that emulation cannot answer. Replace the
# steps with whatever your loop needs. Never hard-code a hostname or
# credentials here -- this file is committed.

step "Open Squelch on the phone and sign in."

step "Turn BKGND on and wait for one call to play."

step "Lock the screen and wait two minutes."

capture PLAYED "Did audio keep playing while locked? (y/n)"

capture PAUSED "Press pause on the lock screen. Did the audio stop? (y/n)"

capture NOTES "Anything else you noticed (or 'none'):"

# --- edit above ---------------------------------------------------------

printf '\n--- Captured ---\n'
printf 'PLAYED=%s\n' "$PLAYED"
printf 'PAUSED=%s\n' "$PAUSED"
printf 'NOTES=%s\n' "$NOTES"
