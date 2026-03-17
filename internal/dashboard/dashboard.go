package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/GrishaMelixov/GMRoute/internal/metrics"
)

func Register(mux *http.ServeMux) {
	mux.HandleFunc("/", handleDashboard)
	mux.HandleFunc("/events", handleSSE)
	mux.HandleFunc("/metrics", handleMetrics)
}

func handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m := metrics.Global
	json.NewEncoder(w).Encode(map[string]int64{
		"active_conns":  m.ActiveConns.Load(),
		"total_conns":   m.TotalConns.Load(),
		"direct_conns":  m.DirectConns.Load(),
		"upstream_conns": m.UpstreamConn.Load(),
		"errors":        m.Errors.Load(),
	})
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			m := metrics.Global
			data, _ := json.Marshal(map[string]int64{
				"active_conns":   m.ActiveConns.Load(),
				"total_conns":    m.TotalConns.Load(),
				"direct_conns":   m.DirectConns.Load(),
				"upstream_conns": m.UpstreamConn.Load(),
				"errors":         m.Errors.Load(),
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func handleDashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GMRoute Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f1117; color: #e2e8f0; min-height: 100vh; padding: 40px; }
  h1 { font-size: 28px; font-weight: 700; margin-bottom: 8px; color: #fff; }
  .subtitle { color: #64748b; margin-bottom: 40px; font-size: 14px; }
  .status { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #22c55e; margin-right: 8px; animation: pulse 2s infinite; }
  @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.4} }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 40px; }
  .card { background: #1e2130; border: 1px solid #2d3348; border-radius: 12px; padding: 24px; }
  .card-label { font-size: 12px; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 12px; }
  .card-value { font-size: 40px; font-weight: 700; color: #fff; line-height: 1; }
  .card-value.green { color: #22c55e; }
  .card-value.blue  { color: #3b82f6; }
  .card-value.amber { color: #f59e0b; }
  .card-value.red   { color: #ef4444; }
  .bar-section { background: #1e2130; border: 1px solid #2d3348; border-radius: 12px; padding: 24px; }
  .bar-section h2 { font-size: 14px; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 20px; }
  .bar-row { margin-bottom: 16px; }
  .bar-header { display: flex; justify-content: space-between; margin-bottom: 6px; font-size: 13px; }
  .bar-track { background: #2d3348; border-radius: 99px; height: 8px; overflow: hidden; }
  .bar-fill { height: 100%; border-radius: 99px; transition: width 0.6s ease; }
  .bar-fill.green { background: #22c55e; }
  .bar-fill.blue  { background: #3b82f6; }
  footer { margin-top: 40px; color: #2d3348; font-size: 12px; text-align: center; }
</style>
</head>
<body>
<h1><span class="status" id="dot"></span>GMRoute</h1>
<p class="subtitle">Real-time proxy traffic dashboard &nbsp;·&nbsp; localhost:9090</p>

<div class="grid">
  <div class="card">
    <div class="card-label">Active Connections</div>
    <div class="card-value green" id="active">—</div>
  </div>
  <div class="card">
    <div class="card-label">Total Connections</div>
    <div class="card-value blue" id="total">—</div>
  </div>
  <div class="card">
    <div class="card-label">Direct</div>
    <div class="card-value amber" id="direct">—</div>
  </div>
  <div class="card">
    <div class="card-label">Via Upstream</div>
    <div class="card-value blue" id="upstream">—</div>
  </div>
  <div class="card">
    <div class="card-label">Errors</div>
    <div class="card-value red" id="errors">—</div>
  </div>
</div>

<div class="bar-section">
  <h2>Routing split</h2>
  <div class="bar-row">
    <div class="bar-header"><span>Direct</span><span id="direct-pct">0%</span></div>
    <div class="bar-track"><div class="bar-fill green" id="direct-bar" style="width:0%"></div></div>
  </div>
  <div class="bar-row">
    <div class="bar-header"><span>Upstream</span><span id="upstream-pct">0%</span></div>
    <div class="bar-track"><div class="bar-fill blue" id="upstream-bar" style="width:0%"></div></div>
  </div>
</div>

<footer>Updates every second via Server-Sent Events</footer>

<script>
const $ = id => document.getElementById(id);
const es = new EventSource('/events');
es.onmessage = e => {
  const d = JSON.parse(e.data);
  $('active').textContent   = d.active_conns;
  $('total').textContent    = d.total_conns;
  $('direct').textContent   = d.direct_conns;
  $('upstream').textContent = d.upstream_conns;
  $('errors').textContent   = d.errors;
  const total = d.direct_conns + d.upstream_conns;
  const dp = total ? Math.round(d.direct_conns / total * 100) : 0;
  const up = total ? 100 - dp : 0;
  $('direct-pct').textContent   = dp + '%';
  $('upstream-pct').textContent = up + '%';
  $('direct-bar').style.width   = dp + '%';
  $('upstream-bar').style.width = up + '%';
};
es.onopen  = () => $('dot').style.background = '#22c55e';
es.onerror = () => $('dot').style.background = '#ef4444';
</script>
</body>
</html>`
