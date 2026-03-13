/* VerificationBot Admin SPA
 * Vanilla JS, no framework, hash-based routing
 */

const API = '/admin/api';
let token = localStorage.getItem('token') || '';
let currentRole = localStorage.getItem('role') || '';

// ── Auth helpers ─────────────────────────────────────────────────────────────

async function apiFetch(path, opts = {}) {
  opts.headers = { ...opts.headers, Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
  let res = await fetch(API + path, opts);
  if (res.status === 401) {
    // Try refresh
    const r = await fetch(API + '/auth/refresh', { method: 'POST' });
    if (r.ok) {
      const d = await r.json();
      token = d.token; localStorage.setItem('token', token);
      opts.headers.Authorization = `Bearer ${token}`;
      res = await fetch(API + path, opts);
    } else {
      logout(); return;
    }
  }
  return res;
}

async function logout() {
  await fetch(API + '/auth/logout', { method: 'POST' });
  localStorage.clear(); window.location = '/admin/login';
}

// ── Init ─────────────────────────────────────────────────────────────────────

async function init() {
  if (!token) { window.location = '/admin/login'; return; }

  // Load branding & user info in parallel
  const [meRes, settingsRes] = await Promise.all([
    apiFetch('/auth/me'),
    apiFetch('/settings'),
  ]);
  if (!meRes || !meRes.ok) { window.location = '/admin/login'; return; }

  const me = await meRes.json();
  const settings = settingsRes.ok ? await settingsRes.json() : {};

  currentRole = me.role;
  localStorage.setItem('role', currentRole);

  // Apply branding
  applyBranding(settings);
  document.getElementById('sb-user').textContent = me.username;
  const roleEl = document.getElementById('sb-role');
  roleEl.textContent = me.role;
  roleEl.className = 'user-role ' + me.role;

  // Show superadmin-only nav items
  if (me.role === 'superadmin') {
    document.querySelectorAll('.superadmin-only').forEach(el => el.style.display = '');
  }

  // Routing
  window.addEventListener('hashchange', route);
  route();
}

function applyBranding(s) {
  if (s.site_name) {
    document.getElementById('sb-name').textContent = s.site_name;
    document.title = s.site_name + ' — Admin';
  }
  if (s.site_logo) {
    document.getElementById('sb-logo').innerHTML =
      `<img class="sidebar-logo" src="${s.site_logo}" alt="logo"/>`;
  }
  if (s.primary_color) {
    document.documentElement.style.setProperty('--accent', s.primary_color);
  }
}

// ── Router ────────────────────────────────────────────────────────────────────

const PAGES = {
  dashboard: { title: '儀表板',   render: renderDashboard },
  logs:      { title: '操作記錄', render: renderLogs },
  settings:  { title: '外觀設定', render: renderSettings },
  users:     { title: '帳號管理', render: renderUsers, roles: ['superadmin'] },
};

function route() {
  const hash = location.hash.replace('#', '') || 'dashboard';
  const page = PAGES[hash] || PAGES.dashboard;

  if (page.roles && !page.roles.includes(currentRole)) {
    location.hash = '#dashboard'; return;
  }

  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === hash);
  });
  document.getElementById('page-title').textContent = page.title;
  const content = document.getElementById('content');
  content.style.opacity = '0';
  content.innerHTML = '<div class="empty">載入中…</div>';
  page.render(content).then(() => {
    content.style.transition = 'opacity .2s';
    content.style.opacity = '1';
  });
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

async function renderDashboard(el) {
  const [statsRes, logsRes] = await Promise.all([
    apiFetch('/stats'),
    apiFetch('/logs?limit=10'),
  ]);
  const stats = statsRes.ok ? await statsRes.json() : {};
  const logsData = logsRes.ok ? await logsRes.json() : { logs: [] };
  const logs = logsData.logs || [];

  el.innerHTML = `
    <div class="stats-grid">
      ${statCard('今日總計', stats.total || 0, 'blue', '📊')}
      ${statCard('驗證通過', stats.pass || 0, 'green', '✅')}
      ${statCard('超時踢出', stats.timeout || 0, 'yellow', '⏰')}
      ${statCard('申請拒絕', stats.declined || 0, 'red', '🚫')}
    </div>
    <div class="card">
      <div class="card-title">最新記錄（10 筆）</div>
      ${logsTable(logs, false)}
      <div style="margin-top:12px;text-align:right">
        <a href="#logs" class="btn btn-outline btn-sm">查看全部 →</a>
      </div>
    </div>`;

  // Auto-refresh every 30s
  el._interval = setInterval(async () => {
    const r = await apiFetch('/stats');
    if (r && r.ok) {
      const s = await r.json();
      el.querySelectorAll('.stat-num').forEach((n, i) => {
        const vals = [s.total || 0, s.pass || 0, s.timeout || 0, s.declined || 0];
        n.textContent = vals[i];
      });
    }
  }, 30000);
}

function statCard(label, value, color, icon) {
  return `<div class="stat-card ${color}">
    <div style="font-size:1.4rem;margin-bottom:6px">${icon}</div>
    <div class="stat-num">${value}</div>
    <div class="stat-label">${label}</div>
  </div>`;
}

// ── Logs ──────────────────────────────────────────────────────────────────────

let logsState = { offset: 0, limit: 50, result: '', chat_id: '' };

async function renderLogs(el) {
  logsState = { offset: 0, limit: 50, result: '', chat_id: '' };
  el.innerHTML = `
    <div class="section-header">
      <span class="section-title">操作記錄</span>
      <button class="btn btn-outline btn-sm" onclick="exportLogs()">⬇ 匯出 CSV</button>
    </div>
    <div class="filters">
      <select id="f-result" onchange="fetchLogs()">
        <option value="">全部結果</option>
        <option value="pass">✅ 通過</option>
        <option value="timeout">⏰ 超時</option>
        <option value="declined">🚫 拒絕</option>
        <option value="fail">❌ 失敗</option>
        <option value="pending">⏳ 待驗證</option>
      </select>
      <input id="f-chat" type="text" placeholder="群組 ID" style="width:140px" onchange="fetchLogs()"/>
    </div>
    <div id="logs-wrap"></div>
    <div class="pagination" id="logs-pg"></div>`;
  await fetchLogs();
}

async function fetchLogs() {
  logsState.result = document.getElementById('f-result')?.value || '';
  logsState.chat_id = document.getElementById('f-chat')?.value || '';
  const qs = new URLSearchParams({
    limit: logsState.limit, offset: logsState.offset,
    result: logsState.result, chat_id: logsState.chat_id,
  });
  const res = await apiFetch('/logs?' + qs);
  if (!res || !res.ok) return;
  const { logs, total } = await res.json();
  document.getElementById('logs-wrap').innerHTML = logsTable(logs || [], true);
  renderPagination(total);
}

function renderPagination(total) {
  const pg = document.getElementById('logs-pg');
  if (!pg) return;
  const totalPages = Math.ceil(total / logsState.limit);
  const cur = Math.floor(logsState.offset / logsState.limit) + 1;
  pg.innerHTML = `
    <button class="page-btn" ${cur <= 1 ? 'disabled' : ''} onclick="changePage(-1)">← 上一頁</button>
    <span>第 ${cur} / ${totalPages || 1} 頁（共 ${total} 筆）</span>
    <button class="page-btn" ${cur >= totalPages ? 'disabled' : ''} onclick="changePage(1)">下一頁 →</button>`;
}

function changePage(dir) {
  logsState.offset = Math.max(0, logsState.offset + dir * logsState.limit);
  fetchLogs();
}

function exportLogs() {
  window.open(API + '/logs/export', '_blank');
}

function logsTable(logs, showAll = true) {
  if (!logs || !logs.length) return '<div class="empty">暫無記錄</div>';
  const rows = logs.map(l => `
    <tr>
      <td style="color:var(--muted);font-size:.78rem">${fmtTime(l.Ts)}</td>
      <td><span class="badge badge-${l.Result}">${resultLabel(l.Result)}</span></td>
      ${showAll ? `<td>${esc(l.ChatTitle) || l.ChatID || '—'}</td>` : ''}
      <td>${esc(l.Username) || l.UserID || '—'}</td>
      <td style="color:var(--muted)">${esc(l.EventType)}</td>
      ${showAll ? `<td style="color:var(--muted);font-size:.8rem">${esc(l.Detail)}</td>` : ''}
    </tr>`).join('');
  return `<div class="table-wrap"><table>
    <thead><tr>
      <th>時間</th><th>結果</th>
      ${showAll ? '<th>群組</th>' : ''}
      <th>用戶</th><th>事件</th>
      ${showAll ? '<th>詳情</th>' : ''}
    </tr></thead>
    <tbody>${rows}</tbody>
  </table></div>`;
}

function resultLabel(r) {
  return { pass: '✅ 通過', fail: '❌ 失敗', timeout: '⏰ 超時', declined: '🚫 拒絕', pending: '⏳ 待驗證' }[r] || r;
}

// ── Settings ──────────────────────────────────────────────────────────────────

async function renderSettings(el) {
  const res = await apiFetch('/settings');
  const s = res.ok ? await res.json() : {};
  const readonly = !['superadmin', 'admin'].includes(currentRole);

  el.innerHTML = `
    <div class="section-title" style="margin-bottom:24px">外觀設定</div>
    <div class="card" style="max-width:520px">
      <div class="form-group">
        <label class="form-label">網站名稱</label>
        <input class="form-control" id="s-name" value="${esc(s.site_name||'')}" ${readonly?'readonly disabled':''} placeholder="VerificationBot Admin"/>
      </div>
      <div class="form-group">
        <label class="form-label">Logo URL</label>
        <input class="form-control" id="s-logo" value="${esc(s.site_logo||'')}" ${readonly?'readonly disabled':''} placeholder="https://example.com/logo.png"/>
      </div>
      <div class="form-group">
        <label class="form-label">主色調</label>
        <div class="color-preview">
          <input class="form-control" id="s-color" value="${esc(s.primary_color||'#2ea6ff')}" ${readonly?'readonly disabled':''} placeholder="#2ea6ff" style="flex:1" oninput="updateSwatch()"/>
          <div class="color-swatch" id="color-swatch" style="background:${esc(s.primary_color||'#2ea6ff')}"></div>
        </div>
      </div>
      ${readonly ? '<p style="color:var(--muted);font-size:.85rem">你的權限不允許修改設定。</p>'
        : `<button class="btn btn-accent" onclick="saveSettings()">💾 儲存設定</button>`}
      <p id="s-msg" style="margin-top:12px;font-size:.88rem;display:none"></p>
    </div>`;
}

function updateSwatch() {
  const v = document.getElementById('s-color').value;
  document.getElementById('color-swatch').style.background = v;
}

async function saveSettings() {
  const body = {
    site_name: document.getElementById('s-name').value,
    site_logo: document.getElementById('s-logo').value,
    primary_color: document.getElementById('s-color').value,
  };
  const res = await apiFetch('/settings', { method: 'PATCH', body: JSON.stringify(body) });
  const msg = document.getElementById('s-msg');
  if (res && res.ok) {
    msg.textContent = '✅ 儲存成功！頁面刷新後生效。'; msg.style.color = 'var(--green)';
    applyBranding(body);
  } else {
    msg.textContent = '❌ 儲存失敗，請重試。'; msg.style.color = 'var(--red)';
  }
  msg.style.display = 'block';
}

// ── Users ─────────────────────────────────────────────────────────────────────

async function renderUsers(el) {
  el.innerHTML = `
    <div class="section-header">
      <span class="section-title">帳號管理</span>
      <button class="btn btn-accent btn-sm" onclick="showCreateUser()">＋ 新增帳號</button>
    </div>
    <div id="users-wrap"></div>`;
  await fetchUsers();
}

async function fetchUsers() {
  const res = await apiFetch('/users');
  if (!res || !res.ok) return;
  const users = await res.json();
  const rows = (users || []).map(u => `
    <tr>
      <td>${u.ID}</td>
      <td><strong>${esc(u.Username)}</strong></td>
      <td><span class="badge badge-${u.Role === 'superadmin' ? 'fail' : u.Role === 'admin' ? 'timeout' : 'pass'}">${u.Role}</span></td>
      <td style="color:var(--muted);font-size:.8rem">${fmtTime(u.CreatedAt)}</td>
      <td>
        <button class="btn btn-outline btn-sm" onclick="showEditUser(${u.ID},'${u.Role}')">編輯</button>
        <button class="btn btn-danger btn-sm" style="margin-left:4px" onclick="deleteUser(${u.ID},'${esc(u.Username)}')">刪除</button>
      </td>
    </tr>`).join('');
  document.getElementById('users-wrap').innerHTML = `
    <div class="table-wrap"><table>
      <thead><tr><th>ID</th><th>帳號</th><th>角色</th><th>建立時間</th><th>操作</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="5" class="empty">暫無帳號</td></tr>'}</tbody>
    </table></div>`;
}

function showCreateUser() {
  showModal(`
    <div class="modal-title">新增帳號</div>
    <div class="form-group"><label class="form-label">帳號</label><input class="form-control" id="m-user" placeholder="username"/></div>
    <div class="form-group"><label class="form-label">密碼</label><input class="form-control" type="password" id="m-pass" placeholder="••••••••"/></div>
    <div class="form-group"><label class="form-label">角色</label>
      <select class="form-control" id="m-role">
        <option value="viewer">viewer — 只讀</option>
        <option value="admin" selected>admin — 可改設定</option>
        <option value="superadmin">superadmin — 完整權限</option>
      </select>
    </div>
    <p id="m-err" style="color:var(--red);font-size:.85rem;display:none"></p>
    <div class="modal-footer">
      <button class="btn btn-outline" onclick="closeModal()">取消</button>
      <button class="btn btn-accent" onclick="createUser()">建立</button>
    </div>`);
}

function showEditUser(id, role) {
  showModal(`
    <div class="modal-title">編輯帳號 #${id}</div>
    <div class="form-group"><label class="form-label">新密碼（留空不改）</label><input class="form-control" type="password" id="m-pass" placeholder="••••••••"/></div>
    <div class="form-group"><label class="form-label">角色</label>
      <select class="form-control" id="m-role">
        <option value="viewer" ${role==='viewer'?'selected':''}>viewer</option>
        <option value="admin" ${role==='admin'?'selected':''}>admin</option>
        <option value="superadmin" ${role==='superadmin'?'selected':''}>superadmin</option>
      </select>
    </div>
    <div class="modal-footer">
      <button class="btn btn-outline" onclick="closeModal()">取消</button>
      <button class="btn btn-accent" onclick="editUser(${id})">儲存</button>
    </div>`);
}

async function createUser() {
  const body = {
    username: document.getElementById('m-user').value,
    password: document.getElementById('m-pass').value,
    role: document.getElementById('m-role').value,
  };
  const res = await apiFetch('/users', { method: 'POST', body: JSON.stringify(body) });
  const data = await res.json();
  if (res.ok) { closeModal(); fetchUsers(); }
  else { const e = document.getElementById('m-err'); e.textContent = data.error; e.style.display = 'block'; }
}

async function editUser(id) {
  const body = {
    password: document.getElementById('m-pass').value,
    role: document.getElementById('m-role').value,
  };
  await apiFetch('/users/' + id, { method: 'PATCH', body: JSON.stringify(body) });
  closeModal(); fetchUsers();
}

async function deleteUser(id, name) {
  if (!confirm(`確定要刪除帳號 "${name}"？`)) return;
  await apiFetch('/users/' + id, { method: 'DELETE' });
  fetchUsers();
}

// ── Modal ─────────────────────────────────────────────────────────────────────

function showModal(html) {
  const overlay = document.createElement('div');
  overlay.className = 'modal-overlay';
  overlay.id = 'modal-overlay';
  overlay.innerHTML = `<div class="modal">${html}</div>`;
  overlay.addEventListener('click', e => { if (e.target === overlay) closeModal(); });
  document.body.appendChild(overlay);
}

function closeModal() {
  document.getElementById('modal-overlay')?.remove();
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function esc(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function fmtTime(ts) {
  if (!ts) return '—';
  try {
    const d = new Date(ts);
    return d.toLocaleString('zh-TW', { month:'2-digit', day:'2-digit',
      hour:'2-digit', minute:'2-digit', second:'2-digit', hour12:false });
  } catch { return ts; }
}

// ── Bootstrap ─────────────────────────────────────────────────────────────────
init();
