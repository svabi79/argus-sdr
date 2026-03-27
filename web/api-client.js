(function (global) {
  const DEFAULT_TIMEOUT_MS = 5000;

  function toErrorMessage(err) {
    if (!err) return 'request failed';
    if (typeof err === 'string') return err;
    return err.message || 'request failed';
  }

  function createClient(opts = {}) {
    const baseUrl = opts.baseUrl || '';
    const timeoutMs = Number.isFinite(opts.timeoutMs) ? opts.timeoutMs : DEFAULT_TIMEOUT_MS;

    async function request(path, options = {}) {
      const controller = new AbortController();
      const start = performance.now();
      const timer = setTimeout(() => controller.abort(), timeoutMs);
      const init = {
        method: options.method || 'GET',
        headers: { ...(options.headers || {}) },
        signal: controller.signal,
      };
      if (options.body !== undefined) {
        init.headers['Content-Type'] = 'application/json';
        init.body = JSON.stringify(options.body);
      }

      try {
        const res = await fetch(`${baseUrl}${path}`, init);
        const durationMs = Math.round(performance.now() - start);
        const contentType = res.headers.get('content-type') || '';
        let data = null;
        if (contentType.includes('application/json')) data = await res.json();
        else data = await res.text();
        if (!res.ok) {
          const error = typeof data === 'string' ? data : (data?.error || `http ${res.status}`);
          return { ok: false, status: res.status, error, data, meta: { duration_ms: durationMs } };
        }
        return { ok: true, status: res.status, data, meta: { duration_ms: durationMs } };
      } catch (err) {
        const durationMs = Math.round(performance.now() - start);
        return { ok: false, status: 0, error: toErrorMessage(err), data: null, meta: { duration_ms: durationMs } };
      } finally {
        clearTimeout(timer);
      }
    }

    return {
      getConfig: () => request('/api/config'),
      postConfig: (payload) => request('/api/config', { method: 'POST', body: payload }),
      postSettings: (payload) => request('/api/sdr/settings', { method: 'POST', body: payload }),
      getSignals: () => request('/api/signals'),
      getStats: () => request('/api/stats'),
      getGPU: () => request('/api/gpu'),
      getPolicy: () => request('/api/pipeline/policy'),
      getRecommendations: () => request('/api/pipeline/recommendations'),
      getRefinement: () => request('/api/refinement'),
      getEvents: ({ limit, since } = {}) => {
        const params = new URLSearchParams();
        if (Number.isFinite(limit) && limit > 0) params.set('limit', String(limit));
        if (Number.isFinite(since) && since > 0) params.set('since', String(since));
        const suffix = params.toString() ? `?${params.toString()}` : '';
        return request(`/api/events${suffix}`);
      },
      getRecordings: () => request('/api/recordings'),
      getRecording: (id) => request(`/api/recordings/${encodeURIComponent(id)}`),
      decodeRecording: (id, mode) => request(`/api/recordings/${encodeURIComponent(id)}/decode?mode=${encodeURIComponent(mode || '')}`),
      getDecoders: () => request('/api/decoders'),
      getTelemetryLive: () => request('/api/debug/telemetry/live'),
    };
  }

  global.SpectreApi = { createClient };
})(window);
