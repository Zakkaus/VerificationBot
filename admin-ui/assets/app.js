// ── Config ──────────────────────────────────────────────────────────────────
const API = '/admin/api';
let token = localStorage.getItem('token') || '';
let role  = localStorage.getItem('role')  || '';
let currentPage = 'dashboard';
let logsOffset = 0;
const LIMIT = 50;

// ── Helpers ──────────────────────────────────────────────────────────────────
function esc(s){ const d=document.createElement('div'); d.textContent=s||''; return d.innerHTML }
function $(id){ return document.getElementById(id) }
function ago(ts){
  if(!ts) return '—';
  const d = new Date(ts.replace ? ts.replace(' ','T')+'Z' : ts);
  const s = Math.floor((Date.now()-d)/1000);
  if(s<60) return s+'s ago';
  if(s<3600) return Math.floor(s/60)+'m ago';
  if(s<86400) return Math.floor(s/3600)+'h ago';
  return Math.floor(s/86400)+'d ago';
}
function resultBadge(r){
  const map={pass:'pass',fail:'fail',timeout:'timeout',declined:'declined',pending:'pending'};
  const cls = map[r]||'pending';
  return `<span class="badge badge-${cls}">${r||'?'}</span>`;
}

// ── Auth helpers ──────────────────────────────────────────────────────────────
async function apiFetch(path, opts={}){
  const headers = {'Content-Type':'application/json', ...(opts.headers||{})};
  if(token) headers['Authorization'] = 'Bearer '+token;
  const res = await fetch(API+path, {...opts, headers});
  if(res.status === 401){
    // Try refresh
    const ref = await fetch(API+'/auth/refresh',{method:'POST',credentials:'include'});
    if(ref.ok){
      const d = await ref.json();
      token = d.token;
      localStorage.setItem('token',token);
      headers['Authorization'] = 'Bearer '+token;
      return fetch(API+path, {...opts, headers});
    }
    doLogout();
    return null;
  }
  return res;
}

function doLogout(){
  localStorage.removeItem('token');
  localStorage.removeItem('role');
  window.location='/admin/login';
}

async function logout(){
  await apiFetch('/auth/logout',{method:'POST'});
  doLogout();
}

// ── Init ──────────────────────────────────────────────────────────────────────
async function init(){
  // 1. Telegram Mini App auto-login
  const tg = window.Telegram?.WebApp;
  if(tg && tg.initData && !token){
    tg.ready(); tg.expand();
    try{
      const res = await fetch(API+'/auth/telegram',{
        method:'POST', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({init_data: tg.initData})
      });
      if(res.ok){
        const d = await res.json();
        token = d.token; role = d.role;
        localStorage.setItem('token',token);
        localStorage.setItem('role',role);
      } else {
        const err = await res.json();
        $('content').innerHTML = `<div style="text-align:center;padding:60px 20px">
          <div style="font-size:2.5rem;margin-bottom:12px">🚫</div>
          <div style="font-size:1rem;color:#e4e4e7;margin-bottom:8px">${esc(err.error||'驗證失敗')}</div>
          <div style="font-size:.82rem;color:#71717a">如需直接登入：<a href="/admin/login" style="color:#2563eb">登入頁面</a></div>
        </div>`;
        return;
      }
    } catch(e){ /* not in Mini App */ }
  }

  // 2. Redirect to login if no token
  if(!token){ window.location='/admin/login'; return; }

  // 3. Load branding + user info
  try{
    const [meRes, settingsRes] = await Promise.all([
      apiFetch('/auth/me'),
      apiFetch('/settings')
    ]);
    if(!meRes){ return; } // 401 handled already
    const me = await meRes.json();
    role = me.role;
    localStorage.setItem('role', role);
    $('user-badge').textContent = me.username + ' · ' + role;

    if(settingsRes && settingsRes.ok){
      const s = await settingsRes.json();
      applyBranding(s);
    }
  } catch(e){
    console.error('init error', e);
  }

  // 4. Show/hide superadmin-only items
  if(role === 'superadmin'){
    document.querySelectorAll('.superadmin-only').forEach(el => el.style.display='');
  }

  // 5. Wire nav
  document.querySelectorAll('.nav-item').forEach(el => {
    el.addEventListener('click', e => {
      e.preventDefault();
      navigate(el.dataset.page);
    });
  });

  // 6. Mobile nav
  const mobileNav = buildMobileNav();
  document.body.appendChild(mobileNav);

  // 7. Show dashboard
  navigate('dashboard');
}

function applyBranding(s){
  if(s.site_name) $('brand-name').textContent = s.site_name;
  if(s.site_logo){
    const img = $('brand-logo');
    img.src = s.site_logo; img.style.display='block';
  }
  if(s.primary_color){
    document.documentElement.style.setProperty('--accent', s.primary_color);
  }
}

function buildMobileNav(){
  const pages = [
    {id:'dashboard', icon:'📊', label:'儀表板'},
    {id:'logs',      icon:'📋', label:'記錄'},
    {id:'settings',  icon:'⚙️',  label:'設定'},
    ...(role==='superadmin'?[{id:'users',icon:'👥',label:'帳號'}]:[]),
  ];
  const nav = document.createElement('div');
  nav.id = 'mobile-nav';
  nav.innerHTML = pages.map(p =>
    `<button class="mob-tab ${p.id===currentPage?'active':''}" data-page="${p.id}" onclick="navigate('${p.id}')">
      <span class="mob-icon">${p.icon}</span>${esc(p.label)}
    </button>`
  ).join('');
  return nav;
}

// ── Navigation ────────────────────────────────────────────────────────────────
function navigate(page){
  currentPage = page;
  logsOffset = 0;

  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page===page);
  });
  document.querySelectorAll('.mob-tab').forEach(el => {
    el.classList.toggle('active', el.dataset.page===page);
  });

  const content = $('content');
  content.innerHTML = '<div class="spinner">載入中…</div>';

  const pages = {
    dashboard: renderDashboard,
    logs:      renderLogs,
    settings:  renderSettings,
    users:     renderUsers,
  };
  if(pages[page]) pages[page](content);
}

// ── Dashboard ─────────────────────────────────────────────────────────────────
async function renderDashboard(el){
  const [sRes, biRes] = await Promise.all([
    apiFetch('/stats'),
    apiFetch('/bot-info'),
  ]);
  if(!sRes || !sRes.ok){ el.innerHTML='<div class="spinner">載入失敗</div>'; return; }
  const s = await sRes.json();
  const bi = (biRes && biRes.ok) ? await biRes.json() : null;

  const total = (s.pass||0)+(s.fail||0)+(s.timeout||0)+(s.declined||0);
  const passRate = total>0 ? Math.round((s.pass||0)/total*100) : 0;

  const roleIcon = {superadmin:'🔑',admin:'👤',none:'👁','':<span style="color:var(--muted)">—</span>};

  const groupsHTML = bi && bi.groups && bi.groups.length ? `
    <div class="page-title" style="font-size:1rem;margin-bottom:12px;margin-top:24px">🏘️ 監管群組</div>
    <div class="table-wrap"><table>
      <thead><tr><th>群組 ID</th><th>你的角色</th></tr></thead>
      <tbody>${bi.groups.map(g=>`<tr>
        <td><code style="font-size:.82rem;color:var(--muted)">${esc(g.group_id)}</code></td>
        <td>${g.role==='superadmin'?'🔑 群主':g.role==='admin'?'👤 管理員':'— 無'}</td>
      </tr>`).join('')}</tbody>
    </table></div>` : '';

  el.innerHTML = `
    <div class="page-title">儀表板</div>
    <div class="stat-grid">
      <div class="stat-card"><div class="stat-label">今日總計</div><div class="stat-value">${total}</div></div>
      <div class="stat-card"><div class="stat-label">通過</div><div class="stat-value" style="color:var(--green)">${s.pass||0}</div></div>
      <div class="stat-card"><div class="stat-label">超時</div><div class="stat-value" style="color:var(--yellow)">${s.timeout||0}</div></div>
      <div class="stat-card"><div class="stat-label">拒絕</div><div class="stat-value" style="color:var(--red)">${s.declined||0}</div></div>
      <div class="stat-card"><div class="stat-label">通過率</div><div class="stat-value">${passRate}%</div></div>
    </div>
    ${groupsHTML}
    <div class="page-title" style="font-size:1rem;margin-bottom:12px;margin-top:24px">最新記錄</div>
    <div id="recent-wrap"><div class="spinner">載入中…</div></div>`;

  const lr = await apiFetch('/logs?limit=10&offset=0');
  if(!lr || !lr.ok){ $('recent-wrap').innerHTML='<div class="spinner">載入失敗</div>'; return; }
  const {logs} = await lr.json();
  $('recent-wrap').innerHTML = logsTable(logs||[]);
}


// ── Logs ──────────────────────────────────────────────────────────────────────
async function renderLogs(el){
  logsOffset = 0;
  el.innerHTML = `
    <div class="page-title">操作記錄</div>
    <div class="filters">
      <input id="f-search" type="text" placeholder="🔍 搜尋用戶/事件" style="min-width:180px">
      <select id="f-result">
        <option value="">全部</option>
        <option value="pass">通過</option>
        <option value="timeout">超時</option>
        <option value="declined">拒絕</option>
        <option value="fail">失敗</option>
        <option value="pending">待驗</option>
      </select>
      <input id="f-uid"  type="text" placeholder="User ID"  style="width:110px">
      <input id="f-chat" type="text" placeholder="群組 ID"  style="width:110px">
      <input id="f-from" type="date">
      <input id="f-to"   type="date">
      <button class="btn btn-outline btn-sm" onclick="clearFilters()">清除</button>
      <button class="btn btn-outline btn-sm" onclick="exportCSV()">⬇ CSV</button>
    </div>
    <div id="logs-wrap"></div>
    <div id="logs-pg" class="pagination"></div>`;

  let t;
  document.getElementById('f-search').addEventListener('input',()=>{ clearTimeout(t); t=setTimeout(fetchLogs,350) });
  ['f-result','f-uid','f-chat','f-from','f-to'].forEach(id=>{
    const el = document.getElementById(id);
    if(el) el.addEventListener('change', fetchLogs);
  });
  await fetchLogs();
}

function clearFilters(){
  ['f-search','f-result','f-uid','f-chat','f-from','f-to'].forEach(id=>{
    const el = document.getElementById(id); if(el) el.value='';
  });
  logsOffset = 0; fetchLogs();
}

async function fetchLogs(){
  const wrap = $('logs-wrap');
  if(wrap) wrap.innerHTML = '<div class="spinner">載入中…</div>';
  const qs = new URLSearchParams({limit:LIMIT, offset:logsOffset});
  const add=(id,key)=>{ const v=document.getElementById(id)?.value; if(v) qs.set(key,v) };
  add('f-search','search'); add('f-result','result');
  add('f-uid','user_id');   add('f-chat','chat_id');
  add('f-from','date_from'); add('f-to','date_to');
  const res = await apiFetch('/logs?'+qs);
  if(!res||!res.ok){ if(wrap) wrap.innerHTML='<div class="spinner">載入失敗</div>'; return; }
  const {logs, total} = await res.json();
  if(wrap) wrap.innerHTML = logsTable(logs||[], true);
  renderPagination(total||0);
}

function renderPagination(total){
  const pg = $('logs-pg'); if(!pg) return;
  const pages = Math.ceil(total/LIMIT);
  const cur = Math.floor(logsOffset/LIMIT);
  pg.innerHTML = `
    <button class="page-btn" onclick="prevPage()" ${cur===0?'disabled':''}>‹ 上頁</button>
    <span class="page-info">${cur+1} / ${pages||1}</span>
    <button class="page-btn" onclick="nextPage()" ${cur>=pages-1?'disabled':''}>下頁 ›</button>`;
}
function prevPage(){ logsOffset=Math.max(0,logsOffset-LIMIT); fetchLogs() }
function nextPage(){ logsOffset+=LIMIT; fetchLogs() }

async function exportCSV(){
  const qs = new URLSearchParams({limit:9999,offset:0});
  const add=(id,key)=>{ const v=document.getElementById(id)?.value; if(v) qs.set(key,v) };
  add('f-search','search'); add('f-result','result');
  add('f-uid','user_id');   add('f-chat','chat_id');
  add('f-from','date_from'); add('f-to','date_to');
  const res = await apiFetch('/logs/export?'+qs);
  if(!res||!res.ok) return;
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a'); a.href=url; a.download='logs.csv'; a.click();
  URL.revokeObjectURL(url);
}

function logsTable(logs, showChat){
  if(!logs.length) return '<div class="spinner">無記錄</div>';
  return `<div class="table-wrap"><table>
    <thead><tr>
      <th>時間</th><th>用戶</th>${showChat?'<th>群組</th>':''}
      <th>事件</th><th>結果</th><th>詳情</th>
    </tr></thead>
    <tbody>${logs.map(l=>`<tr>
      <td style="color:var(--muted);white-space:nowrap">${ago(l.ts)}</td>
      <td>${esc(l.username||'?')}<br><span style="color:var(--muted);font-size:.75rem">${l.user_id||''}</span></td>
      ${showChat?`<td style="font-size:.78rem;color:var(--muted)">${esc(l.chat_title||l.chat_id||'')}</td>`:''}
      <td>${esc(l.event_type||'')}</td>
      <td>${resultBadge(l.result)}</td>
      <td style="color:var(--muted);max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(l.detail||'')}</td>
    </tr>`).join('')}</tbody>
  </table></div>`;
}

// ── Settings ──────────────────────────────────────────────────────────────────
async function renderSettings(el){
  const res = await apiFetch('/settings');
  const s = (res&&res.ok) ? await res.json() : {};
  const ro = !['superadmin','admin'].includes(role);
  const dis = ro ? 'disabled' : '';
  const ct = s.captcha_type||'recaptcha';

  el.innerHTML = `
    <div class="page-title">設定</div>
    <div id="s-alert"></div>

    <div class="form-section">
      <div class="form-section-title">🎨 外觀</div>
      <div class="field"><label>網站名稱</label>
        <input id="s-name" value="${esc(s.site_name||'')}" ${dis} placeholder="VerificationBot Admin"></div>
      <div class="field"><label>Logo URL</label>
        <input id="s-logo" value="${esc(s.site_logo||'')}" ${dis} placeholder="https://example.com/logo.png"></div>
      <div class="field"><label>主色</label>
        <div class="color-row">
          <input id="s-color" value="${esc(s.primary_color||'#2563eb')}" ${dis} style="flex:1">
          <div class="color-swatch" id="color-swatch" style="background:${esc(s.primary_color||'#2563eb')}"></div>
        </div>
      </div>
    </div>

    <div class="form-section">
      <div class="form-section-title">🛡️ 人機驗證</div>
      <div class="field"><label>類型</label>
        <select id="s-ct" ${dis}>
          <option value="turnstile" ${ct==='turnstile'?'selected':''}>Cloudflare Turnstile（推薦）</option>
          <option value="recaptcha" ${ct==='recaptcha'?'selected':''}>Google reCAPTCHA v2</option>
        </select></div>
      <div class="field"><label>Site Key（前端）</label>
        <input id="s-sk" value="${esc(s.captcha_site_key||'')}" ${dis} placeholder="0x4AAA... 或 6LeI..."></div>
      <div class="field"><label>Secret Key（後端）</label>
        <input id="s-sec" type="password" value="${esc(s.captcha_secret||'')}" ${dis} placeholder="••••••••"></div>
    </div>

    ${ro
      ? '<div style="color:var(--muted);font-size:.85rem">你的權限無法修改設定。</div>'
      : '<button class="btn" onclick="saveSettings()">💾 儲存設定</button>'
    }`;

  const colorInput = document.getElementById('s-color');
  if(colorInput) colorInput.addEventListener('input', ()=>{
    $('color-swatch').style.background = colorInput.value;
  });
}

async function saveSettings(){
  const body = {
    site_name:        $('s-name')?.value,
    site_logo:        $('s-logo')?.value,
    primary_color:    $('s-color')?.value,
    captcha_type:     $('s-ct')?.value,
    captcha_site_key: $('s-sk')?.value,
    captcha_secret:   $('s-sec')?.value,
  };
  const res = await apiFetch('/settings',{method:'PATCH',body:JSON.stringify(body)});
  const al = $('s-alert');
  if(res&&res.ok){
    al.innerHTML='<div class="alert alert-ok">✅ 儲存成功！設定立即生效。</div>';
    applyBranding(body);
  } else {
    al.innerHTML='<div class="alert alert-err">❌ 儲存失敗，請重試。</div>';
  }
  setTimeout(()=>{ if(al) al.innerHTML='' }, 4000);
}

// ── Users ─────────────────────────────────────────────────────────────────────
async function renderUsers(el){
  if(role!=='superadmin'){ el.innerHTML='<div class="spinner">無權限</div>'; return; }
  el.innerHTML = `
    <div class="page-title">帳號管理</div>
    <div id="u-alert"></div>
    <div class="form-section" style="margin-bottom:16px">
      <div class="form-section-title">新增管理員</div>
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        <input id="nu-user" placeholder="帳號" style="flex:1;min-width:100px;background:#09090b;border:1px solid var(--border);border-radius:8px;padding:9px 12px;color:var(--text)">
        <input id="nu-pass" type="password" placeholder="密碼" style="flex:1;min-width:100px;background:#09090b;border:1px solid var(--border);border-radius:8px;padding:9px 12px;color:var(--text)">
        <select id="nu-role" style="background:#09090b;border:1px solid var(--border);border-radius:8px;padding:9px 12px;color:var(--text)">
          <option value="viewer">viewer</option>
          <option value="admin">admin</option>
        </select>
        <button class="btn btn-sm" onclick="createUser()">新增</button>
      </div>
    </div>
    <div id="users-list"><div class="spinner">載入中…</div></div>`;
  await loadUsers();
}

async function loadUsers(){
  const res = await apiFetch('/users');
  if(!res||!res.ok){ $('users-list').innerHTML='<div class="spinner">載入失敗</div>'; return; }
  const users = await res.json();
  $('users-list').innerHTML = `<div class="table-wrap"><table>
    <thead><tr><th>ID</th><th>帳號</th><th>角色</th><th>Telegram ID</th><th>最後登入</th><th>操作</th></tr></thead>
    <tbody>${users.map(u=>`<tr>
      <td style="color:var(--muted)">${u.id||u.ID}</td>
      <td><strong>${esc(u.username||u.Username)}</strong></td>
      <td><span class="badge">${esc(u.role||u.Role)}</span></td>
      <td style="color:var(--muted);font-size:.8rem">${u.telegram_id||u.TelegramID||'—'}</td>
      <td style="color:var(--muted);font-size:.8rem">${ago(u.last_login||u.LastLogin)}</td>
      <td class="actions">
        <button class="btn btn-outline btn-sm" onclick="setTelegramID(${u.id||u.ID},'${esc(u.telegram_id||u.TelegramID||'')}')">🔗 綁定TG</button>
        <button class="btn btn-red btn-sm" onclick="deleteUser(${u.id||u.ID},'${esc(u.username||u.Username)}')">刪除</button>
      </td>
    </tr>`).join('')}</tbody>
  </table></div>`;
}

async function createUser(){
  const u=$('nu-user').value, p=$('nu-pass').value, r=$('nu-role').value;
  if(!u||!p){ showUserAlert('帳號和密碼不能為空','err'); return; }
  const res = await apiFetch('/users',{method:'POST',body:JSON.stringify({username:u,password:p,role:r})});
  if(res&&res.ok){ $('nu-user').value=''; $('nu-pass').value=''; showUserAlert('✅ 新增成功','ok'); await loadUsers(); }
  else{ const d=await res.json(); showUserAlert('❌ '+(d.error||'新增失敗'),'err'); }
}

async function deleteUser(id, name){
  if(!confirm(`刪除用戶 ${name}？`)) return;
  await apiFetch('/users/'+id,{method:'DELETE'});
  await loadUsers();
}

async function setTelegramID(id, current){
  const v = prompt('輸入 Telegram User ID（0 = 解除綁定）', current||'');
  if(v===null) return;
  const tid = parseInt(v)||0;
  const res = await apiFetch('/users/'+id+'/telegram',{method:'PATCH',body:JSON.stringify({telegram_id:tid})});
  if(res&&res.ok){ showUserAlert('✅ 綁定成功','ok'); await loadUsers(); }
  else showUserAlert('❌ 操作失敗','err');
}

function showUserAlert(msg, type){
  const al = $('u-alert');
  if(!al) return;
  al.innerHTML = `<div class="alert alert-${type==='ok'?'ok':'err'}">${msg}</div>`;
  setTimeout(()=>{ if(al) al.innerHTML=''; }, 4000);
}

// ── Start ─────────────────────────────────────────────────────────────────────
init();
