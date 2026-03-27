(function (global) {
  function fmtHz(value) {
    if (!Number.isFinite(value)) return 'n/a';
    if (value >= 1e6) return `${(value / 1e6).toFixed(3)} MHz`;
    if (value >= 1e3) return `${(value / 1e3).toFixed(2)} kHz`;
    return `${Math.round(value)} Hz`;
  }

  function fmtAgeShort(ms) {
    if (!Number.isFinite(ms) || ms < 0) return '-';
    if (ms < 1000) return `${Math.round(ms)} ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)} s`;
    return `${Math.round(ms / 60000)} min`;
  }

  function applyStatusClass(el, level) {
    if (!el) return;
    el.classList.remove('status-val--ok', 'status-val--warn', 'status-val--bad');
    if (level === 'ok') el.classList.add('status-val--ok');
    if (level === 'warn') el.classList.add('status-val--warn');
    if (level === 'bad') el.classList.add('status-val--bad');
  }

  function levelSummary(level) {
    if (!level || typeof level !== 'object') return 'n/a';
    const bits = [];
    if (level.name) bits.push(level.name);
    if (Number.isFinite(level.fft_size) && level.fft_size > 0) bits.push(`${level.fft_size} bins`);
    if (Number.isFinite(level.span_hz) && level.span_hz > 0) bits.push(fmtHz(level.span_hz));
    return bits.join(' · ') || 'n/a';
  }

  function summarizeReason(reason) {
    if (!reason) return 'n/a';
    const parts = String(reason).split(':').filter(Boolean);
    if (!parts.length) return 'n/a';
    const tail = parts.slice(-3);
    const compact = tail.join(' › ');
    return compact.length > 64 ? compact.slice(0, 61) + '…' : compact;
  }

  function renderRefinementDetails(root, refinementInfo) {
    if (!root) return;
    const plan = refinementInfo?.plan || {};
    const queue = refinementInfo?.arbitration?.queue || {};
    const summary = refinementInfo?.arbitration?.decision_summary || {};
    const reasons = summary?.reasons || {};
    const topReason = Object.entries(reasons).sort((a, b) => Number(b[1]) - Number(a[1]))[0];
    const primary = refinementInfo?.surveillance_level_set?.primary || refinementInfo?.surveillance_level;
    const display = refinementInfo?.surveillance_level_set?.presentation || refinementInfo?.display_level;
    const spans = refinementInfo?.window_summary?.refinement || refinementInfo?.window_stats || {};

    const rows = [
      `Budget: ${(plan.selected || []).length}/${plan.budget || 0}`,
      `Queue: rec ${queue.record_queued || 0} · dec ${queue.decode_queued || 0}`,
      `Drop: snr ${plan.dropped_by_snr || 0} · budget ${plan.dropped_by_budget || 0}`,
      `Reason: ${topReason ? `${summarizeReason(topReason[0])} (${topReason[1]})` : 'n/a'}`,
      `Primary: ${levelSummary(primary)}`,
      `Display: ${levelSummary(display)}`,
      `Windows: ${spans.count ? `${spans.count} · ${fmtHz(spans.min_span_hz || 0)}-${fmtHz(spans.max_span_hz || 0)}` : 'n/a'}`
    ];
    root.innerHTML = rows.map((row) => `<div class="ops-line">${row}</div>`).join('');
  }

  function renderTelemetryEvents(root, telemetryLive) {
    if (!root) return;
    const items = Array.isArray(telemetryLive?.recent_events) ? telemetryLive.recent_events.slice(0, 6) : [];
    if (!items.length) {
      root.innerHTML = '<div class="ops-line ops-line--muted">No recent telemetry events.</div>';
      return;
    }
    root.innerHTML = items.map((item) => {
      const ts = item?.timestamp ? new Date(item.timestamp).toLocaleTimeString() : '--:--:--';
      const level = item?.level || 'info';
      const name = item?.name || item?.metric || item?.category || 'event';
      const detail = item?.message || item?.detail || item?.summary || '';
      const shortDetail = detail ? String(detail).slice(0, 72) : '';
      return `<div class="ops-line"><span class="ops-level ops-level--${level}">${level}</span><span class="ops-ts">${ts}</span><span class="ops-name">${name}${shortDetail ? ` · ${shortDetail}` : ''}</span></div>`;
    }).join('');
  }

  function create(elements) {
    function updateStatus(data) {
      const {
        wsState, wsLastMessageTs, apiState, configStatusText, refinementInfo, telemetryLive, sourceAgeMs
      } = data;

      if (elements.healthWs) {
        const age = wsLastMessageTs > 0 ? fmtAgeShort(Date.now() - wsLastMessageTs) : '-';
        elements.healthWs.textContent = `${wsState} · last ${age}`;
        applyStatusClass(elements.healthWs, wsState === 'live' ? 'ok' : (wsState === 'retrying' ? 'bad' : 'warn'));
      }
      if (elements.healthApi) {
        const latency = Number.isFinite(apiState?.latencyMs) ? `${apiState.latencyMs} ms` : 'n/a';
        const isOk = !!apiState?.ok;
        elements.healthApi.textContent = isOk ? `ok · ${latency}` : `degraded · ${apiState?.lastError || 'n/a'}`;
        applyStatusClass(elements.healthApi, isOk ? 'ok' : 'bad');
      }
      if (elements.healthConfig) {
        elements.healthConfig.textContent = configStatusText || '-';
        applyStatusClass(elements.healthConfig, /failed|offline/i.test(configStatusText || '') ? 'bad' : 'ok');
      }
      if (elements.healthRefine) {
        const plan = refinementInfo?.plan || {};
        const queue = refinementInfo?.arbitration?.queue || {};
        const budget = Number(plan?.budget || 0);
        const selected = Number((plan?.selected || []).length || 0);
        elements.healthRefine.textContent = `${selected}/${budget} · q ${queue.record_queued || 0}/${queue.decode_queued || 0}`;
        applyStatusClass(elements.healthRefine, budget > 0 && selected >= budget ? 'warn' : 'ok');
      }
      if (elements.healthTelemetry) {
        if (!telemetryLive) {
          elements.healthTelemetry.textContent = 'unavailable';
          applyStatusClass(elements.healthTelemetry, 'bad');
        } else {
          const enabled = telemetryLive.enabled === false ? 'off' : 'on';
          const collector = telemetryLive.collector || {};
          const recent = Array.isArray(telemetryLive.recent_events) ? telemetryLive.recent_events.length : 0;
          const heavy = collector.heavy_enabled ? 'heavy' : 'light';
          elements.healthTelemetry.textContent = `${enabled} · ${heavy} · events ${recent}`;
          applyStatusClass(elements.healthTelemetry, enabled === 'on' ? 'ok' : 'warn');
        }
      }
      if (elements.healthSource) {
        const text = Number.isFinite(sourceAgeMs) && sourceAgeMs >= 0 ? `${sourceAgeMs} ms` : 'n/a';
        elements.healthSource.textContent = text;
        applyStatusClass(elements.healthSource, Number.isFinite(sourceAgeMs) && sourceAgeMs < 1500 ? 'ok' : 'warn');
      }
      renderRefinementDetails(elements.refineDetails, refinementInfo);
      renderTelemetryEvents(elements.telemetryEvents, telemetryLive);
    }

    return { updateStatus };
  }

  global.OperatorPanel = { create };
})(window);
