package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/GrishaMelixov/GMRoute/internal/config"
	"github.com/GrishaMelixov/GMRoute/internal/connlog"
	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/metrics"
	"github.com/GrishaMelixov/GMRoute/internal/router"
)

type Dashboard struct {
	r          *router.Router
	f          *failover.Failover
	cfg        *config.Config
	configPath string
}

func New(r *router.Router, f *failover.Failover, cfg *config.Config, configPath string) *Dashboard {
	return &Dashboard{r: r, f: f, cfg: cfg, configPath: configPath}
}

// Register keeps backward compat for callers that don't need settings.
func Register(mux *http.ServeMux) {
	d := &Dashboard{}
	d.register(mux)
}

func (d *Dashboard) Register(mux *http.ServeMux) {
	d.register(mux)
}

func (d *Dashboard) register(mux *http.ServeMux) {
	mux.HandleFunc("/", d.handleIndex)
	mux.HandleFunc("/events", d.handleSSE)
	mux.HandleFunc("/api/config", d.handleConfig)
	mux.HandleFunc("/api/rules", d.handleRules)
	mux.HandleFunc("/api/connections", d.handleConnections)
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

func (d *Dashboard) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := connlog.Global.Subscribe()
	defer connlog.Global.Unsubscribe(ch)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: connection\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			m := metrics.Global
			data, _ := json.Marshal(map[string]int64{
				"active_conns":   m.ActiveConns.Load(),
				"total_conns":    m.TotalConns.Load(),
				"direct_conns":   m.DirectConns.Load(),
				"upstream_conns": m.UpstreamConn.Load(),
				"errors":         m.Errors.Load(),
			})
			fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (d *Dashboard) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if d.r == nil {
		json.NewEncoder(w).Encode(map[string]any{"upstream": "", "rules": []any{}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"upstream": d.cfg.Upstream,
		"rules":    d.r.GetRules(),
	})
}

func (d *Dashboard) handleRules(w http.ResponseWriter, r *http.Request) {
	if d.r == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Domain string `json:"domain"`
			Route  string `json:"route"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var route router.Route
		if body.Route == "upstream" && d.cfg.Upstream != "" {
			route = router.NewUpstreamRoute(d.cfg.Upstream)
		} else {
			route = router.RouteDirectly
			body.Route = "direct"
		}
		d.r.AddRule(body.Domain, route)
		d.f.ClearCache(body.Domain)
		d.saveConfig()
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			http.Error(w, "domain required", http.StatusBadRequest)
			return
		}
		d.r.RemoveRule(domain)
		d.f.ClearCache(domain)
		d.saveConfig()
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (d *Dashboard) handleConnections(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connlog.Global.Recent())
}

func (d *Dashboard) saveConfig() {
	if d.configPath == "" || d.cfg == nil {
		return
	}
	// sync rules from router back to config
	entries := d.r.GetRules()
	d.cfg.Rules = make([]config.Rule, 0, len(entries))
	for _, e := range entries {
		d.cfg.Rules = append(d.cfg.Rules, config.Rule{Domain: e.Domain, Route: e.Route})
	}
	data, err := yaml.Marshal(d.cfg)
	if err != nil {
		return
	}
	os.WriteFile(d.configPath, data, 0644)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GMRoute Dashboard</title>
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#0a0e1a;color:#e2e8f0;height:100vh;display:flex;flex-direction:column;overflow:hidden}
  /* top bar */
  .topbar{display:flex;align-items:center;gap:24px;padding:14px 24px;background:#0f1420;border-bottom:1px solid #1e2535;flex-shrink:0}
  .logo{font-size:18px;font-weight:700;color:#fff;display:flex;align-items:center;gap:8px}
  .dot{width:8px;height:8px;border-radius:50%;background:#22c55e;animation:pulse 2s infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.3}}
  .stat{display:flex;flex-direction:column;align-items:center;min-width:64px}
  .stat-val{font-size:22px;font-weight:700;line-height:1}
  .stat-lbl{font-size:10px;color:#475569;text-transform:uppercase;letter-spacing:.05em;margin-top:2px}
  .green{color:#22c55e}.blue{color:#3b82f6}.amber{color:#f59e0b}.red{color:#ef4444}.purple{color:#a78bfa}
  .sep{width:1px;height:32px;background:#1e2535}
  .spacer{flex:1}
  .btn{background:#1e2535;border:1px solid #2d3a52;color:#94a3b8;padding:6px 14px;border-radius:8px;cursor:pointer;font-size:13px;transition:all .15s}
  .btn:hover{background:#2d3a52;color:#e2e8f0}
  /* main layout */
  .main{display:flex;flex:1;overflow:hidden}
  /* globe */
  #globe-wrap{flex:1;position:relative;overflow:hidden}
  #globe{width:100%;height:100%}
  /* sidebar */
  .sidebar{width:360px;background:#0f1420;border-left:1px solid #1e2535;display:flex;flex-direction:column;flex-shrink:0}
  .sidebar-header{padding:16px 20px;border-bottom:1px solid #1e2535;font-size:13px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:.05em;display:flex;justify-content:space-between;align-items:center}
  .filter-btns{display:flex;gap:6px}
  .fbtn{background:#1e2535;border:1px solid #2d3a52;color:#64748b;padding:3px 10px;border-radius:6px;cursor:pointer;font-size:11px;transition:all .15s}
  .fbtn.active{background:#3b82f6;border-color:#3b82f6;color:#fff}
  .conn-list{flex:1;overflow-y:auto;padding:8px 0}
  .conn-list::-webkit-scrollbar{width:4px}
  .conn-list::-webkit-scrollbar-track{background:transparent}
  .conn-list::-webkit-scrollbar-thumb{background:#2d3a52;border-radius:2px}
  .conn-item{display:flex;align-items:center;gap:10px;padding:8px 20px;border-bottom:1px solid #0f1420;transition:background .1s;cursor:default}
  .conn-item:hover{background:#1e2535}
  .badge{font-size:10px;font-weight:600;padding:2px 7px;border-radius:4px;text-transform:uppercase;flex-shrink:0}
  .badge-direct{background:#14532d;color:#22c55e}
  .badge-upstream{background:#1e3a5f;color:#3b82f6}
  .conn-domain{font-size:12px;color:#cbd5e1;flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .conn-meta{font-size:10px;color:#475569;flex-shrink:0;text-align:right}
  /* settings panel */
  .settings-panel{position:fixed;top:0;right:-420px;width:420px;height:100vh;background:#0f1420;border-left:1px solid #1e2535;z-index:100;transition:right .25s ease;overflow-y:auto;padding:24px}
  .settings-panel.open{right:0}
  .settings-panel h2{font-size:16px;font-weight:600;margin-bottom:20px;display:flex;justify-content:space-between;align-items:center}
  .close-btn{background:none;border:none;color:#64748b;cursor:pointer;font-size:20px;line-height:1}
  .close-btn:hover{color:#e2e8f0}
  .form-group{margin-bottom:16px}
  .form-label{font-size:12px;color:#64748b;margin-bottom:6px;display:block}
  .form-input{width:100%;background:#1e2535;border:1px solid #2d3a52;color:#e2e8f0;padding:8px 12px;border-radius:8px;font-size:13px;outline:none}
  .form-input:focus{border-color:#3b82f6}
  .form-select{width:100%;background:#1e2535;border:1px solid #2d3a52;color:#e2e8f0;padding:8px 12px;border-radius:8px;font-size:13px;outline:none;cursor:pointer}
  .save-btn{background:#3b82f6;border:none;color:#fff;padding:8px 16px;border-radius:8px;cursor:pointer;font-size:13px;font-weight:600;width:100%;margin-top:4px;transition:background .15s}
  .save-btn:hover{background:#2563eb}
  .rule-item{display:flex;align-items:center;gap:8px;padding:8px 12px;background:#1e2535;border-radius:8px;margin-bottom:6px}
  .rule-domain{flex:1;font-size:13px;color:#e2e8f0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .del-btn{background:none;border:none;color:#64748b;cursor:pointer;font-size:14px;flex-shrink:0;line-height:1}
  .del-btn:hover{color:#ef4444}
  .divider{border:none;border-top:1px solid #1e2535;margin:20px 0}
  .overlay{position:fixed;inset:0;background:rgba(0,0,0,.5);z-index:99;display:none}
  .overlay.open{display:block}
  .empty{padding:40px 20px;text-align:center;color:#475569;font-size:13px}
</style>
</head>
<body>

<div class="topbar">
  <div class="logo"><span class="dot" id="dot"></span>GMRoute</div>
  <div class="sep"></div>
  <div class="stat"><div class="stat-val green" id="s-active">&#x2014;</div><div class="stat-lbl">Active</div></div>
  <div class="stat"><div class="stat-val blue" id="s-total">&#x2014;</div><div class="stat-lbl">Total</div></div>
  <div class="stat"><div class="stat-val amber" id="s-direct">&#x2014;</div><div class="stat-lbl">Direct</div></div>
  <div class="stat"><div class="stat-val purple" id="s-upstream">&#x2014;</div><div class="stat-lbl">Upstream</div></div>
  <div class="stat"><div class="stat-val red" id="s-errors">&#x2014;</div><div class="stat-lbl">Errors</div></div>
  <div class="spacer"></div>
  <button class="btn" onclick="openSettings()">&#9881; Settings</button>
</div>

<div class="main">
  <div id="globe-wrap"><div id="globe"></div></div>
  <div class="sidebar">
    <div class="sidebar-header">
      <span>Connections</span>
      <div class="filter-btns">
        <button class="fbtn active" onclick="setFilter('all',this)">All</button>
        <button class="fbtn" onclick="setFilter('direct',this)">Direct</button>
        <button class="fbtn" onclick="setFilter('upstream',this)">Upstream</button>
      </div>
    </div>
    <div class="conn-list" id="conn-list">
      <div class="empty" id="empty-msg">No connections yet.<br>Configure your browser to use SOCKS5 proxy at localhost:1080</div>
    </div>
  </div>
</div>

<div class="overlay" id="overlay" onclick="closeSettings()"></div>
<div class="settings-panel" id="settings-panel">
  <h2>Settings <button class="close-btn" onclick="closeSettings()">&#x2715;</button></h2>

  <div class="form-group">
    <label class="form-label">Upstream SOCKS5 Proxy</label>
    <input class="form-input" id="upstream-input" placeholder="127.0.0.1:7890" />
  </div>
  <button class="save-btn" onclick="saveUpstream()">Save Upstream</button>

  <hr class="divider">

  <div class="form-group">
    <label class="form-label">Add Routing Rule</label>
    <input class="form-input" id="rule-domain" placeholder="youtube.com" style="margin-bottom:8px"/>
    <select class="form-select" id="rule-route">
      <option value="upstream">Via Upstream</option>
      <option value="direct">Direct</option>
    </select>
  </div>
  <button class="save-btn" onclick="addRule()">Add Rule</button>

  <hr class="divider">

  <div class="form-label" style="margin-bottom:10px">Current Rules</div>
  <div id="rules-list"></div>
</div>

<script src="//unpkg.com/globe.gl@2"></script>
<script>
// Globe
const countries = [
  {n:'Russia',lat:61.5,lng:105.3},{n:'USA',lat:37.1,lng:-95.7},{n:'China',lat:35.9,lng:104.2},
  {n:'Brazil',lat:-14.2,lng:-51.9},{n:'Australia',lat:-25.3,lng:133.8},{n:'India',lat:20.6,lng:78.9},
  {n:'Canada',lat:56.1,lng:-106.3},{n:'Argentina',lat:-38.4,lng:-63.6},{n:'Germany',lat:51.2,lng:10.5},
  {n:'France',lat:46.2,lng:2.2},{n:'UK',lat:55.4,lng:-3.4},{n:'Japan',lat:36.2,lng:138.3},
  {n:'Mexico',lat:23.6,lng:-102.6},{n:'Indonesia',lat:-0.8,lng:113.9},{n:'Saudi Arabia',lat:23.9,lng:45.1},
  {n:'Turkey',lat:38.9,lng:35.2},{n:'South Africa',lat:-30.6,lng:22.9},{n:'Nigeria',lat:9.1,lng:8.7},
  {n:'Egypt',lat:26.8,lng:30.8},{n:'Spain',lat:40.5,lng:-3.7},{n:'Italy',lat:41.9,lng:12.6},
  {n:'Ukraine',lat:48.4,lng:31.2},{n:'Poland',lat:51.9,lng:19.1},{n:'Kazakhstan',lat:48.0,lng:66.9},
  {n:'Sweden',lat:60.1,lng:18.6},{n:'Norway',lat:60.5,lng:8.5},{n:'Netherlands',lat:52.1,lng:5.3},
  {n:'South Korea',lat:35.9,lng:127.8},{n:'Vietnam',lat:14.1,lng:108.3},{n:'Thailand',lat:15.9,lng:100.9},
  {n:'Pakistan',lat:30.4,lng:69.3},{n:'Iran',lat:32.4,lng:53.7},{n:'Colombia',lat:4.6,lng:-74.3},
  {n:'Chile',lat:-35.7,lng:-71.5},{n:'Algeria',lat:28.0,lng:1.7},{n:'Morocco',lat:31.8,lng:-7.1},
  {n:'Kenya',lat:0.0,lng:37.9},{n:'Ethiopia',lat:9.1,lng:40.5},{n:'New Zealand',lat:-40.9,lng:174.9},
  {n:'Philippines',lat:12.9,lng:121.8},{n:'Malaysia',lat:4.2,lng:108.0},{n:'Peru',lat:-9.2,lng:-75.0},
]

const wrap = document.getElementById('globe-wrap')
const globe = Globe()
  .globeImageUrl('//unpkg.com/three-globe/example/img/earth-night.jpg')
  .backgroundImageUrl('//unpkg.com/three-globe/example/img/night-sky.png')
  .arcColor('color')
  .arcDashLength(0.4)
  .arcDashGap(0.2)
  .arcDashAnimateTime(1500)
  .arcStroke(0.5)
  .arcAltitude(0.3)
  .labelsData(countries)
  .labelLat('lat')
  .labelLng('lng')
  .labelText('n')
  .labelColor(() => 'rgba(255,255,255,0.3)')
  .labelSize(0.5)
  .labelDotRadius(0)
  .labelResolution(2)
  (document.getElementById('globe'))

globe.controls().autoRotate = true
globe.controls().autoRotateSpeed = 0.4

new ResizeObserver(() => {
  globe.width(wrap.clientWidth).height(wrap.clientHeight)
}).observe(wrap)
globe.width(wrap.clientWidth).height(wrap.clientHeight)

let arcs = []
function addArc(conn) {
  if (!conn.src_lat && !conn.dst_lat) return
  const id = Date.now() + Math.random()
  const color = conn.route === 'upstream' ? '#3b82f6' : '#22c55e'
  arcs = [...arcs, {id, startLat: conn.src_lat, startLng: conn.src_lng, endLat: conn.dst_lat, endLng: conn.dst_lng, color}]
  globe.arcsData(arcs)
  setTimeout(() => {
    arcs = arcs.filter(a => a.id !== id)
    globe.arcsData(arcs)
  }, 4000)
}

// Connection log
let allConns = []
let filter = 'all'

function setFilter(f, btn) {
  filter = f
  document.querySelectorAll('.fbtn').forEach(b => b.classList.remove('active'))
  btn.classList.add('active')
  renderConns()
}

function renderConns() {
  const list = document.getElementById('conn-list')
  const empty = document.getElementById('empty-msg')
  const visible = filter === 'all' ? allConns : allConns.filter(c => c.route === filter)
  if (visible.length === 0) {
    empty.style.display = 'block'
    list.querySelectorAll('.conn-item').forEach(e => e.remove())
    return
  }
  empty.style.display = 'none'
  list.querySelectorAll('.conn-item').forEach(e => e.remove())
  visible.slice().reverse().forEach(c => {
    const el = document.createElement('div')
    el.className = 'conn-item'
    const t = new Date(c.time)
    const ts = t.getHours().toString().padStart(2,'0') + ':' + t.getMinutes().toString().padStart(2,'0') + ':' + t.getSeconds().toString().padStart(2,'0')
    el.innerHTML = '<span class="badge badge-' + c.route + '">' + c.route + '</span>'
      + '<span class="conn-domain" title="' + c.domain + '">' + c.domain + '</span>'
      + '<span class="conn-meta">' + (c.country || '') + '<br>' + ts + '</span>'
    list.appendChild(el)
  })
}

function addConn(c) {
  allConns.push(c)
  if (allConns.length > 200) allConns = allConns.slice(-200)
  renderConns()
}

// load recent connections on page load
fetch('/api/connections').then(r => r.json()).then(data => {
  if (Array.isArray(data)) { allConns = data; renderConns() }
})

// SSE
const es = new EventSource('/events')
const dot = document.getElementById('dot')
es.addEventListener('metrics', e => {
  const d = JSON.parse(e.data)
  document.getElementById('s-active').textContent = d.active_conns
  document.getElementById('s-total').textContent = d.total_conns
  document.getElementById('s-direct').textContent = d.direct_conns
  document.getElementById('s-upstream').textContent = d.upstream_conns
  document.getElementById('s-errors').textContent = d.errors
})
es.addEventListener('connection', e => {
  const c = JSON.parse(e.data)
  addConn(c)
  addArc(c)
})
es.onopen = () => dot.style.background = '#22c55e'
es.onerror = () => dot.style.background = '#ef4444'

// Settings
function openSettings() {
  loadConfig()
  document.getElementById('settings-panel').classList.add('open')
  document.getElementById('overlay').classList.add('open')
}
function closeSettings() {
  document.getElementById('settings-panel').classList.remove('open')
  document.getElementById('overlay').classList.remove('open')
}

function loadConfig() {
  fetch('/api/config').then(r => r.json()).then(cfg => {
    document.getElementById('upstream-input').value = cfg.upstream || ''
    renderRules(cfg.rules || [])
  })
}

function renderRules(rules) {
  const el = document.getElementById('rules-list')
  el.innerHTML = ''
  if (!rules.length) { el.innerHTML = '<div style="color:#475569;font-size:12px">No rules configured</div>'; return }
  rules.forEach(r => {
    const div = document.createElement('div')
    div.className = 'rule-item'
    div.innerHTML = '<span class="rule-domain" title="' + r.domain + '">' + r.domain + '</span>'
      + '<span class="badge badge-' + r.route + '">' + r.route + '</span>'
      + '<button class="del-btn" onclick="deleteRule(\'' + r.domain + '\')">&#x2715;</button>'
    el.appendChild(div)
  })
}

function saveUpstream() {
  const val = document.getElementById('upstream-input').value.trim()
  alert('Restart the proxy with upstream="' + val + '" in config.yaml to apply.')
}

function addRule() {
  const domain = document.getElementById('rule-domain').value.trim()
  const route = document.getElementById('rule-route').value
  if (!domain) return
  fetch('/api/rules', {
    method: 'POST',
    headers: {'Content-Type':'application/json'},
    body: JSON.stringify({domain, route})
  }).then(r => {
    if (r.ok) {
      document.getElementById('rule-domain').value = ''
      loadConfig()
    }
  })
}

function deleteRule(domain) {
  fetch('/api/rules?domain=' + encodeURIComponent(domain), {method:'DELETE'}).then(r => {
    if (r.ok) loadConfig()
  })
}
</script>
</body>
</html>`
