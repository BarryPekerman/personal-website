#!/bin/bash
# Quick test script for the terminal

echo "🧪 Testing Portfolio Terminal"
echo "============================"
echo ""

# Check if server is running
if ! curl -s http://localhost:8080 > /dev/null 2>&1; then
    echo "❌ Server is not running on port 8080"
    echo "   Start it with: APP_MODE=local go run cmd/server/main.go"
    exit 1
fi

echo "✅ Server is running"
echo ""

# Check content directory
if [ ! -d "content" ]; then
    echo "❌ Content directory not found"
    exit 1
fi

echo "✅ Content directory exists"
echo "   Files: $(ls content/ | tr '\n' ' ')"
echo ""

# Test HTTP endpoint
echo "🌐 Testing HTTP endpoint..."
HTTP_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080)
if [ "$HTTP_RESPONSE" = "200" ]; then
    echo "✅ HTTP endpoint responding (200 OK)"
else
    echo "❌ HTTP endpoint returned: $HTTP_RESPONSE"
fi
echo ""

# Check WebSocket endpoint (should fail with HTTP, but endpoint exists)
echo "🔌 Testing WebSocket endpoint..."
WS_TEST=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ws 2>&1)
if echo "$WS_TEST" | grep -q "400\|426\|101"; then
    echo "✅ WebSocket endpoint exists (expected HTTP error for WebSocket)"
else
    echo "⚠️  WebSocket endpoint check: $WS_TEST"
fi
echo ""

echo "📋 Next Steps:"
echo "   1. Open http://localhost:8080 in your browser"
echo "   2. Test these commands in the terminal:"
echo "      - ls"
echo "      - cat cv.txt"
echo "      - pwd"
echo "      - whoami"
echo ""
echo "📝 Server logs: tail -f /tmp/portfolio-server.log"
echo ""



