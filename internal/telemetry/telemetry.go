package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Enabled          bool          `json:"enabled"`
	HeavyEnabled     bool          `json:"heavy_enabled"`
	HeavySampleEvery int           `json:"heavy_sample_every"`
	MetricSampleEvery int          `json:"metric_sample_every"`
	MetricHistoryMax int           `json:"metric_history_max"`
	EventHistoryMax  int           `json:"event_history_max"`
	Retention        time.Duration `json:"retention"`
	PersistEnabled   bool          `json:"persist_enabled"`
	PersistDir       string        `json:"persist_dir"`
	RotateMB         int           `json:"rotate_mb"`
	KeepFiles        int           `json:"keep_files"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		HeavyEnabled:      false,
		HeavySampleEvery:  12,
		MetricSampleEvery: 2,
		MetricHistoryMax:  12_000,
		EventHistoryMax:   4_000,
		Retention:         15 * time.Minute,
		PersistEnabled:    false,
		PersistDir:        "debug/telemetry",
		RotateMB:          16,
		KeepFiles:         8,
	}
}

type Tags map[string]string

type MetricPoint struct {
	Timestamp time.Time `json:"ts"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Value     float64   `json:"value"`
	Tags      Tags      `json:"tags,omitempty"`
}

type Event struct {
	ID        uint64         `json:"id"`
	Timestamp time.Time      `json:"ts"`
	Name      string         `json:"name"`
	Level     string         `json:"level"`
	Message   string         `json:"message,omitempty"`
	Tags      Tags           `json:"tags,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

type SeriesValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Tags  Tags    `json:"tags,omitempty"`
}

type DistValue struct {
	Name  string  `json:"name"`
	Count int64   `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
	Last  float64 `json:"last"`
	P95   float64 `json:"p95"`
	Tags  Tags    `json:"tags,omitempty"`
}

type LiveSnapshot struct {
	Now           time.Time     `json:"now"`
	StartedAt     time.Time     `json:"started_at"`
	UptimeMs      int64         `json:"uptime_ms"`
	Config        Config        `json:"config"`
	Counters      []SeriesValue `json:"counters"`
	Gauges        []SeriesValue `json:"gauges"`
	Distributions []DistValue   `json:"distributions"`
	RecentEvents  []Event       `json:"recent_events"`
	Status        map[string]any `json:"status,omitempty"`
}

type Query struct {
	From      time.Time
	To        time.Time
	Limit     int
	Name      string
	NamePrefix string
	Level     string
	Tags      Tags
	IncludePersisted bool
}

type collectorMetric struct {
	name string
	tags Tags
	value float64
}

type distMetric struct {
	name string
	tags Tags
	count int64
	sum float64
	min float64
	max float64
	last float64
	samples []float64
	next int
	full bool
}

type persistedEnvelope struct {
	Kind   string      `json:"kind"`
	Metric *MetricPoint `json:"metric,omitempty"`
	Event  *Event      `json:"event,omitempty"`
}

type Collector struct {
	mu sync.RWMutex
	cfg Config
	startedAt time.Time
	counterSeq uint64
	heavySeq uint64
	eventSeq uint64

	counters map[string]*collectorMetric
	gauges map[string]*collectorMetric
	dists map[string]*distMetric
	metricsHistory []MetricPoint
	events []Event
	status map[string]any

	writer *jsonlWriter
}

func New(cfg Config) (*Collector, error) {
	cfg = sanitizeConfig(cfg)
	c := &Collector{
		cfg: cfg,
		startedAt: time.Now().UTC(),
		counters: map[string]*collectorMetric{},
		gauges: map[string]*collectorMetric{},
		dists: map[string]*distMetric{},
		// Pre-allocate with slack so append never reallocates and trimLocked can
		// compact in place; the buffer oscillates between max and historyCap (#29).
		metricsHistory: make([]MetricPoint, 0, historyCap(cfg.MetricHistoryMax)),
		events: make([]Event, 0, historyCap(cfg.EventHistoryMax)),
		status: map[string]any{},
	}
	if cfg.PersistEnabled {
		writer, err := newJSONLWriter(cfg)
		if err != nil {
			return nil, err
		}
		c.writer = writer
	}
	return c, nil
}

func (c *Collector) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	writer := c.writer
	c.writer = nil
	c.mu.Unlock()
	if writer != nil {
		return writer.Close()
	}
	return nil
}

func (c *Collector) Configure(cfg Config) error {
	if c == nil {
		return nil
	}
	cfg = sanitizeConfig(cfg)
	var writer *jsonlWriter
	var err error
	if cfg.PersistEnabled {
		writer, err = newJSONLWriter(cfg)
		if err != nil {
			return err
		}
	}
	c.mu.Lock()
	old := c.writer
	c.cfg = cfg
	c.writer = writer
	c.trimLocked(time.Now().UTC())
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (c *Collector) Config() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

func (c *Collector) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg.Enabled
}

func (c *Collector) ShouldSampleHeavy() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	cfg := c.cfg
	c.mu.RUnlock()
	if !cfg.Enabled || !cfg.HeavyEnabled {
		return false
	}
	n := cfg.HeavySampleEvery
	if n <= 1 {
		return true
	}
	seq := atomic.AddUint64(&c.heavySeq, 1)
	return seq%uint64(n) == 0
}

func (c *Collector) SetStatus(key string, value any) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.status[key] = value
	c.mu.Unlock()
}

func (c *Collector) IncCounter(name string, delta float64, tags Tags) {
	c.recordMetric("counter", name, delta, tags, true)
}

func (c *Collector) SetGauge(name string, value float64, tags Tags) {
	c.recordMetric("gauge", name, value, tags, false)
}

func (c *Collector) Observe(name string, value float64, tags Tags) {
	c.recordMetric("distribution", name, value, tags, false)
}

func (c *Collector) Event(name string, level string, message string, tags Tags, fields map[string]any) {
	if c == nil {
		return
	}
	now := time.Now().UTC()
	c.mu.Lock()
	if !c.cfg.Enabled {
		c.mu.Unlock()
		return
	}
	ev := Event{
		ID: atomic.AddUint64(&c.eventSeq, 1),
		Timestamp: now,
		Name: name,
		Level: strings.TrimSpace(strings.ToLower(level)),
		Message: message,
		Tags: cloneTags(tags),
		Fields: cloneFields(fields),
	}
	if ev.Level == "" {
		ev.Level = "info"
	}
	c.events = append(c.events, ev)
	c.trimLocked(now)
	writer := c.writer
	c.mu.Unlock()
	if writer != nil {
		_ = writer.Write(persistedEnvelope{Kind: "event", Event: &ev})
	}
}

func (c *Collector) recordMetric(kind string, name string, value float64, tags Tags, add bool) {
	if c == nil || strings.TrimSpace(name) == "" {
		return
	}
	now := time.Now().UTC()
	c.mu.Lock()
	if !c.cfg.Enabled {
		c.mu.Unlock()
		return
	}
	key := metricKey(name, tags)
	switch kind {
	case "counter":
		m := c.counters[key]
		if m == nil {
			m = &collectorMetric{name: name, tags: cloneTags(tags)}
			c.counters[key] = m
		}
		if add {
			m.value += value
		} else {
			m.value = value
		}
	case "gauge":
		m := c.gauges[key]
		if m == nil {
			m = &collectorMetric{name: name, tags: cloneTags(tags)}
			c.gauges[key] = m
		}
		m.value = value
	case "distribution":
		d := c.dists[key]
		if d == nil {
			d = &distMetric{
				name: name,
				tags: cloneTags(tags),
				min: value,
				max: value,
				samples: make([]float64, 64),
			}
			c.dists[key] = d
		}
		d.count++
		d.sum += value
		d.last = value
		if d.count == 1 || value < d.min {
			d.min = value
		}
		if d.count == 1 || value > d.max {
			d.max = value
		}
		if len(d.samples) > 0 {
			d.samples[d.next] = value
			d.next++
			if d.next >= len(d.samples) {
				d.next = 0
				d.full = true
			}
		}
	}
	sampleN := c.cfg.MetricSampleEvery
	seq := atomic.AddUint64(&c.counterSeq, 1)
	forceStore := strings.HasPrefix(name, "iq.extract.raw.boundary.") || strings.HasPrefix(name, "iq.extract.trimmed.boundary.")
	shouldStore := forceStore || sampleN <= 1 || seq%uint64(sampleN) == 0 || kind == "counter"
	var mp MetricPoint
	if shouldStore {
		mp = MetricPoint{
			Timestamp: now,
			Name: name,
			Type: kind,
			Value: value,
			Tags: cloneTags(tags),
		}
		c.metricsHistory = append(c.metricsHistory, mp)
	}
	c.trimLocked(now)
	writer := c.writer
	c.mu.Unlock()

	if writer != nil && shouldStore {
		_ = writer.Write(persistedEnvelope{Kind: "metric", Metric: &mp})
	}
}

func (c *Collector) LiveSnapshot() LiveSnapshot {
	now := time.Now().UTC()
	c.mu.RLock()
	cfg := c.cfg
	out := LiveSnapshot{
		Now: now,
		StartedAt: c.startedAt,
		UptimeMs: now.Sub(c.startedAt).Milliseconds(),
		Config: cfg,
		Counters: make([]SeriesValue, 0, len(c.counters)),
		Gauges: make([]SeriesValue, 0, len(c.gauges)),
		Distributions: make([]DistValue, 0, len(c.dists)),
		RecentEvents: make([]Event, 0, min(40, len(c.events))),
		Status: cloneFields(c.status),
	}
	for _, m := range c.counters {
		out.Counters = append(out.Counters, SeriesValue{Name: m.name, Value: m.value, Tags: cloneTags(m.tags)})
	}
	for _, m := range c.gauges {
		out.Gauges = append(out.Gauges, SeriesValue{Name: m.name, Value: m.value, Tags: cloneTags(m.tags)})
	}
	for _, d := range c.dists {
		mean := 0.0
		if d.count > 0 {
			mean = d.sum / float64(d.count)
		}
		out.Distributions = append(out.Distributions, DistValue{
			Name: d.name,
			Count: d.count,
			Min: d.min,
			Max: d.max,
			Mean: mean,
			Last: d.last,
			P95: p95FromDist(d),
			Tags: cloneTags(d.tags),
		})
	}
	start := len(c.events) - cap(out.RecentEvents)
	if start < 0 {
		start = 0
	}
	for _, ev := range c.events[start:] {
		out.RecentEvents = append(out.RecentEvents, copyEvent(ev))
	}
	c.mu.RUnlock()
	sort.Slice(out.Counters, func(i, j int) bool { return out.Counters[i].Name < out.Counters[j].Name })
	sort.Slice(out.Gauges, func(i, j int) bool { return out.Gauges[i].Name < out.Gauges[j].Name })
	sort.Slice(out.Distributions, func(i, j int) bool { return out.Distributions[i].Name < out.Distributions[j].Name })
	return out
}

func (c *Collector) QueryMetrics(q Query) ([]MetricPoint, error) {
	if c == nil {
		return nil, nil
	}
	q = normalizeQuery(q)
	c.mu.RLock()
	items := make([]MetricPoint, 0, len(c.metricsHistory))
	for _, m := range c.metricsHistory {
		if metricMatch(m, q) {
			items = append(items, copyMetric(m))
		}
	}
	cfg := c.cfg
	c.mu.RUnlock()
	if q.IncludePersisted && cfg.PersistEnabled {
		persisted, err := readPersistedMetrics(cfg, q)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		items = append(items, persisted...)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp.Before(items[j].Timestamp)
	})
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[len(items)-q.Limit:]
	}
	return items, nil
}

func (c *Collector) QueryEvents(q Query) ([]Event, error) {
	if c == nil {
		return nil, nil
	}
	q = normalizeQuery(q)
	c.mu.RLock()
	items := make([]Event, 0, len(c.events))
	for _, ev := range c.events {
		if eventMatch(ev, q) {
			items = append(items, copyEvent(ev))
		}
	}
	cfg := c.cfg
	c.mu.RUnlock()
	if q.IncludePersisted && cfg.PersistEnabled {
		persisted, err := readPersistedEvents(cfg, q)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		items = append(items, persisted...)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp.Before(items[j].Timestamp)
	})
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[len(items)-q.Limit:]
	}
	return items, nil
}

// historyCap is the backing capacity reserved for a history buffer whose logical
// size is bounded by max. The slack lets trimLocked compact in place (no
// allocation) only when the buffer fills, amortizing the O(n) shift over ~max
// appends instead of copying the whole window on every record (#29 / OI-04).
func historyCap(max int) int {
	if max <= 0 {
		return 0
	}
	return 2 * max
}

// historyTrimStart returns how many leading entries to drop from a history buffer
// of length n. To stay amortized O(1) (and allocation-free), trimming is deferred
// until the buffer fills to its slack capacity (2*max); the common path returns 0
// without scanning, so it neither copies nor allocates. When it does fire, it
// drops down to max recent entries plus any older than the retention cutoff. The
// extra (up to max) entries that linger between compactions are bounded by the
// reserved capacity and are still filtered by each query's own time range.
func historyTrimStart(n int, max int, expired func(i int) bool, hasRet bool) int {
	if max <= 0 {
		// Degenerate config (no count bound): retention-only, scanned each call.
		start := 0
		if hasRet {
			for start < n && expired(start) {
				start++
			}
		}
		return start
	}
	if n < 2*max {
		return 0
	}
	start := n - max
	if hasRet {
		for start < n && expired(start) {
			start++
		}
	}
	return start
}

func (c *Collector) trimLocked(now time.Time) {
	hasRet := c.cfg.Retention > 0
	cut := now.Add(-c.cfg.Retention)
	if mh := c.metricsHistory; len(mh) > 0 {
		start := historyTrimStart(len(mh), c.cfg.MetricHistoryMax, func(i int) bool {
			return mh[i].Timestamp.Before(cut)
		}, hasRet)
		if start > 0 {
			c.metricsHistory = mh[:copy(mh, mh[start:])] // in-place compaction, no allocation
		}
	}
	if ev := c.events; len(ev) > 0 {
		start := historyTrimStart(len(ev), c.cfg.EventHistoryMax, func(i int) bool {
			return ev[i].Timestamp.Before(cut)
		}, hasRet)
		if start > 0 {
			c.events = ev[:copy(ev, ev[start:])] // in-place compaction, no allocation
		}
	}
}

func sanitizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if cfg.HeavySampleEvery <= 0 {
		cfg.HeavySampleEvery = def.HeavySampleEvery
	}
	if cfg.MetricSampleEvery <= 0 {
		cfg.MetricSampleEvery = def.MetricSampleEvery
	}
	if cfg.MetricHistoryMax <= 0 {
		cfg.MetricHistoryMax = def.MetricHistoryMax
	}
	if cfg.EventHistoryMax <= 0 {
		cfg.EventHistoryMax = def.EventHistoryMax
	}
	if cfg.Retention <= 0 {
		cfg.Retention = def.Retention
	}
	if strings.TrimSpace(cfg.PersistDir) == "" {
		cfg.PersistDir = def.PersistDir
	}
	if cfg.RotateMB <= 0 {
		cfg.RotateMB = def.RotateMB
	}
	if cfg.KeepFiles <= 0 {
		cfg.KeepFiles = def.KeepFiles
	}
	return cfg
}

func normalizeQuery(q Query) Query {
	if q.Limit <= 0 || q.Limit > 5000 {
		q.Limit = 500
	}
	if q.Tags == nil {
		q.Tags = Tags{}
	}
	return q
}

func metricMatch(m MetricPoint, q Query) bool {
	if !q.From.IsZero() && m.Timestamp.Before(q.From) {
		return false
	}
	if !q.To.IsZero() && m.Timestamp.After(q.To) {
		return false
	}
	if q.Name != "" && m.Name != q.Name {
		return false
	}
	if q.NamePrefix != "" && !strings.HasPrefix(m.Name, q.NamePrefix) {
		return false
	}
	for k, v := range q.Tags {
		if m.Tags[k] != v {
			return false
		}
	}
	return true
}

func eventMatch(ev Event, q Query) bool {
	if !q.From.IsZero() && ev.Timestamp.Before(q.From) {
		return false
	}
	if !q.To.IsZero() && ev.Timestamp.After(q.To) {
		return false
	}
	if q.Name != "" && ev.Name != q.Name {
		return false
	}
	if q.NamePrefix != "" && !strings.HasPrefix(ev.Name, q.NamePrefix) {
		return false
	}
	if q.Level != "" && !strings.EqualFold(q.Level, ev.Level) {
		return false
	}
	for k, v := range q.Tags {
		if ev.Tags[k] != v {
			return false
		}
	}
	return true
}

func metricKey(name string, tags Tags) string {
	if len(tags) == 0 {
		return name
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.Grow(len(name) + len(keys)*16)
	b.WriteString(name)
	for _, k := range keys {
		b.WriteString("|")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(tags[k])
	}
	return b.String()
}

func cloneTags(tags Tags) Tags {
	if len(tags) == 0 {
		return nil
	}
	out := make(Tags, len(tags))
	for k, v := range tags {
		out[k] = v
	}
	return out
}

func cloneFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = v
	}
	return out
}

func copyMetric(m MetricPoint) MetricPoint {
	return MetricPoint{
		Timestamp: m.Timestamp,
		Name: m.Name,
		Type: m.Type,
		Value: m.Value,
		Tags: cloneTags(m.Tags),
	}
}

func copyEvent(ev Event) Event {
	return Event{
		ID: ev.ID,
		Timestamp: ev.Timestamp,
		Name: ev.Name,
		Level: ev.Level,
		Message: ev.Message,
		Tags: cloneTags(ev.Tags),
		Fields: cloneFields(ev.Fields),
	}
}

func p95FromDist(d *distMetric) float64 {
	if d == nil || d.count == 0 {
		return 0
	}
	n := d.next
	if d.full {
		n = len(d.samples)
	}
	if n <= 0 {
		return d.last
	}
	buf := make([]float64, n)
	copy(buf, d.samples[:n])
	sort.Float64s(buf)
	idx := int(float64(n-1) * 0.95)
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return buf[idx]
}

type jsonlWriter struct {
	cfg Config
	mu sync.Mutex
	dir string
	f *os.File
	w *bufio.Writer
	currentPath string
	currentSize int64
	seq int64
}

func newJSONLWriter(cfg Config) (*jsonlWriter, error) {
	dir := filepath.Clean(cfg.PersistDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &jsonlWriter{cfg: cfg, dir: dir}
	if err := w.rotateLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *jsonlWriter) Write(v persistedEnvelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil || w.w == nil {
		return nil
	}
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if w.currentSize+int64(len(line)) > int64(w.cfg.RotateMB)*1024*1024 {
		if err := w.rotateLocked(); err != nil {
			return err
		}
	}
	n, err := w.w.Write(line)
	w.currentSize += int64(n)
	if err != nil {
		return err
	}
	return w.w.Flush()
}

func (w *jsonlWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.w != nil {
		_ = w.w.Flush()
	}
	if w.f != nil {
		err := w.f.Close()
		w.f = nil
		w.w = nil
		return err
	}
	return nil
}

func (w *jsonlWriter) rotateLocked() error {
	if w.w != nil {
		_ = w.w.Flush()
	}
	if w.f != nil {
		_ = w.f.Close()
	}
	w.seq++
	name := fmt.Sprintf("telemetry-%s-%04d.jsonl", time.Now().UTC().Format("20060102-150405"), w.seq)
	path := filepath.Join(w.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	info, _ := f.Stat()
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	w.f = f
	w.w = bufio.NewWriterSize(f, 64*1024)
	w.currentPath = path
	w.currentSize = size
	_ = pruneFiles(w.dir, w.cfg.KeepFiles)
	return nil
}

func pruneFiles(dir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	files := make([]string, 0, len(ents))
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, "telemetry-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	if len(files) <= keep {
		return nil
	}
	sort.Strings(files)
	for _, path := range files[:len(files)-keep] {
		_ = os.Remove(path)
	}
	return nil
}

func readPersistedMetrics(cfg Config, q Query) ([]MetricPoint, error) {
	files, err := listPersistedFiles(cfg.PersistDir)
	if err != nil {
		return nil, err
	}
	out := make([]MetricPoint, 0, 256)
	for _, path := range files {
		points, err := parsePersistedFile(path, q)
		if err != nil {
			continue
		}
		for _, p := range points.metrics {
			if metricMatch(p, q) {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

func readPersistedEvents(cfg Config, q Query) ([]Event, error) {
	files, err := listPersistedFiles(cfg.PersistDir)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, 128)
	for _, path := range files {
		points, err := parsePersistedFile(path, q)
		if err != nil {
			continue
		}
		for _, ev := range points.events {
			if eventMatch(ev, q) {
				out = append(out, ev)
			}
		}
	}
	return out, nil
}

type parsedFile struct {
	metrics []MetricPoint
	events  []Event
}

func parsePersistedFile(path string, q Query) (parsedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return parsedFile{}, err
	}
	defer f.Close()
	out := parsedFile{
		metrics: make([]MetricPoint, 0, 64),
		events: make([]Event, 0, 32),
	}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 32*1024), 1024*1024)
	for s.Scan() {
		line := s.Bytes()
		if len(line) == 0 {
			continue
		}
		var env persistedEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		if env.Metric != nil {
			out.metrics = append(out.metrics, *env.Metric)
		} else if env.Event != nil {
			out.events = append(out.events, *env.Event)
		}
		if q.Limit > 0 && len(out.metrics)+len(out.events) > q.Limit*2 {
			// keep bounded while scanning
			if len(out.metrics) > q.Limit {
				out.metrics = out.metrics[len(out.metrics)-q.Limit:]
			}
			if len(out.events) > q.Limit {
				out.events = out.events[len(out.events)-q.Limit:]
			}
		}
	}
	return out, s.Err()
}

func listPersistedFiles(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(ents))
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasPrefix(name, "telemetry-") && strings.HasSuffix(name, ".jsonl") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

func ParseTimeQuery(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if ms > 1e12 {
			return time.UnixMilli(ms).UTC(), nil
		}
		return time.Unix(ms, 0).UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("invalid time query")
}

func TagsWith(base Tags, key string, value any) Tags {
	out := cloneTags(base)
	if out == nil {
		out = Tags{}
	}
	out[key] = fmt.Sprint(value)
	return out
}

func TagsFromPairs(kv ...string) Tags {
	if len(kv) < 2 {
		return nil
	}
	out := Tags{}
	for i := 0; i+1 < len(kv); i += 2 {
		k := strings.TrimSpace(kv[i])
		if k == "" {
			continue
		}
		out[k] = kv[i+1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
