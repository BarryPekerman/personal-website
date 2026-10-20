// Sound effects and ambient music for the front page: generated live with Web
// Audio, both off by default, each with its own toggle (#sfx, #music).
// ---------- sound effects: separate toggle, off by default ----------
let sharedCtx = null;
function audioContext() {
  if (!sharedCtx) sharedCtx = new AudioContext();
  return sharedCtx;
}

const Sfx = {
  on: false, out: null, noiseBuf: null,

  init() {
    const ctx = audioContext();
    this.out = ctx.createGain();
    this.out.gain.value = 0.9;
    this.out.connect(ctx.destination);
    // one second of white noise, reused for clicks
    this.noiseBuf = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
    const data = this.noiseBuf.getChannelData(0);
    for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1;
  },

  // a short tone, optionally sliding in pitch
  tone(freq, dur, { type = 'square', vol = 0.05, delay = 0, slideTo } = {}) {
    if (!this.on) return;
    const ctx = audioContext();
    const t = ctx.currentTime + delay;
    const osc = ctx.createOscillator();
    const g = ctx.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(freq, t);
    if (slideTo) osc.frequency.exponentialRampToValueAtTime(slideTo, t + dur);
    g.gain.setValueAtTime(vol, t);
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    osc.connect(g);
    g.connect(this.out);
    osc.start(t);
    osc.stop(t + dur + 0.02);
  },

  // a filtered noise tick, like a keyboard click
  tick(dur = 0.015, vol = 0.08) {
    if (!this.on) return;
    const ctx = audioContext();
    const src = ctx.createBufferSource();
    src.buffer = this.noiseBuf;
    const hp = ctx.createBiquadFilter();
    hp.type = 'highpass';
    hp.frequency.value = 3000;
    const g = ctx.createGain();
    const t = ctx.currentTime;
    g.gain.setValueAtTime(vol, t);
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    src.connect(hp); hp.connect(g); g.connect(this.out);
    src.start(t, Math.random() * 0.9, dur + 0.01);
  },

  key()       { this.tick(0.012 + Math.random() * 0.01, 0.06 + Math.random() * 0.04); },
  click()     { this.tone(1400, 0.03, { vol: 0.04 }); this.tone(900, 0.04, { vol: 0.03, delay: 0.03 }); },
  enter()     { this.tone(520, 0.08, { vol: 0.04, slideTo: 780 }); },
  hop(i)      { this.tone(660 + i * 220, 0.1, { vol: 0.04 }); },
  connected() { this.tone(784, 0.08, { vol: 0.05 }); this.tone(1047, 0.14, { vol: 0.05, delay: 0.09 }); },
  denied()    { this.tone(220, 0.25, { vol: 0.06, slideTo: 110 }); },
  error()     { this.tone(140, 0.12, { vol: 0.06 }); this.tone(140, 0.12, { vol: 0.06, delay: 0.16 }); },
  traceTick(p) { this.tone(300 + p * 6, 0.05, { type: 'sine', vol: 0.08 }); },
  alarm()     { for (let i = 0; i < 6; i++) this.tone(i % 2 ? 660 : 880, 0.11, { vol: 0.045, delay: i * 0.13 }); },

  toggle() {
    if (!this.out) this.init();
    audioContext().resume();
    this.on = !this.on;
    document.getElementById('sfx').textContent = this.on ? 'sfx on' : 'sfx off';
    try { localStorage.setItem('sfx', this.on ? 'on' : 'off'); } catch (e) { /* storage blocked */ }
  },
};
document.getElementById('sfx').addEventListener('click', () => Sfx.toggle());
// one click sound for every button and system entry
document.addEventListener('click', e => { if (e.target.closest('.btn, .sys')) Sfx.click(); });
try {
  if (localStorage.getItem('sfx') === 'on') document.getElementById('sfx').textContent = 'sfx off (click to resume)';
} catch (e) { /* storage blocked */ }

// ---------- ambient music: generated live, off by default ----------
// A slow minor progression (Am - Fmaj7 - Dm - E) on detuned saw pads through
// a low-pass filter and echo, a soft low pulse, and sparse bell notes.
const Music = {
  ctx: null, master: null, on: false, timer: null, step: 0,
  CHORDS: [
    [110.0, 130.8, 164.8, 220.0],  // Am
    [87.31, 110.0, 130.8, 164.8],  // Fmaj7
    [73.42, 146.8, 174.6, 220.0],  // Dm
    [82.41, 207.7, 246.9, 329.6],  // E (the tense one)
  ],
  BAR: 6, // seconds per chord

  init() {
    const ctx = this.ctx = audioContext();
    this.master = ctx.createGain();
    this.master.gain.value = 0;
    const filter = ctx.createBiquadFilter();
    filter.type = 'lowpass';
    filter.frequency.value = 900;
    const delay = ctx.createDelay();
    delay.delayTime.value = 0.45;
    const feedback = ctx.createGain();
    feedback.gain.value = 0.35;
    this.master.connect(filter);
    filter.connect(ctx.destination);
    filter.connect(delay);
    delay.connect(feedback);
    feedback.connect(delay);
    delay.connect(ctx.destination);
  },

  voice(freq, start, dur, type, peak, detune = 0) {
    const ctx = this.ctx;
    const osc = ctx.createOscillator();
    const g = ctx.createGain();
    osc.type = type;
    osc.frequency.value = freq;
    osc.detune.value = detune;
    g.gain.setValueAtTime(0, start);
    g.gain.linearRampToValueAtTime(peak, start + Math.min(2, dur / 3));
    g.gain.linearRampToValueAtTime(0, start + dur);
    osc.connect(g);
    g.connect(this.master);
    osc.start(start);
    osc.stop(start + dur + 0.1);
  },

  bar() {
    const t = this.ctx.currentTime + 0.05;
    const chord = this.CHORDS[this.step++ % this.CHORDS.length];
    chord.forEach(f => {
      this.voice(f, t, this.BAR + 1.5, 'sawtooth', 0.035, -7);
      this.voice(f, t, this.BAR + 1.5, 'sawtooth', 0.035, +7);
    });
    for (let beat = 0; beat < this.BAR; beat += 1.5) {          // low pulse
      this.voice(chord[0] / 2, t + beat, 0.9, 'sine', 0.12);
    }
    for (let i = 0; i < 3; i++) {                                 // sparse bells
      if (Math.random() < 0.6) {
        const f = chord[1 + (Math.random() * 3 | 0)] * 4;
        this.voice(f, t + Math.random() * this.BAR, 1.6, 'triangle', 0.025);
      }
    }
  },

  toggle() {
    if (!this.ctx) this.init();
    this.on = !this.on;
    const now = this.ctx.currentTime;
    this.master.gain.cancelScheduledValues(now);
    this.master.gain.setValueAtTime(this.master.gain.value, now);
    if (this.on) {
      this.ctx.resume();
      this.master.gain.linearRampToValueAtTime(0.8, now + 2);   // fade in
      this.bar();
      this.timer = setInterval(() => this.bar(), this.BAR * 1000);
    } else {
      this.master.gain.linearRampToValueAtTime(0, now + 1);     // fade out
      clearInterval(this.timer);
    }
    document.getElementById('music').textContent = this.on ? '♪ on' : '♪ off';
    try { localStorage.setItem('music', this.on ? 'on' : 'off'); } catch (e) { /* storage blocked */ }
  },
};
document.getElementById('music').addEventListener('click', () => Music.toggle());
// Browsers forbid sound before a user gesture, so a remembered "on" only shows as a hint
try {
  if (localStorage.getItem('music') === 'on') document.getElementById('music').textContent = '♪ off (click to resume)';
} catch (e) { /* storage blocked */ }
