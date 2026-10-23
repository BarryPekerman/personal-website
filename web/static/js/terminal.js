// Browser side of the portfolio terminal: xterm.js renders whatever the
// server streams back, and every keystroke goes to the server as
// {"type": "input", "data": ...}. The server owns echo, prompt and editing.
//
// lazy: open the session on the visitor's first key press or command, not on
// page load, so a visitor who only reads holds no session on the server. After
// the server closes it (idle, shell exit), a key press opens a new one.
class PortfolioTerminal {
    constructor(element, { theme, fontSize = 14, fontFamily, onInput, onOutput, onEdge, lazy = false } = {}) {
        this.onOutput = onOutput;
        this.onEdge = onEdge;
        this.ws = null;
        this.pending = [];  // messages waiting for the connection to open
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

        if (lazy) this.term.write('\x1b[2mPress any key, or pick a command below, to open a session.\x1b[0m\r\n');
        else this.connect();
        this.term.onData(data => {
            // with no session, a key press only opens one
            if (!this.ws) { this.connect(); return; }
            this.send('input', data);
            if (onInput) onInput(data);
        });
        // follow the element's size, not just the window's (it may sit in a panel)
        new ResizeObserver(() => this.fit()).observe(element);
        this.fit();
    }

    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = this.ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
        ws.binaryType = 'arraybuffer';
        const decoder = new TextDecoder();

        ws.onopen = () => {
            this.fit();
            for (const m of this.pending.splice(0)) ws.send(m);
        };
        ws.onmessage = event => {
            // the one text frame: where the connection entered Cloudflare
            if (typeof event.data === 'string') {
                try {
                    const m = JSON.parse(event.data);
                    if (m.type === 'edge' && this.onEdge) this.onEdge(m);
                } catch (e) { /* not ours */ }
                return;
            }
            // stream: true keeps multi-byte characters split across frames intact
            this.term.write(decoder.decode(event.data, { stream: true }));
            if (this.onOutput) this.onOutput();
        };
        // the server says why it closed (idle, too many sessions) in the close frame
        ws.onclose = e => {
            this.ws = null;
            this.pending = [];
            this.term.write(`\r\n\x1b[1;33m${e.reason || 'Connection closed.'} Press any key to reconnect.\x1b[0m\r\n`);
        };
        return ws;
    }

    // send delivers a message, queueing it while the connection opens
    send(type, data) {
        if (!this.ws) return;
        const m = JSON.stringify({ type, data });
        if (this.ws.readyState === WebSocket.OPEN) this.ws.send(m);
        else if (this.ws.readyState === WebSocket.CONNECTING) this.pending.push(m);
    }

    // run types a command for the visitor, as if they had pressed Enter,
    // opening a session first if there is none
    run(command) {
        if (!this.ws) this.connect();
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
