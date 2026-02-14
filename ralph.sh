#!/bin/bash

for i in {1..6}; do
    echo "=== Run $i of 6 ==="
    claude --dangerously-skip-permissions -p "continue. Take *one* or *two* issues and work on them spawning subagents as necessary. After successfully finishing, close the issue. If not successful, update issues. After this update handoff and land the plane" --verbose --output-format stream-json --include-partial-messages | jq
    echo ""
done
