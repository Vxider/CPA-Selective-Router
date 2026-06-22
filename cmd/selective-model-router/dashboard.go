package main

// dashboardHTML returns the self-contained route log + statistics dashboard page.
// Served as a resource at /v0/resource/plugins/selective-router/dashboard.
// It fetches relative "api/stats" and "api/logs" (sibling resource routes).
func dashboardHTML() string {
	return dashboardPage
}

const dashboardPage = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Selective Router - 路由日志</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
:root{
  --bg:#0a0e14;
  --bg-2:#0f1620;
  --panel:#111b27;
  --panel-2:#0d141d;
  --border:#1e2c3c;
  --border-2:#27384c;
  --text:#e6edf3;
  --muted:#7d8a9a;
  --dim:#4f5b6b;
  --accent:#2dd4bf;
  --accent-2:#22d3ee;
  --amber:#fbbf24;
  --rose:#fb7185;
  --violet:#a78bfa;
  --green:#34d399;
  --blue:#60a5fa;
  --pink:#f472b6;
  --orange:#fb923c;
}
*{box-sizing:border-box}
html,body{margin:0;padding:0}
body{
  background:
    radial-gradient(1200px 600px at 85% -10%, rgba(45,212,191,.10), transparent 60%),
    radial-gradient(900px 500px at 0% 0%, rgba(34,211,238,.08), transparent 55%),
    linear-gradient(180deg, var(--bg) 0%, var(--bg-2) 100%);
  background-attachment: fixed;
  color:var(--text);
  font-family:"Space Grotesk", system-ui, sans-serif;
  min-height:100vh;
  -webkit-font-smoothing:antialiased;
}
.wrap{max-width:1240px;margin:0 auto;padding:32px 24px 64px}
header.head{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;margin-bottom:28px}
.brand{display:flex;align-items:center;gap:14px}
.logo{
  width:46px;height:46px;border-radius:13px;
  background:conic-gradient(from 210deg, var(--accent), var(--accent-2), var(--violet), var(--accent));
  display:grid;place-items:center;
  box-shadow:0 8px 30px rgba(45,212,191,.25), inset 0 0 0 1px rgba(255,255,255,.08);
}
.logo svg{width:24px;height:24px}
.brand h1{font-size:22px;font-weight:700;letter-spacing:-.01em;margin:0;line-height:1.1}
.brand p{margin:2px 0 0;font-size:12.5px;color:var(--muted);font-family:"JetBrains Mono",monospace}
.head-meta{display:flex;align-items:center;gap:10px}
.live{
  display:inline-flex;align-items:center;gap:8px;
  font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--muted);
  padding:8px 12px;border:1px solid var(--border);border-radius:999px;background:rgba(255,255,255,.02)
}
.dot{width:8px;height:8px;border-radius:50%;background:var(--green);box-shadow:0 0 0 0 rgba(52,211,153,.7);animation:pulse 1.8s infinite}
@keyframes pulse{0%{box-shadow:0 0 0 0 rgba(52,211,153,.5)}70%{box-shadow:0 0 0 8px rgba(52,211,153,0)}100%{box-shadow:0 0 0 0 rgba(52,211,153,0)}}
.btn{
  font-family:"JetBrains Mono",monospace;font-size:12px;font-weight:500;
  color:var(--text);background:rgba(255,255,255,.04);
  border:1px solid var(--border-2);border-radius:10px;padding:8px 14px;cursor:pointer;
  transition:.15s ease;
}
.btn:hover{border-color:var(--accent);color:var(--accent);background:rgba(45,212,191,.06)}
.btn:active{transform:translateY(1px)}

.cards{display:grid;grid-template-columns:repeat(3,1fr);gap:14px;margin-bottom:24px}
.card{
  position:relative;overflow:hidden;
  background:linear-gradient(180deg, var(--panel), var(--panel-2));
  border:1px solid var(--border);border-radius:16px;padding:18px 18px 20px;
}
.card::after{content:"";position:absolute;inset:0 0 auto 0;height:2px;opacity:.9}
.card.c1::after{background:linear-gradient(90deg,var(--accent),transparent)}
.card.c2::after{background:linear-gradient(90deg,var(--amber),transparent)}
.card.c3::after{background:linear-gradient(90deg,var(--violet),transparent)}
.card .k{font-size:12px;color:var(--muted);letter-spacing:.04em;text-transform:uppercase;font-weight:500}
.card .v{font-size:34px;font-weight:700;line-height:1.1;margin-top:8px;font-family:"JetBrains Mono",monospace;letter-spacing:-.02em}
.card .sub{font-size:11.5px;color:var(--dim);margin-top:6px;font-family:"JetBrains Mono",monospace}

.grid{display:grid;grid-template-columns:1.05fr 1fr;gap:18px;margin-bottom:22px}
.panel{
  background:linear-gradient(180deg, var(--panel), var(--panel-2));
  border:1px solid var(--border);border-radius:18px;padding:20px 20px 22px;
}
.panel h2{margin:0 0 4px;font-size:15px;font-weight:600;letter-spacing:-.01em}
.panel .ph{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:18px}
.panel .ph .hint{font-size:11.5px;color:var(--dim);font-family:"JetBrains Mono",monospace}

.bars{display:flex;flex-direction:column;gap:12px}
.bar-row{display:grid;grid-template-columns:120px 1fr 52px;align-items:center;gap:12px;font-size:12.5px}
.bar-row .name{color:var(--text);font-family:"JetBrains Mono",monospace;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.bar-track{height:16px;background:rgba(255,255,255,.04);border-radius:8px;overflow:hidden;border:1px solid var(--border)}
.bar-fill{height:100%;border-radius:8px;transition:width .5s cubic-bezier(.22,1,.36,1);position:relative}
.bar-row .cnt{text-align:right;font-family:"JetBrains Mono",monospace;color:var(--muted);font-weight:500}
.route-meter{display:flex;align-items:center;gap:12px;margin:4px 0 14px;flex-wrap:wrap}
.route-meter .score{font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--muted);white-space:nowrap}
.route-meter .score strong{color:var(--green);font-weight:600}
.route-meter .score .route-count{color:var(--accent-2);font-weight:600}
.route-cells{display:flex;gap:3px;min-width:0;flex:1;align-items:center;flex-wrap:nowrap}
.route-cell{flex:1 1 0;min-width:6px;height:22px;border-radius:4px;background:#1a2433;box-shadow:inset 0 0 0 1px rgba(255,255,255,.04);cursor:default;transition:transform .12s ease,box-shadow .12s ease;position:relative}
.route-cell.empty{background:#2a3548;opacity:1}
.route-cell:hover{transform:scaleY(1.15);box-shadow:inset 0 0 0 1px rgba(255,255,255,.12),0 0 8px rgba(255,255,255,.06);z-index:2}
.route-legend{display:flex;flex-wrap:wrap;gap:10px 14px;margin-top:14px}
.legend-item{display:inline-flex;align-items:center;gap:7px;font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--muted)}
.legend-dot{width:8px;height:8px;border-radius:50%;box-shadow:0 0 0 2px rgba(255,255,255,.04)}
.legend-item strong{color:var(--text);font-weight:600}
.route-note{font-family:"JetBrains Mono",monospace;font-size:11.5px;color:var(--dim);margin-top:12px}
.cell-tip{position:fixed;z-index:9999;pointer-events:none;background:#06090e;border:1px solid var(--border-2);border-radius:10px;padding:10px 12px;font-family:"JetBrains Mono",monospace;font-size:11.5px;color:var(--text);box-shadow:0 12px 36px rgba(0,0,0,.5);max-width:240px;line-height:1.6}
.cell-tip .tip-time{color:var(--muted);margin-bottom:6px}
.cell-tip .tip-row{display:flex;align-items:center;gap:7px;justify-content:space-between}
.cell-tip .tip-row .tl{display:inline-flex;align-items:center;gap:6px}
.cell-tip .tip-row .td{width:6px;height:6px;border-radius:50%}
.cell-tip .tip-total{color:var(--accent);font-weight:600;margin-top:6px;padding-top:6px;border-top:1px solid var(--border)}
.cell-tip-portal{position:fixed;z-index:9999;pointer-events:none;background:#06090e;border:1px solid var(--border-2);border-radius:10px;padding:10px 12px;font-family:"JetBrains Mono",monospace;font-size:11.5px;color:var(--text);box-shadow:0 12px 36px rgba(0,0,0,.5);max-width:240px;line-height:1.6;display:none}

.breakdown{display:grid;grid-template-columns:1fr 1fr;gap:22px;margin-top:6px}
.bd h3{margin:0 0 12px;font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.05em;font-weight:600}
.bd ul{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:9px}
.bd li{display:flex;align-items:center;justify-content:space-between;gap:10px;font-size:13px;font-family:"JetBrains Mono",monospace}
.bd li .lab{color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.bd li .num{color:var(--muted)}

.badge{font-family:"JetBrains Mono",monospace;font-size:10.5px;font-weight:600;padding:3px 8px;border-radius:6px;letter-spacing:.02em;white-space:nowrap}
.badge .d{display:inline-block;width:6px;height:6px;border-radius:50%;margin-right:6px;vertical-align:middle}

.log-head{display:flex;gap:10px;align-items:center;justify-content:space-between;margin-bottom:14px;flex-wrap:wrap}
.search{flex:1;min-width:160px;font-family:"JetBrains Mono",monospace;font-size:12.5px;color:var(--text);background:rgba(0,0,0,.25);border:1px solid var(--border-2);border-radius:10px;padding:9px 12px;outline:none}
.search:focus{border-color:var(--accent);box-shadow:0 0 0 3px rgba(45,212,191,.12)}
.filterpills{display:flex;gap:7px;flex-wrap:wrap}
.pill{font-family:"JetBrains Mono",monospace;font-size:11px;color:var(--muted);padding:6px 10px;border:1px solid var(--border);border-radius:999px;cursor:pointer;transition:.15s}
.pill.active{color:var(--bg);background:var(--accent);border-color:var(--accent);font-weight:600}
.pill:not(.active):hover{border-color:var(--border-2);color:var(--text)}

table{width:100%;border-collapse:collapse;font-family:"JetBrains Mono",monospace;font-size:12px}
thead th{
  text-align:left;font-weight:500;color:var(--muted);font-size:10.5px;text-transform:uppercase;letter-spacing:.06em;
  padding:10px 10px 12px;border-bottom:1px solid var(--border);position:sticky;top:0;background:var(--panel-2)
}
thead th:first-child{padding-left:16px}
tbody td{padding:11px 10px;border-bottom:1px solid rgba(255,255,255,.04);color:var(--text);vertical-align:top}
tbody td:first-child{padding-left:16px;color:var(--muted);white-space:nowrap}
tbody tr:hover{background:rgba(255,255,255,.02)}
.scroll{max-height:560px;overflow:auto}
.mono-empty{color:var(--dim);text-align:center;padding:48px 0;font-family:"JetBrains Mono",monospace;font-size:13px}
.muted2{color:var(--dim)}
.foot{margin-top:26px;text-align:center;color:var(--dim);font-size:11.5px;font-family:"JetBrains Mono",monospace}
.foot a{color:var(--muted);text-decoration:none}

@media(max-width:900px){
  .cards{grid-template-columns:repeat(2,1fr)}
  .grid{grid-template-columns:1fr}
  .breakdown{grid-template-columns:1fr}
  .bar-row{grid-template-columns:96px 1fr 44px}
}
.fade-in{animation:fade .4s ease both}
@keyframes fade{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:none}}
</style>
</head>
<body>
<div class="wrap">
  <header class="head fade-in">
    <div class="brand">
      <div class="logo">
        <svg viewBox="0 0 24 24" fill="none" stroke="#06121a" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 7h16"/><path d="M4 12h10"/><path d="M4 17h6"/><circle cx="18" cy="16" r="2.4"/>
        </svg>
      </div>
      <div>
        <h1>Selective Router</h1>
        <p>route triggers &middot; live log</p>
      </div>
    </div>
    <div class="head-meta">
      <span class="live"><span class="dot"></span><span id="liveStat">connecting</span></span>
      <button class="btn" id="refreshBtn">刷新</button>
      <button class="btn" id="clearBtn">清空日志</button>
    </div>
  </header>

  <section class="grid fade-in">
    <div class="panel">
      <div class="ph"><div><h2>路由统计</h2></div><span class="hint" id="catHint">recent requests</span></div>
      <div id="routeProgress"><div class="mono-empty">暂无数据</div></div>
    </div>
    <div class="panel">
      <div class="ph"><div><h2>目标与模型</h2></div><span class="hint">by requested model</span></div>
      <div class="breakdown">
        <div class="bd">
          <h3>请求模型 Top</h3>
          <ul id="modelList"><li class="mono-empty">-</li></ul>
        </div>
      </div>
    </div>
  </section>

  <section class="panel fade-in" style="margin-bottom:0">
    <div class="log-head">
      <div><h2 style="margin-bottom:2px">路由触发日志</h2></div>
      <div style="display:flex;gap:10px;flex:1;justify-content:flex-end;flex-wrap:wrap;align-items:center">
        <input class="search" id="search" placeholder="筛选 model / provider / reason / category..." />
        <div class="filterpills" id="pills"></div>
      </div>
    </div>
    <div class="scroll">
      <table>
        <thead><tr>
          <th>时间</th><th>阶段</th><th>模型</th><th>分类</th><th>原因</th><th>目标</th><th>流</th>
        </tr></thead>
        <tbody id="logBody"><tr><td colspan="7" class="mono-empty">等待数据...</td></tr></tbody>
      </table>
    </div>
  </section>

  <div class="foot">selective-router dashboard &middot; in-memory ring buffer (capacity 500) &middot; restart clears logs</div>
</div>

<script>
var CAT_COLORS={
  normal:"#34d399",compact:"#2dd4bf",auto_review:"#a78bfa",web_search:"#38bdf8",vision:"#fbbf24",
  image_generation:"#f472b6",disabled:"#4f5b6b" ,route_provider_unavailable:"#fb7185"
};
var CAT_LABEL={
  normal:"direct",compact:"Compact",auto_review:"Auto Review",web_search:"WebSearch",vision:"Visual",
  image_generation:"Image Gen",disabled:"已禁用",route_provider_unavailable:"Provider 不可用"
};
var CAT_ORDER=["normal","web_search","vision","image_generation","auto_review","compact","route_provider_unavailable","disabled"];
var lastStats=null;
var lastEvents=[];
var activeFilter="all";

function rel(url){
  var base=location.pathname.replace(/\/dashboard\/?$/,"/");
  return base+url;
}
function fmtTime(t){
  if(!t) return "-";
  var d=new Date(t);
  if(isNaN(d)) return t;
  var p=function(n){return (n<10?"0":"")+n};
  return p(d.getMonth()+1)+"-"+p(d.getDate())+" "+p(d.getHours())+":"+p(d.getMinutes())+":"+p(d.getSeconds());
}
function fmtClock(t){
  if(!t) return "-";
  var d=new Date(t); if(isNaN(d)) return "-";
  var p=function(n){return (n<10?"0":"")+n};
  return p(d.getHours())+":"+p(d.getMinutes())+":"+p(d.getSeconds());
}
function esc(s){
  if(s===undefined||s===null) return "";
  return String(s).replace(/[&<>"]/g,function(c){return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;"}[c]});
}
function badge(cat){
  var color=CAT_COLORS[cat]||"#7d8a9a";
  var label=CAT_LABEL[cat]||cat||"-";
  return '<span class="badge" style="background:'+color+"22;color:"+color+';border:1px solid '+color+"44\"><span class=\"d\" style=\"background:"+color+'"></span>'+esc(label)+"</span>";
}
async function fetchJSON(url){
  var r=await fetch(url,{cache:"no-store"});
  if(!r.ok) throw new Error(url+" "+r.status);
  return r.json();
}
function sum(m){var t=0;for(var k in m){t+=m[k]}return t}
function topEntries(m,n){
  var arr=[];for(var k in m){arr.push([k,m[k]])}
  arr.sort(function(a,b){return b[1]-a[1]});
  return arr.slice(0,n);
}
function orderedCategories(m){
  var seen={}, arr=[];
  for(var i=0;i<CAT_ORDER.length;i++){var k=CAT_ORDER[i];if(m[k]){arr.push([k,m[k]]);seen[k]=true}}
  var rest=[];for(var k in m){if(!seen[k])rest.push([k,m[k]])}
  rest.sort(function(a,b){return b[1]-a[1]});
  return arr.concat(rest);
}
function hexToRgb(h){h=h.replace("#","");if(h.length===3)h=h[0]+h[0]+h[1]+h[1]+h[2]+h[2];return [parseInt(h.slice(0,2),16),parseInt(h.slice(2,4),16),parseInt(h.slice(4,6),16)]}
function rgbToHex(r,g,b){function p(n){n=Math.max(0,Math.min(255,Math.round(n)));return (n<16?"0":"")+n.toString(16)}return "#"+p(r)+p(g)+p(b)}
function blendColors(parts){
  var tr=0,tg=0,tb=0,w=0;
  for(var i=0;i<parts.length;i++){var c=hexToRgb(parts[i].color);var n=parts[i].weight;tr+=c[0]*n;tg+=c[1]*n;tb+=c[2]*n;w+=n}
  if(!w) return "#2a3548";
  return rgbToHex(tr/w,tg/w,tb/w);
}
function bucketColor(bucket){
  var parts=[];
  for(var cat in bucket.by_category){parts.push({color:CAT_COLORS[cat]||"#7d8a9a",weight:bucket.by_category[cat]})}
  return blendColors(parts);
}
function fmtRange(start,end){
  function p(n){return (n<10?"0":"")+n}
  var a=new Date(start),b=new Date(end);
  return p(a.getHours())+":"+p(a.getMinutes())+"~"+p(b.getHours())+":"+p(b.getMinutes());
}
function bucketTipHTML(bucket){
  var rows=[];
  var cats=orderedCategories(bucket.by_category);
  for(var i=0;i<cats.length;i++){
    var cat=cats[i][0],n=cats[i][1],color=CAT_COLORS[cat]||"#7d8a9a";
    rows.push('<div class="tip-row"><span class="tl"><span class="td" style="background:'+color+'"></span>'+esc(CAT_LABEL[cat]||cat)+'</span><span>'+n+'</span></div>');
  }
  if(!bucket.total){
    return '<div class="tip-time">'+fmtRange(bucket.start,bucket.end)+'</div><div class="tip-row" style="color:var(--dim);justify-content:center">无请求</div>';
  }
  return '<div class="tip-time">'+fmtRange(bucket.start,bucket.end)+'</div>'+rows.join("")+'<div class="tip-total">合计 '+bucket.total+'</div>';
}
function renderBuckets(buckets){
  if(!buckets||!buckets.length) return '<span class="mono-empty">暂无数据</span>';
  var h='<div class="route-cells">';
  for(var i=0;i<buckets.length;i++){
    var b=buckets[i],color=b.total?bucketColor(b):"",cls="route-cell"+(b.total?"":" empty");
    h+='<span class="'+cls+'"'+(b.total?' style="background:'+color+'"':'')+'>';
    h+='<div class="cell-tip" style="display:none">'+bucketTipHTML(b)+'</div>';
    h+='</span>';
  }
  h+='</div>';
  return h;
}
var cellTip=null;
function attachCellHover(container){
  var cells=container.querySelectorAll(".route-cell");
  for(var i=0;i<cells.length;i++){
    cells[i].addEventListener("mouseenter",function(e){
      var tip=this.querySelector(".cell-tip"); if(!tip) return;
      if(!cellTip){cellTip=document.createElement("div");cellTip.className="cell-tip-portal";document.body.appendChild(cellTip)}
      cellTip.innerHTML=tip.innerHTML;cellTip.style.display="block";
      moveTip(e);
    });
    cells[i].addEventListener("mousemove",moveTip);
    cells[i].addEventListener("mouseleave",function(){if(cellTip)cellTip.style.display="none"});
  }
}
function moveTip(e){
  if(!cellTip||cellTip.style.display==="none") return;
  var x=e.clientX+14,y=e.clientY+14;
  var r=cellTip.getBoundingClientRect();
  if(x+r.width>window.innerWidth-10) x=e.clientX-r.width-14;
  if(y+r.height>window.innerHeight-10) y=e.clientY-r.height-14;
  cellTip.style.left=x+"px";cellTip.style.top=y+"px";
}
function renderStats(s){
  lastStats=s;
  var total=s.total_routes||0;
  var handled=s.handled_routes||0;
  var normal=(s.by_category&&s.by_category.normal)||0;
  var rate=total?Math.round(handled/total*100):0;

  var progress=document.getElementById("routeProgress");
  var byCat=s.by_category||{};
  var cats=orderedCategories(byCat);
  if(!total){progress.innerHTML='<div class="mono-empty">暂无路由记录</div>'}
  else{
    var h='<div class="route-meter">'
      +renderBuckets(s.buckets)
      +'</div><div class="route-legend">';
    for(var i=0;i<cats.length;i++){
      var cat=cats[i][0], count=cats[i][1], color=CAT_COLORS[cat]||"#7d8a9a";
      h+='<span class="legend-item"><span class="legend-dot" style="background:'+color+'"></span><span>'+esc(CAT_LABEL[cat]||cat)+'</span><strong>'+count+'</strong></span>';
    }
    h+='</div>';
    progress.innerHTML=h;
    attachCellHover(progress);
  }
  document.getElementById("catHint").textContent=sum(s.by_category)+" categorized";

  function list(id,m){var el=document.getElementById(id);var t=topEntries(m,8);if(!t.length){el.innerHTML='<li class="mono-empty">-</li>';return}el.innerHTML=t.map(function(e){return '<li><span class="lab" title="'+esc(e[0])+'">'+esc(e[0])+'</span><span class="num">'+e[1]+'</span></li>'}).join("")}
  list("modelList",s.by_requested_model||{});
  document.getElementById("liveStat").textContent="updated "+fmtClock(s.generated_at);
}
function renderPills(events){
  var cats={};for(var i=0;i<events.length;i++){var c=events[i].category||"";if(c)cats[c]=(cats[c]||0)+1}
  var arr=Object.keys(cats).sort(function(a,b){return cats[b]-cats[a]});
  var pills=document.getElementById("pills");
  var h='<span class="pill'+(activeFilter==="all"?" active":"")+'" data-cat="all">全部 '+events.length+'</span>';
  for(var i=0;i<arr.length;i++){var c=arr[i];h+='<span class="pill'+(activeFilter===c?" active":"")+'" data-cat="'+esc(c)+'">'+esc(CAT_LABEL[c]||c)+' '+cats[c]+'</span>'}
  pills.innerHTML=h;
  var nodes=pills.querySelectorAll(".pill");
  for(var i=0;i<nodes.length;i++){nodes[i].onclick=function(){activeFilter=this.getAttribute("data-cat");renderLog(lastEvents,lastStats)}}
}
function renderLog(events,stats){
  lastEvents=events||[];
  renderPills(lastEvents);
  var q=(document.getElementById("search").value||"").toLowerCase().trim();
  var rows=[];
  for(var i=0;i<lastEvents.length;i++){
    var ev=lastEvents[i];
    if(activeFilter!=="all"&&(ev.category||"")!==activeFilter) continue;
    var hay=[ev.requested_model,ev.category,ev.reason,ev.target_provider,ev.target_model,ev.phase].join(" ").toLowerCase();
    if(q&&hay.indexOf(q)<0) continue;
    rows.push(ev);
  }
  var body=document.getElementById("logBody");
  if(!rows.length){body.innerHTML='<tr><td colspan="7" class="mono-empty">'+(lastEvents.length?"无匹配记录":"等待数据...")+'</td></tr>';return}
  var limit=500;var h="";
  if(rows.length>limit)rows=rows.slice(0,limit);
  for(var i=0;i<rows.length;i++){
    var ev=rows[i];
    h+='<tr>'
      +'<td>'+esc(fmtTime(ev.time))+'</td>'
      +'<td>'+(ev.phase==="route"?"<span style='color:var(--accent)'>route</span>":"<span style='color:var(--amber)'>norm</span>")+'</td>'
      +'<td>'+esc(ev.requested_model||"-")+'</td>'
      +'<td>'+badge(ev.category)+'</td>'
      +'<td class="muted2">'+esc(ev.reason||"-")+'</td>'
      +'<td>'+(ev.target_provider?esc(ev.target_provider)+"/"+esc(ev.target_model||"-"):'<span class="muted2">-</span>')+'</td>'
      +'<td>'+(ev.stream?'<span style="color:var(--accent-2)">●</span>':'<span class="muted2">○</span>')+'</td>'
      +'</tr>';
  }
  body.innerHTML=h;
}
async function load(){
  try{
    var s=await fetchJSON(rel("api/stats"));
    var l=await fetchJSON(rel("api/logs"));
    renderStats(s);
    renderLog(l.events||[],s);
  }catch(e){
    document.getElementById("liveStat").textContent="error";
    console.error(e);
  }
}
document.getElementById("search").addEventListener("input",function(){renderLog(lastEvents,lastStats)});
document.getElementById("refreshBtn").addEventListener("click",load);
document.getElementById("clearBtn").addEventListener("click",async function(){
  if(!confirm("确认清空路由日志缓冲?")) return;
  try{
    await fetch(rel("api/clear?confirm=1"),{cache:"no-store"});
    await load();
  }catch(e){alert("清空失败: "+e.message)}
});
load();
</script>
</body>
</html>`
