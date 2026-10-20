// Browser side of the portfolio terminal: xterm.js renders whatever the
// server streams back, and every keystroke goes to the server as
// {"type": "input", "data": ...}. The server owns echo, prompt and editing.
class PortfolioTerminal {
    constructor(element, { theme, fontSize = 14, fontFamily, onInput, onOutput } = {}) {
        this.onOutput = onOutput;
        this.term = new Terminal({
            cursorBlink: true,
            cursorStyle: 'block',
            fontSize,
            fontFamily: fontFamily || '"Courier New", Courier, monospace',
            theme: theme || {
                background: '#000000',
                foreground: '#00ff00',
                cursor: '#00ff00',
            },
            scrollback: 10000,
        });
        this.fitAddon = new FitAddon.FitAddon();
        this.term.loadAddon(this.fitAddon);
        this.term.loadAddon(new WebLinksAddon.WebLinksAddon());
        this.term.open(element);
        this.term.focus();

        this.ws = this.connect();
        this.term.onData(data => {
            this.send('input', data);
            if (onInput) onInput(data);
        });
        // follow the element's size, not just the window's (it may sit in a panel)
        new ResizeObserver(() => this.fit()).observe(element);
        this.fit();
    }

    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
        ws.binaryType = 'arraybuffer';
        const decoder = new TextDecoder();

        ws.onopen = () => this.fit();
        ws.onmessage = event => {
            // stream: true keeps multi-byte characters split across frames intact
            this.term.write(decoder.decode(event.data, { stream: true }));
            if (this.onOutput) this.onOutput();
        };
        ws.onclose = () => this.term.write('\r\n\x1b[1;33mConnection closed. Reload to reconnect.\x1b[0m\r\n');
        ws.onerror = () => this.term.write('\r\n\x1b[1;31mConnection error.\x1b[0m\r\n');
        return ws;
    }

    send(type, data) {
        if (this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type, data }));
        }
    }

    // run types a command for the visitor, as if they had pressed Enter
    run(command) {
        this.send('input', command + '\r');
        this.term.focus();
    }

    fit() {
        this.fitAddon.fit();
        this.send('resize', `${this.term.cols},${this.term.rows}`);
    }
}

// The full-screen page (/terminal) has a bare #terminal; the front page
// builds its own, themed instance
document.addEventListener('DOMContentLoaded', () => {
    const el = document.getElementById('terminal');
    if (el && !el.dataset.manual) new PortfolioTerminal(el);
});
