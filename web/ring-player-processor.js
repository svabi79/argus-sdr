// ring-player-processor.js — AudioWorklet processor for LiveListenWS
// Runs on the audio rendering thread, immune to main-thread blocking.

class RingPlayerProcessor extends AudioWorkletProcessor {
  constructor(options) {
    super();
    const ch = options.processorOptions?.channels || 1;
    this._channels = ch;
    // 500ms ring buffer at sampleRate
    this._ringSize = Math.ceil(sampleRate * ch * 0.5);
    this._ring = new Float32Array(this._ringSize);
    this._writePos = 0;
    this._readPos = 0;
    this._started = false;
    this._fadeGain = 1.0;
    this._startThreshold = Math.ceil(sampleRate * ch * 0.2); // 200ms

    this.port.onmessage = (e) => {
      if (e.data.type === 'pcm') {
        this._pushSamples(e.data.samples);
      }
    };
  }

  _available() {
    return (this._writePos - this._readPos + this._ringSize) % this._ringSize;
  }

  _pushSamples(float32arr) {
    const ring = this._ring;
    const size = this._ringSize;
    const n = float32arr.length;

    // Overrun: advance read cursor to make room
    const used = this._available();
    const free = size - used - 1;
    if (n > free) {
      this._readPos = (this._readPos + (n - free)) % size;
    }

    let w = this._writePos;
    // Fast path: contiguous write
    if (w + n <= size) {
      ring.set(float32arr, w);
      w += n;
      if (w >= size) w = 0;
    } else {
      // Wrap around
      const first = size - w;
      ring.set(float32arr.subarray(0, first), w);
      ring.set(float32arr.subarray(first), 0);
      w = n - first;
    }
    this._writePos = w;

    if (!this._started && this._available() >= this._startThreshold) {
      this._started = true;
    }
  }

  process(inputs, outputs, parameters) {
    const output = outputs[0];
    const outLen = output[0]?.length || 128;
    const ch = this._channels;
    const ring = this._ring;
    const size = this._ringSize;

    if (!this._started) {
      for (let c = 0; c < output.length; c++) output[c].fill(0);
      return true;
    }

    const need = outLen * ch;
    const avail = this._available();

    if (avail < need) {
      // Underrun: play what we have with fade-out, fill rest with silence
      const have = avail;
      const haveFrames = Math.floor(have / ch);
      const fadeLen = Math.min(64, haveFrames);
      const fadeStart = haveFrames - fadeLen;
      let r = this._readPos;

      for (let i = 0; i < haveFrames; i++) {
        let env = this._fadeGain;
        if (i >= fadeStart) {
          env *= 1.0 - (i - fadeStart) / fadeLen;
        }
        for (let c = 0; c < ch; c++) {
          if (c < output.length) {
            output[c][i] = ring[r] * env;
          }
          r = (r + 1) % size;
        }
      }
      this._readPos = r;

      // Silence the rest
      for (let i = haveFrames; i < outLen; i++) {
        for (let c = 0; c < output.length; c++) output[c][i] = 0;
      }
      this._fadeGain = 0;
      return true;
    }

    // Normal path
    let r = this._readPos;
    const fadeInLen = (this._fadeGain < 1.0) ? Math.min(64, outLen) : 0;

    for (let i = 0; i < outLen; i++) {
      let env = 1.0;
      if (i < fadeInLen) {
        env = this._fadeGain + (1.0 - this._fadeGain) * (i / fadeInLen);
      }
      for (let c = 0; c < ch; c++) {
        if (c < output.length) {
          output[c][i] = ring[r] * env;
        }
        r = (r + 1) % size;
      }
    }
    this._readPos = r;
    this._fadeGain = 1.0;
    return true;
  }
}

registerProcessor('ring-player-processor', RingPlayerProcessor);
