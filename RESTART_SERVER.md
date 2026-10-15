# How to Restart the MVP Server

## Quick Steps

1. **Stop the current server:**
   ```bash
   pkill -f mvp-main.go
   ```
   Or if you started it with a compiled binary:
   ```bash
   pkill -f mvp-server
   ```

2. **Start it again:**
   ```bash
   cd ~/mvp-test
   go run /home/berry/github/personal-website/mvp-main.go
   ```

## Alternative: Find and Kill Process Manually

If `pkill` doesn't work:

```bash
# Find the process
ps aux | grep mvp-main

# Kill it using the PID (first number in the output)
kill <PID>
```

## Or Use Ctrl+C

If the server is running in your current terminal:
- Press `Ctrl+C` to stop it
- Then run `go run /home/berry/github/personal-website/mvp-main.go` again

## After Restarting

1. **Hard refresh your browser** to clear cache:
   - **Chrome/Firefox (Linux/Windows)**: `Ctrl + Shift + R`
   - **Chrome/Firefox (Mac)**: `Cmd + Shift + R`
   - Or open in incognito/private mode

2. **Test the changes:**
   - Check if input font matches terminal output
   - Try pressing Tab for auto-completion
   - Try Arrow Up/Down for command history


