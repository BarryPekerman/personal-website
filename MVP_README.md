# MVP Terminal - Quick Start Guide

## Prerequisites

- Go installed (tested with Go 1.21+)
- A test directory with some files

## Setup Test Environment

```bash
# Create test directory structure
mkdir -p ~/mvp-test/projects/project-a
echo "Hello from Project A" > ~/mvp-test/projects/project-a/readme.txt
echo "I am a DevOps Engineer" > ~/mvp-test/cv.txt
```

## Running the MVP

### Option 1: Run from test directory (Recommended)

```bash
cd ~/mvp-test
go run /home/berry/github/personal-website/mvp-main.go
```

The server will start on `http://localhost:8080`

### Option 2: Compile and run

```bash
# Compile
go build -o mvp-server /home/berry/github/personal-website/mvp-main.go

# Run from test directory
cd ~/mvp-test
./mvp-server
```

### Option 3: Run from anywhere (specify working directory)

The server uses the current working directory where you run it as the starting point. If you want to use a specific directory:

```bash
cd ~/mvp-test
go run /home/berry/github/personal-website/mvp-main.go
```

## Testing

### Via Browser

1. Open `http://localhost:8080` in your browser
2. Try these commands:
   - `ls` - List files
   - `cd projects` - Change directory
   - `ls` - List files in projects (should show project-a)
   - `cat project-a/readme.txt` - Read file
   - `export TEST=123` - Set variable
   - `echo $TEST` - Should be empty (proves stateless)

### Via Command Line (API Testing)

```bash
SESSION_ID="my_test_session"

# Test ls
curl -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"ls\"}"

# Test cd
curl -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"cd projects\"}"

# Test ls after cd
curl -X POST http://localhost:8080/api/cmd \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"command\":\"ls\"}"
```

### Using the Test Script

```bash
# Make sure server is running first, then:
/home/berry/github/personal-website/test-mvp.sh
```

## How It Works

1. **Session Management**: Each browser tab gets a random session ID
2. **State Tracking**: The server remembers the current directory for each session ID
3. **Command Execution**: Commands run via `exec.Command` in the tracked directory
4. **Stateless**: No persistent shell - each command is independent

## Stopping the Server

Press `Ctrl+C` in the terminal where it's running, or:

```bash
pkill -f mvp-main.go
# or
pkill -f mvp-server
```

## Troubleshooting

- **Port 8080 already in use**: Change the port in `mvp-main.go` (line 155)
- **Commands not working**: Make sure you're running from the test directory
- **Permission errors**: Check file permissions in your test directory


