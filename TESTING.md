# Testing the Local Terminal

## Quick Start

1. **Start the server:**
   ```bash
   APP_MODE=local go run cmd/server/main.go
   ```
   Or use the script:
   ```bash
   ./run-local.sh
   ```

2. **Open in browser:**
   ```
   http://localhost:8080
   ```

## Test Cases

### Basic Functionality

1. **Terminal Display**
   - ✅ Terminal should show a prompt (e.g., `guest@portfolio:~$`)
   - ✅ Cursor should blink
   - ✅ Green text on black background

2. **Input Handling**
   - ✅ Type characters - they should appear immediately
   - ✅ Backspace should work
   - ✅ Enter should execute commands

3. **Basic Commands**
   ```bash
   ls                    # Should list files
   cat cv.txt           # Should display CV content
   cat contact_info.md  # Should display contact info
   ls projects/         # Should list projects
   pwd                  # Should show /path/to/content
   whoami               # Should show 'guest'
   ```

4. **Terminal Features**
   - ✅ Tab completion (if available)
   - ✅ Arrow keys for history (if available)
   - ✅ Clear command works
   - ✅ Colors display correctly

### Expected Behavior

- **Prompt**: Should show `guest@portfolio:~/content$` or similar
- **Working Directory**: Should be in the `content/` directory
- **Files Available**: `cv.txt`, `contact_info.md`, `projects/`
- **Response Time**: Commands should execute immediately (<100ms)
- **Output**: Should display correctly with proper formatting

### Troubleshooting

**Terminal not responding:**
- Check browser console for WebSocket errors
- Check server logs: `tail -f /tmp/portfolio-server.log`
- Verify server is running: `curl http://localhost:8080`

**No output:**
- Check if content directory exists: `ls content/`
- Check server logs for errors
- Try refreshing the browser

**Commands not working:**
- Verify you're in local mode (check server startup logs)
- Check if `/bin/sh` is available: `which sh`
- Check content directory permissions

## Server Logs

Watch server logs in real-time:
```bash
tail -f /tmp/portfolio-server.log
```

Or if running in foreground, logs appear in terminal.

## WebSocket Testing

You can test WebSocket connection manually:
```bash
# Install wscat if needed: npm install -g wscat
wscat -c ws://localhost:8080/ws
```

Then send messages:
```json
{"type":"input","data":"ls\n"}
{"type":"resize","data":"80,24"}
```



