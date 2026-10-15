#!/bin/bash

SESSION_ID="test_$(date +%s)"

echo "=== Testing MVP Terminal ==="
echo "Session ID: $SESSION_ID"
echo ""

echo "1. Testing 'ls' command:"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"ls\"}")
echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print('Output:', repr(d['output'])); print('Prompt:', d['prompt'])"
echo ""

echo "2. Testing 'cd projects' command:"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"cd projects\"}")
echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print('Output:', repr(d['output'])); print('Prompt:', d['prompt'])"
echo ""

echo "3. Testing 'ls' after cd (should show project-a):"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"ls\"}")
echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print('Output:', repr(d['output'])); print('Prompt:', d['prompt'])"
echo ""

echo "4. Testing 'cat project-a/readme.txt':"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"cat project-a/readme.txt\"}")
echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print('Output:', repr(d['output'])); print('Prompt:', d['prompt'])"
echo ""

echo "5. Testing 'export TEST=123':"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"export TEST=123\"}")
echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print('Output:', repr(d['output'])); print('Prompt:', d['prompt'])"
echo ""

echo "6. Testing 'echo \$TEST' (should be empty - proves stateless):"
RESPONSE=$(curl -s -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"echo \\$TEST\"}")
echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print('Output:', repr(d['output'])); print('Prompt:', d['prompt'])"
echo ""

echo "=== Tests Complete ==="


