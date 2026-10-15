class PortfolioTerminal {
    constructor() {
        this.term = null;
        this.ws = null;
        this.fitAddon = null;
        this.isConnected = false;
        this.scrollTimeout = null;
        this.init();
    }

    init() {
        // Get terminal element and ensure it's completely clean
        const terminalElement = document.getElementById('terminal');
        
        // Completely clear and reset the terminal element
        terminalElement.innerHTML = '';
        terminalElement.textContent = '';
        terminalElement.innerText = '';
        
        // Remove any existing terminal instance
        if (this.term) {
            this.term.dispose();
            this.term = null;
        }
        
        // Initialize xterm.js
        this.term = new Terminal({
            cursorBlink: true,
            cursorStyle: 'block',
            fontSize: 14,
            fontFamily: '"Courier New", Courier, monospace',
            theme: {
                background: '#000000',
                foreground: '#00ff00',
                cursor: '#00ff00',
            },
            allowTransparency: false,
            scrollback: 10000,
            // Ensure terminal behaves like a real terminal
            convertEol: true,
            disableStdin: false,
        });

        // Initialize addons
        this.fitAddon = new FitAddon.FitAddon();
        this.term.loadAddon(this.fitAddon);

        const webLinksAddon = new WebLinksAddon.WebLinksAddon();
        this.term.loadAddon(webLinksAddon);

        // Open terminal
        this.term.open(terminalElement);
        
        // Clear terminal to remove any cached/old content
        this.term.clear();
        this.term.reset(); // Reset terminal state completely
        
        // Force clear again after a brief delay to catch any delayed rendering
        setTimeout(() => {
            this.term.clear();
            this.term.reset();
        }, 100);
        
        this.fitTerminal();

        // Force scroll to bottom initially
        requestAnimationFrame(() => {
            this.term.scrollToBottom();
        });

        // Connect WebSocket
        this.connectWebSocket();

        // Handle resize
        window.addEventListener('resize', () => {
            this.fitTerminal();
        });

        // Handle input - send immediately (preserves special keys like TAB, arrows)
        this.term.onData(data => {
            if (this.ws && this.isConnected && this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify({
                    type: 'input',
                    data: data,
                }));
            }
        });

        // Auto-scroll on any data write
        const originalWrite = this.term.write.bind(this.term);
        this.term.write = (data) => {
            originalWrite(data);
            this.scheduleScroll();
        };
    }

    scheduleScroll() {
        // Debounce scrolling for performance
        if (this.scrollTimeout) {
            clearTimeout(this.scrollTimeout);
        }
        this.scrollTimeout = setTimeout(() => {
            this.term.scrollToBottom();
        }, 10);
    }

    fitTerminal() {
        if (this.fitAddon) {
            this.fitAddon.fit();
            // Scroll to bottom after resize
            requestAnimationFrame(() => {
                this.term.scrollToBottom();
            });
            if (this.ws && this.isConnected && this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify({
                    type: 'resize',
                    data: `${this.term.cols},${this.term.rows}`,
                }));
            }
        }
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            this.isConnected = true;
            // Clear terminal again when WebSocket connects to ensure clean state
            // Do this multiple times to catch any delayed rendering
            this.term.clear();
            this.term.reset();
            setTimeout(() => {
                this.term.clear();
                this.term.reset();
            }, 50);
            setTimeout(() => {
                this.term.clear();
                this.term.reset();
            }, 200);
            this.fitTerminal();
            // Ensure we're at bottom when connected
            requestAnimationFrame(() => {
                this.term.scrollToBottom();
            });
        };

        this.ws.onmessage = (event) => {
            // Handle binary messages (shell output)
            if (event.data instanceof Blob) {
                const reader = new FileReader();
                reader.onload = () => {
                    const text = reader.result;
                    // Write in chunks to avoid overwhelming the terminal
                    this.writeInChunks(text);
                };
                reader.readAsText(event.data);
            } else if (event.data instanceof ArrayBuffer) {
                const decoder = new TextDecoder();
                const text = decoder.decode(event.data);
                this.writeInChunks(text);
            } else if (typeof event.data === 'string') {
                // Try JSON first
                try {
                    const msg = JSON.parse(event.data);
                    if (msg.type === 'error') {
                        this.term.write(`\r\n\x1b[1;31mError: ${msg.data}\x1b[0m\r\n`);
                        this.scheduleScroll();
                    }
                } catch (e) {
                    // Not JSON, treat as raw output
                    this.writeInChunks(event.data);
                }
            }
        };

        this.ws.onclose = () => {
            this.isConnected = false;
            this.term.write('\r\n\x1b[1;33mConnection closed.\x1b[0m\r\n');
            this.scheduleScroll();
        };

        this.ws.onerror = () => {
            this.term.write('\r\n\x1b[1;31mConnection error.\x1b[0m\r\n');
            this.scheduleScroll();
        };
    }

    writeInChunks(text) {
        // Write in chunks to handle large outputs without blocking
        const chunkSize = 1024;
        let offset = 0;
        
        const writeChunk = () => {
            if (offset < text.length) {
                const chunk = text.substring(offset, offset + chunkSize);
                this.term.write(chunk);
                offset += chunkSize;
                this.scheduleScroll();
                // Use requestAnimationFrame for smooth rendering
                requestAnimationFrame(writeChunk);
            }
        };
        
        writeChunk();
    }
}

// Initialize when page loads
document.addEventListener('DOMContentLoaded', () => {
    new PortfolioTerminal();
});
