// API 基础地址
const API_BASE = '/api';

// 当前选中的连接
let currentConnId = null;
let currentTable = null;
let currentDb = '';
let currentRows = [];
let isEditing = false;
let quickMode = '';

// ========== 认证相关 ==========
let authToken = localStorage.getItem('db_token') || '';
let currentUser = null;

// 带认证的 fetch
async function authFetch(url, options = {}) {
    const headers = options.headers || {};
    if (authToken) {
        headers['Authorization'] = 'Bearer ' + authToken;
    }
    options.headers = headers;
    const res = await fetch(url, options);
    if (res.status === 401) {
        authToken = '';
        localStorage.removeItem('db_token');
        showApp(false);
        showToast('登录已过期，请重新登录', 'warning');
        throw new Error('unauthorized');
    }
    return res;
}

// 切换登录/注册标签
function switchAuthTab(tab) {
    document.getElementById('tabLogin').classList.toggle('active', tab === 'login');
    document.getElementById('tabRegister').classList.toggle('active', tab === 'register');
    document.getElementById('authSubmitBtn').textContent = tab === 'login' ? '登 录' : '注 册';
    document.getElementById('authHint').textContent = '';
    document.getElementById('authUsername').value = '';
    document.getElementById('authPassword').value = '';
}

// 提交登录/注册
async function submitAuth() {
    const isLogin = document.getElementById('tabLogin').classList.contains('active');
    const username = document.getElementById('authUsername').value.trim();
    const password = document.getElementById('authPassword').value;
    const hint = document.getElementById('authHint');

    if (!username || !password) {
        hint.textContent = '请输入用户名和密码';
        hint.className = 'auth-hint error';
        return;
    }

    const url = isLogin ? `${API_BASE}/auth/login` : `${API_BASE}/auth/register`;
    try {
        const res = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });
        const data = await res.json();
        if (data.code === 0) {
            if (isLogin) {
                authToken = data.data.token;
                currentUser = data.data.user;
                localStorage.setItem('db_token', authToken);
                showApp(true);
                showToast('登录成功', 'success');
            } else {
                hint.textContent = '注册成功，请等待管理员审批后登录';
                hint.className = 'auth-hint success';
                switchAuthTab('login');
            }
        } else {
            hint.textContent = data.message;
            hint.className = 'auth-hint error';
        }
    } catch (e) {
        hint.textContent = '请求失败：' + e.message;
        hint.className = 'auth-hint error';
    }
}

// 退出登录
async function logout() {
    try {
        await authFetch(`${API_BASE}/auth/logout`, { method: 'POST' });
    } catch (e) {}
    authToken = '';
    currentUser = null;
    localStorage.removeItem('db_token');
    showApp(false);
}

// 显示/隐藏主应用
function showApp(show) {
    document.getElementById('authPage').style.display = show ? 'none' : 'flex';
    document.getElementById('app').style.display = show ? 'flex' : 'none';
    if (show) {
        document.getElementById('userName').textContent = currentUser.username;
        document.getElementById('userRole').textContent = currentUser.role === 'admin' ? '管理员' : '用户';
        document.getElementById('adminSection').style.display = currentUser.role === 'admin' ? 'block' : 'none';
        document.getElementById('onlineBadge').style.display = currentUser.role === 'admin' ? 'inline-flex' : 'none';
        loadConnections();
        startOnlinePolling();
    } else {
        stopOnlinePolling();
    }
}

// 验证已有 token
async function checkAuth() {
    if (!authToken) {
        showApp(false);
        return;
    }
    try {
        const res = await authFetch(`${API_BASE}/auth/me`);
        const data = await res.json();
        if (data.code === 0) {
            currentUser = data.data;
            showApp(true);
        } else {
            showApp(false);
        }
    } catch (e) {
        showApp(false);
    }
}

// ========== 在线用户轮询 ==========
let onlineTimer = null;
function startOnlinePolling() {
    if (currentUser && currentUser.role === 'admin') {
        refreshOnlineCount();
        onlineTimer = setInterval(refreshOnlineCount, 15000);
    }
}
function stopOnlinePolling() {
    if (onlineTimer) {
        clearInterval(onlineTimer);
        onlineTimer = null;
    }
}
async function refreshOnlineCount() {
    try {
        const res = await authFetch(`${API_BASE}/admin/online-users`);
        const data = await res.json();
        if (data.code === 0) {
            document.getElementById('onlineCount').textContent = data.data.count;
        }
    } catch (e) {}
}

// ========== 管理员面板 ==========
function openAdminPanel(view) {
    if (currentUser.role !== 'admin') {
        showToast('需要管理员权限', 'error');
        return;
    }
    document.getElementById('welcome').style.display = 'none';
    document.getElementById('queryPanel').style.display = 'none';
    document.getElementById('toolbar').style.display = 'none';
    document.getElementById('adminPanel').style.display = 'block';
    document.getElementById('adminUsersView').style.display = 'none';
    document.getElementById('adminAuditView').style.display = 'none';
    document.getElementById('adminOnlineView').style.display = 'none';

    const titles = { users: '用户管理', audit: '审计日志', online: '在线用户' };
    document.getElementById('topbarTitle').textContent = '系统管理 - ' + titles[view];

    if (view === 'users') {
        document.getElementById('adminUsersView').style.display = 'block';
        loadUsers();
    } else if (view === 'audit') {
        document.getElementById('adminAuditView').style.display = 'block';
        loadAuditLogs();
    } else if (view === 'online') {
        document.getElementById('adminOnlineView').style.display = 'block';
        loadOnlineUsers();
    }
}

function closeAdminPanel() {
    document.getElementById('adminPanel').style.display = 'none';
    document.getElementById('topbarTitle').textContent = '数据库管理';
    if (currentConnId) {
        document.getElementById('queryPanel').style.display = 'flex';
        document.getElementById('toolbar').style.display = 'flex';
    } else {
        document.getElementById('welcome').style.display = 'flex';
    }
}

// 用户管理
async function loadUsers() {
    try {
        const res = await authFetch(`${API_BASE}/admin/users`);
        const data = await res.json();
        if (data.code === 0) {
            const tbody = document.getElementById('usersTbody');
            tbody.innerHTML = data.data.map(u => `
                <tr>
                    <td>${u.username}</td>
                    <td><span class="role-badge role-${u.role}">${u.role === 'admin' ? '管理员' : '普通用户'}</span></td>
                    <td>${statusBadge(u.status)}</td>
                    <td>${fmtTime(u.created_at)}</td>
                    <td>${u.last_login_at ? fmtTime(u.last_login_at) : '-'}</td>
                    <td>
                        ${u.status === 'pending' ? `<button class="btn btn-xs btn-primary" onclick="approveUser('${u.id}')">通过</button>` : ''}
                        ${u.role === 'user' ? `<button class="btn btn-xs" onclick="changeRole('${u.id}')">设为管理员</button>` : `<button class="btn btn-xs" onclick="changeRole('${u.id}')">取消管理员</button>`}
                        ${u.role !== 'admin' ? `<button class="btn btn-xs" onclick="toggleUser('${u.id}')">${u.status === 'disabled' ? '启用' : '禁用'}</button>` : ''}
                        ${u.role !== 'admin' ? `<button class="btn btn-xs btn-danger" onclick="deleteUser('${u.id}')">删除</button>` : ''}
                    </td>
                </tr>
            `).join('');
        }
    } catch (e) {
        showToast('加载用户失败', 'error');
    }
}

function statusBadge(status) {
    const map = {
        pending: '<span class="status-badge status-pending">待审批</span>',
        active: '<span class="status-badge status-active">正常</span>',
        disabled: '<span class="status-badge status-disabled">已禁用</span>'
    };
    return map[status] || status;
}

function fmtTime(t) {
    if (!t) return '-';
    const d = new Date(t);
    return d.toLocaleString('zh-CN', { hour12: false });
}

async function approveUser(id) {
    try {
        const res = await authFetch(`${API_BASE}/admin/users/${id}/approve`, { method: 'PUT' });
        const data = await res.json();
        if (data.code === 0) {
            showToast('已通过审批', 'success');
            loadUsers();
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('操作失败', 'error');
    }
}

async function toggleUser(id) {
    try {
        const res = await authFetch(`${API_BASE}/admin/users/${id}/disable`, { method: 'PUT' });
        const data = await res.json();
        if (data.code === 0) {
            showToast('操作成功', 'success');
            loadUsers();
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('操作失败', 'error');
    }
}

async function changeRole(id) {
    if (!confirm('确定要更改该用户的角色吗？更改后该用户需重新登录。')) return;
    try {
        const res = await authFetch(`${API_BASE}/admin/users/${id}/role`, { method: 'PUT' });
        const data = await res.json();
        if (data.code === 0) {
            showToast(data.data, 'success');
            loadUsers();
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('操作失败', 'error');
    }
}

async function deleteUser(id) {
    if (!confirm('确定要删除该用户吗？')) return;
    try {
        const res = await authFetch(`${API_BASE}/admin/users/${id}`, { method: 'DELETE' });
        const data = await res.json();
        if (data.code === 0) {
            showToast('删除成功', 'success');
            loadUsers();
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('删除失败', 'error');
    }
}

// 审计日志
async function loadAuditLogs() {
    const username = document.getElementById('auditUserFilter').value.trim();
    const action = document.getElementById('auditActionFilter').value.trim();
    let url = `${API_BASE}/admin/audit-logs?limit=200`;
    if (username) url += `&username=${encodeURIComponent(username)}`;
    if (action) url += `&action=${encodeURIComponent(action)}`;
    try {
        const res = await authFetch(url);
        const data = await res.json();
        if (data.code === 0) {
            const tbody = document.getElementById('auditTbody');
            if (data.data.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;color:#86909c;">暂无日志</td></tr>';
                return;
            }
            tbody.innerHTML = data.data.map(log => `
                <tr>
                    <td>${fmtTime(log.time)}</td>
                    <td>${log.username}</td>
                    <td>${log.role === 'admin' ? '管理员' : (log.role || '-')}</td>
                    <td>${log.action}</td>
                    <td>${log.resource}</td>
                    <td title="${log.detail}">${log.detail.length > 60 ? log.detail.substring(0, 60) + '...' : log.detail}</td>
                    <td>${log.ip}</td>
                </tr>
            `).join('');
        }
    } catch (e) {
        showToast('加载日志失败', 'error');
    }
}

// 在线用户
async function loadOnlineUsers() {
    try {
        const res = await authFetch(`${API_BASE}/admin/online-users`);
        const data = await res.json();
        if (data.code === 0) {
            document.getElementById('onlineNum').textContent = data.data.count;
            const tbody = document.getElementById('onlineTbody');
            if (data.data.users.length === 0) {
                tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:#86909c;">暂无在线用户</td></tr>';
                return;
            }
            tbody.innerHTML = data.data.users.map(u => `
                <tr>
                    <td>${u.username}</td>
                    <td>${u.role === 'admin' ? '管理员' : '普通用户'}</td>
                    <td>${fmtTime(u.login_at)}</td>
                    <td>${fmtTime(u.last_active)}</td>
                    <td>${u.ip}</td>
                </tr>
            `).join('');
        }
    } catch (e) {
        showToast('加载失败', 'error');
    }
}

// ========== 原有功能（使用 authFetch） ==========

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', () => {
    checkAuth();

    // Ctrl+Enter 执行查询
    const editor = document.getElementById('sqlEditor');
    if (editor) {
        editor.addEventListener('keydown', (e) => {
            if (e.ctrlKey && e.key === 'Enter') {
                executeQuery();
            }
        });
    }
});

// 加载连接列表
async function loadConnections() {
    try {
        const res = await authFetch(`${API_BASE}/connections`);
        const data = await res.json();
        if (data.code === 0) {
            renderConnectionList(data.data);
        }
    } catch (e) {
        showToast('加载连接列表失败', 'error');
    }
}

// 渲染连接列表
function renderConnectionList(connections) {
    const list = document.getElementById('connectionList');
    if (connections.length === 0) {
        list.innerHTML = '<div style="padding: 20px; text-align: center; color: #86909c; font-size: 13px;">暂无连接，点击上方按钮创建</div>';
        return;
    }
    
    list.innerHTML = connections.map(conn => `
        <div class="conn-item ${conn.id === currentConnId ? 'active' : ''}" 
             onclick="selectConnection('${conn.id}')"
             ondblclick="editConnection('${conn.id}')">
            <div class="conn-item-title">
                <span class="type-badge type-${conn.type}">${getTypeLabel(conn.type)}</span>
                ${conn.name}
            </div>
            <div class="conn-item-type">${getConnSubtitle(conn)}</div>
        </div>
    `).join('');
    
    const switchSel = document.getElementById('connQuickSwitch');
    if (switchSel) {
        switchSel.innerHTML = connections.map(conn => `
            <option value="${conn.id}">${getTypeLabel(conn.type)} · ${conn.name}</option>
        `).join('');
        if (currentConnId) switchSel.value = currentConnId;
    }
}

// 获取类型标签
function getTypeLabel(type) {
    const labels = { mysql: 'MySQL', postgres: 'PgSQL', sqlite: 'SQLite', mongodb: 'Mongo', redis: 'Redis' };
    return labels[type] || type;
}

// 获取连接副标题
function getConnSubtitle(conn) {
    if (conn.type === 'sqlite') {
        return conn.file_path || '本地文件';
    } else if (conn.type === 'redis') {
        return `${conn.host || 'localhost'}:${conn.port || 6379} / DB${conn.db_index || 0}`;
    } else {
        const host = conn.host || 'localhost';
        const port = conn.port ? ':' + conn.port : '';
        const db = conn.database ? ' / ' + conn.database : '';
        return host + port + db;
    }
}

// 顶部快速切换连接
function quickSwitchConn() {
    const sel = document.getElementById('connQuickSwitch');
    const id = sel.value;
    if (id && id !== currentConnId) {
        selectConnection(id);
    }
}

// 选择连接
async function selectConnection(id) {
    currentConnId = id;
    currentOptTab = '';
    document.getElementById('optimizePanel').style.display = 'none';
    closeAdminPanel();
    loadConnections();
    
    document.getElementById('sqlEditor').value = '';
    document.getElementById('resultInfo').textContent = '执行结果';
    document.getElementById('resultContent').innerHTML = '<div class="empty-result">正在加载连接...</div>';
    
    try {
        const res = await authFetch(`${API_BASE}/connections/${id}`);
        const data = await res.json();
        if (data.code === 0) {
            const conn = data.data;
            document.getElementById('currentConnName').textContent = conn.name;
            document.getElementById('currentConnType').textContent = getTypeLabel(conn.type);
            document.getElementById('toolbar').style.display = 'flex';
            document.getElementById('welcome').style.display = 'none';
            document.getElementById('queryPanel').style.display = 'flex';
            
            const collectionInput = document.getElementById('collectionInput');
            if (conn.type === 'mongodb') {
                collectionInput.style.display = 'inline-block';
            } else {
                collectionInput.style.display = 'none';
            }
            
            document.getElementById('btnNewDb').style.display = (conn.type === 'mysql' || conn.type === 'postgres') ? 'inline-block' : 'none';
            document.getElementById('btnNewTable').style.display = (conn.type === 'redis') ? 'none' : 'inline-block';
            currentTable = null;
            
            document.getElementById('resultInfo').textContent = '执行结果';
            document.getElementById('resultContent').innerHTML = '<div class="empty-result">执行查询后显示结果</div>';
            
            if (conn.type === 'mysql' || conn.type === 'postgres') {
                await loadDatabases(conn.id, conn.database);
            } else {
                document.getElementById('dbSelector').style.display = 'none';
                currentDb = '';
                loadTables(id);
            }
        }
    } catch (e) {
        showToast('加载连接信息失败', 'error');
    }
}

// 加载数据库列表
async function loadDatabases(connId, defaultDb) {
    const selector = document.getElementById('dbSelector');
    const select = document.getElementById('dbSelect');
    selector.style.display = 'block';
    select.innerHTML = '<option>加载中...</option>';
    try {
        const res = await authFetch(`${API_BASE}/quick/${connId}/databases`);
        const data = await res.json();
        if (data.code === 0) {
            const dbs = data.data;
            if (dbs.length === 0) {
                select.innerHTML = '<option value="">(无)</option>';
                currentDb = '';
            } else {
                select.innerHTML = dbs.map(db => `<option value="${db}">${db}</option>`).join('');
                if (defaultDb && dbs.includes(defaultDb)) {
                    select.value = defaultDb;
                }
                currentDb = select.value;
            }
            loadTables(connId);
        } else {
            selector.style.display = 'none';
            loadTables(connId);
        }
    } catch (e) {
        selector.style.display = 'none';
        loadTables(connId);
    }
}

function onDbChange() {
    currentDb = document.getElementById('dbSelect').value;
    currentTable = null;
    loadTables(currentConnId);
}

async function loadTables(connId) {
    const tableList = document.getElementById('tableList');
    tableList.innerHTML = '<div class="loading">加载中...</div>';
    
    try {
        let url = `${API_BASE}/connections/${connId}/tables`;
        if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
            url = `${API_BASE}/quick/${connId}/tables?db=${encodeURIComponent(currentDb)}`;
        }
        const res = await authFetch(url);
        const data = await res.json();
        if (data.code === 0) {
            const tables = data.data;
            if (tables.length === 0) {
                tableList.innerHTML = '<div style="color: #86909c; font-size: 13px;">暂无表</div>';
            } else {
                tableList.innerHTML = tables.map(table => {
                    const safe = table.replace(/'/g, "\\'");
                    return `
                    <div class="table-item ${table === currentTable ? 'selected' : ''}">
                        <span class="table-name" onclick="selectTable('${safe}')" ondblclick="insertTableName('${safe}')"
                              title="单击查看数据，双击插入SQL">${table}</span>
                        <span class="table-op">
                            <button class="table-op-btn" title="改名" onclick="quickRenameTable('${safe}')">✏️</button>
                            <button class="table-op-btn danger" title="删除" onclick="quickDropTable()">🗑</button>
                        </span>
                    </div>`;
                }).join('');
            }
        } else {
            tableList.innerHTML = `<div style="color: #f53f3f; font-size: 13px;">${data.message}</div>`;
        }
    } catch (e) {
        tableList.innerHTML = '<div style="color: #f53f3f; font-size: 13px;">加载失败</div>';
    }
}

function refreshTables() {
    if (currentConnId) loadTables(currentConnId);
}

function insertTableName(tableName) {
    const editor = document.getElementById('sqlEditor');
    const connType = document.getElementById('currentConnType').textContent;
    if (connType === 'Redis') {
        document.getElementById('collectionInput').value = '';
        editor.value = `GET ${tableName}`;
    } else if (connType === 'Mongo') {
        document.getElementById('collectionInput').value = tableName;
        editor.value = '{}';
    } else {
        editor.value = `SELECT * FROM ${tableName} LIMIT 100;`;
    }
    editor.focus();
}

async function executeQuery() {
    if (!currentConnId) { showToast('请先选择一个连接', 'warning'); return; }
    const sql = document.getElementById('sqlEditor').value.trim();
    if (!sql) { showToast('请输入查询语句', 'warning'); return; }
    
    const collection = document.getElementById('collectionInput').value.trim();
    const resultContent = document.getElementById('resultContent');
    const resultInfo = document.getElementById('resultInfo');
    
    resultInfo.textContent = '执行中...';
    resultContent.innerHTML = '<div class="empty-result">执行中...</div>';
    const startTime = Date.now();
    
    try {
        const res = await authFetch(`${API_BASE}/connections/query`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ connection_id: currentConnId, sql, collection })
        });
        const data = await res.json();
        const elapsed = Date.now() - startTime;
        
        if (data.code === 0) {
            const result = data.data;
            if (result.success) {
                if (result.rows && result.rows.length > 0) {
                    resultInfo.textContent = `执行成功，共 ${result.count} 条记录，耗时 ${elapsed}ms`;
                    renderResultTable(result);
                } else {
                    resultInfo.textContent = `执行成功，耗时 ${elapsed}ms`;
                    resultContent.innerHTML = `<div class="result-success">${result.message || '执行成功'}</div>`;
                }
            } else {
                resultInfo.textContent = '执行失败';
                resultContent.innerHTML = `<div class="result-error">${result.message}</div>`;
            }
        } else {
            resultInfo.textContent = '执行失败';
            resultContent.innerHTML = `<div class="result-error">${data.message}</div>`;
        }
    } catch (e) {
        resultInfo.textContent = '请求失败';
        resultContent.innerHTML = `<div class="result-error">${e.message}</div>`;
    }
}

function renderResultTable(result) {
    const resultContent = document.getElementById('resultContent');
    const columns = result.columns || Object.keys(result.rows[0] || {});
    const isSqlLike = ['MySQL', 'PgSQL', 'SQLite'].includes(getConnType());
    
    const html = `
        <table class="result-table">
            <thead>
                <tr>
                    ${columns.map(col => `<th>${col}</th>`).join('')}
                    ${isSqlLike ? '<th class="row-ops-th">操作</th>' : ''}
                </tr>
            </thead>
            <tbody>
                ${result.rows.map((row, idx) => {
                    if (typeof row === 'object') {
                        const cells = columns.map(col => `<td>${formatValue(row[col])}</td>`).join('');
                        const ops = isSqlLike
                            ? `<td class="row-ops">
                                 <button class="row-op-btn" title="编辑" onclick="editRow(${idx})">编辑</button>
                                 <button class="row-op-btn danger" title="删除" onclick="deleteRow(${idx})">删除</button>
                               </td>`
                            : '';
                        return `<tr>${cells}${ops}</tr>`;
                    }
                    return `<tr><td>${formatValue(row)}</td></tr>`;
                }).join('')}
            </tbody>
        </table>
    `;
    resultContent.innerHTML = html;
}

function quoteSqlVal(v) {
    if (v === null || v === undefined) return 'NULL';
    if (typeof v === 'number') return String(v);
    return "'" + String(v).replace(/'/g, "''") + "'";
}

function getCurrentConn() {
    const items = [...document.querySelectorAll('.conn-item')];
    const active = items.find(i => i.classList.contains('active'));
    return active ? { name: active.textContent.trim().split('\n')[1].trim() } : null;
}

async function getTableColumns(table) {
    let url = `${API_BASE}/quick/${currentConnId}/columns?table=${encodeURIComponent(table)}`;
    if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
        url += `&db=${encodeURIComponent(currentDb)}`;
    }
    const res = await authFetch(url);
    const data = await res.json();
    if (data.code !== 0) throw new Error(data.message);
    const rows = data.data.rows || [];
    let pk = null;
    for (const row of rows) {
        if (row.Key === 'PRI' || (typeof row.pk === 'number' && row.pk > 0)) {
            pk = row.Field || row.name;
            break;
        }
    }
    return { columns: rows, pk };
}

async function editRow(idx) {
    const row = currentRows[idx];
    if (!row) return;
    const table = currentTable;
    try {
        const { columns, pk } = await getTableColumns(table);
        if (!pk) { showToast('该表无主键，无法生成更新条件', 'warning'); return; }
        const colNames = columns.map(c => c.Field || c.name || c.column_name).filter(Boolean);
        const sets = colNames
            .filter(c => row[c] !== undefined && c !== pk)
            .map(c => `\`${c}\` = ${quoteSqlVal(row[c])}`)
            .join(',\n    ');
        const where = `\`${pk}\` = ${quoteSqlVal(row[pk])}`;
        const sql = `UPDATE \`${table}\`\nSET ${sets}\nWHERE ${where};`;
        
        quickMode = 'execSql';
        openQuick(`更新行 - ${table}`);
        document.getElementById('quickFormContainer').innerHTML = `
            <p style="color:#86909c;font-size:12px;margin-bottom:8px;">示例语句（以主键 ${pk} 为条件，可直接修改后执行）</p>
            <textarea id="qSql" style="width:100%;height:180px;font-family:Consolas,monospace;font-size:12px;padding:8px;border:1px solid #e5e6eb;border-radius:4px;box-sizing:border-box;">${sql.replace(/</g, '&lt;')}</textarea>
        `;
    } catch (e) {
        showToast('获取列信息失败：' + e.message, 'error');
    }
}

async function deleteRow(idx) {
    const row = currentRows[idx];
    if (!row) return;
    const table = currentTable;
    try {
        const { pk } = await getTableColumns(table);
        if (!pk) { showToast('该表无主键，无法生成删除条件', 'warning'); return; }
        const where = `\`${pk}\` = ${quoteSqlVal(row[pk])}`;
        if (!confirm(`确定要删除该记录吗？\n\nDELETE FROM ${table} WHERE ${where}`)) return;
        const sql = `DELETE FROM \`${table}\` WHERE ${where};`;
        const res = await authFetch(`${API_BASE}/connections/query`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ connection_id: currentConnId, sql, collection: '' })
        });
        const data = await res.json();
        if (data.code === 0 && data.data.success) {
            showToast('删除成功', 'success');
            quickViewRows();
        } else {
            showToast('删除失败：' + (data.data?.message || data.message), 'error');
        }
    } catch (e) {
        showToast('删除失败：' + e.message, 'error');
    }
}

function quickRenameTable(table) {
    currentTable = table;
    quickMode = 'renameTable';
    openQuick(`表改名 - ${table}`);
    document.getElementById('quickFormContainer').innerHTML = `
        <input type="hidden" id="qTable" value="${table}">
        <div class="form-group">
            <label>新表名 *</label>
            <input type="text" id="qNewName" value="${table}" placeholder="新表名">
        </div>
    `;
}

function formatValue(val) {
    if (val === null || val === undefined) return '<span style="color: #86909c;">NULL</span>';
    if (typeof val === 'object') return JSON.stringify(val);
    return String(val).length > 200 ? String(val).substring(0, 200) + '...' : String(val);
}

function openAddModal() {
    isEditing = false;
    document.getElementById('modalTitle').textContent = '新建连接';
    document.getElementById('connForm').reset();
    document.getElementById('connId').value = '';
    document.getElementById('connHost').value = 'localhost';
    document.getElementById('connPort').value = '3306';
    document.getElementById('connDBIndex').value = '0';
    onTypeChange();
    document.getElementById('connModal').classList.add('show');
}

async function editConnection(id) {
    try {
        const res = await authFetch(`${API_BASE}/connections/${id}`);
        const data = await res.json();
        if (data.code === 0) {
            const conn = data.data;
            isEditing = true;
            document.getElementById('modalTitle').textContent = '编辑连接';
            document.getElementById('connId').value = conn.id;
            document.getElementById('connName').value = conn.name;
            document.getElementById('connType').value = conn.type;
            document.getElementById('connHost').value = conn.host || 'localhost';
            document.getElementById('connPort').value = conn.port || 3306;
            document.getElementById('connUsername').value = conn.username || '';
            document.getElementById('connPassword').value = '';
            document.getElementById('connDatabase').value = conn.database || '';
            document.getElementById('connFilePath').value = conn.file_path || '';
            document.getElementById('connDBIndex').value = conn.db_index || 0;
            onTypeChange();
            document.getElementById('connModal').classList.add('show');
        }
    } catch (e) {
        showToast('加载连接信息失败', 'error');
    }
}

function onTypeChange() {
    const type = document.getElementById('connType').value;
    const hostPortRow = document.getElementById('hostPortRow');
    const dbGroup = document.getElementById('dbGroup');
    const fileGroup = document.getElementById('fileGroup');
    const dbIndexGroup = document.getElementById('dbIndexGroup');
    
    const ports = { mysql: 3306, postgres: 5432, sqlite: 0, mongodb: 27017, redis: 6379 };
    if (ports[type]) document.getElementById('connPort').value = ports[type];
    
    if (type === 'sqlite') {
        hostPortRow.style.display = 'none';
        dbGroup.style.display = 'none';
        fileGroup.style.display = 'block';
        dbIndexGroup.style.display = 'none';
    } else if (type === 'redis') {
        hostPortRow.style.display = 'flex';
        dbGroup.style.display = 'none';
        fileGroup.style.display = 'none';
        dbIndexGroup.style.display = 'block';
    } else if (type === 'mongodb') {
        hostPortRow.style.display = 'flex';
        dbGroup.style.display = 'block';
        fileGroup.style.display = 'none';
        dbIndexGroup.style.display = 'none';
    } else {
        hostPortRow.style.display = 'flex';
        dbGroup.style.display = 'block';
        fileGroup.style.display = 'none';
        dbIndexGroup.style.display = 'none';
    }
}

function closeModal() {
    document.getElementById('connModal').classList.remove('show');
}

async function testConnection() {
    const connData = getFormData();
    try {
        const res = await authFetch(`${API_BASE}/connections/test`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(connData)
        });
        const data = await res.json();
        if (data.code === 0) {
            const result = data.data;
            if (result.success) {
                showToast(`连接成功！延迟 ${result.latency_ms}ms`, 'success');
            } else {
                showToast(`连接失败：${result.message}`, 'error');
            }
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('请求失败：' + e.message, 'error');
    }
}

async function testCurrentConnection() {
    if (!currentConnId) return;
    try {
        const res = await authFetch(`${API_BASE}/connections/${currentConnId}`);
        const data = await res.json();
        if (data.code === 0) {
            const conn = data.data;
            const testRes = await authFetch(`${API_BASE}/connections/test`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(conn)
            });
            const testData = await testRes.json();
            if (testData.code === 0) {
                const result = testData.data;
                if (result.success) {
                    showToast(`连接成功！延迟 ${result.latency_ms}ms`, 'success');
                } else {
                    showToast(`连接失败：${result.message}`, 'error');
                }
            }
        }
    } catch (e) {
        showToast('测试失败：' + e.message, 'error');
    }
}

function getFormData() {
    const type = document.getElementById('connType').value;
    return {
        id: document.getElementById('connId').value,
        name: document.getElementById('connName').value,
        type: type,
        host: document.getElementById('connHost').value,
        port: parseInt(document.getElementById('connPort').value) || 0,
        username: document.getElementById('connUsername').value,
        password: document.getElementById('connPassword').value,
        database: document.getElementById('connDatabase').value,
        file_path: document.getElementById('connFilePath').value,
        db_index: parseInt(document.getElementById('connDBIndex').value) || 0
    };
}

async function saveConnection() {
    const connData = getFormData();
    if (!connData.name) { showToast('请输入连接名称', 'error'); return; }
    
    const url = isEditing ? `${API_BASE}/connections/${connData.id}` : `${API_BASE}/connections`;
    const method = isEditing ? 'PUT' : 'POST';
    
    try {
        const res = await authFetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(connData)
        });
        const data = await res.json();
        if (data.code === 0) {
            showToast(isEditing ? '更新成功' : '创建成功', 'success');
            closeModal();
            loadConnections();
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('保存失败：' + e.message, 'error');
    }
}

async function deleteCurrentConnection() {
    if (!currentConnId) return;
    if (!confirm('确定要删除这个连接吗？')) return;
    try {
        const res = await authFetch(`${API_BASE}/connections/${currentConnId}`, { method: 'DELETE' });
        const data = await res.json();
        if (data.code === 0) {
            showToast('删除成功', 'success');
            currentConnId = null;
            document.getElementById('toolbar').style.display = 'none';
            document.getElementById('welcome').style.display = 'flex';
            document.getElementById('queryPanel').style.display = 'none';
            loadConnections();
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('删除失败：' + e.message, 'error');
    }
}

function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type} show`;
    setTimeout(() => { toast.classList.remove('show'); }, 2500);
}

document.getElementById('connModal').addEventListener('click', (e) => {
    if (e.target.id === 'connModal') closeModal();
});

// ================= 快捷操作 =================
function selectTable(table) {
    currentTable = table;
    loadTables(currentConnId);
    quickViewRows();
}

function getConnType() {
    return document.getElementById('currentConnType').textContent;
}

function openQuick(title) {
    document.getElementById('quickModalTitle').textContent = title;
    document.getElementById('quickModal').classList.add('show');
}

function closeQuickModal() {
    document.getElementById('quickModal').classList.remove('show');
}

function addColRow() {
    const container = document.getElementById('colDefs');
    const row = document.createElement('div');
    row.className = 'col-row';
    row.innerHTML = `
        <input class="col-name" placeholder="列名" style="width:110px">
        <input class="col-type" placeholder="类型" value="VARCHAR(255)" style="width:120px">
        <label><input type="checkbox" class="col-null"> 可空</label>
        <label><input type="checkbox" class="col-pk"> 主键</label>
        <label><input type="checkbox" class="col-auto"> 自增</label>
        <input class="col-default" placeholder="默认值" style="width:90px">
        <button class="btn btn-xs btn-danger" onclick="this.parentElement.remove()">×</button>
    `;
    container.appendChild(row);
}

function addKvRow(containerId) {
    const container = document.getElementById(containerId);
    const row = document.createElement('div');
    row.className = 'kv-row';
    row.innerHTML = `
        <input class="kv-key" placeholder="字段名" style="width:140px">
        <input class="kv-val" placeholder="值" style="width:200px">
        <button class="btn btn-xs btn-danger" onclick="this.parentElement.remove()">×</button>
    `;
    container.appendChild(row);
}

function readKv(containerId) {
    const obj = {};
    document.querySelectorAll(`#${containerId} .kv-row`).forEach(row => {
        const k = row.querySelector('.kv-key').value.trim();
        const v = row.querySelector('.kv-val').value.trim();
        if (k) obj[k] = v;
    });
    return obj;
}

function quickCreateDatabase() {
    quickMode = 'createDb';
    openQuick('新建数据库');
    document.getElementById('quickFormContainer').innerHTML = `
        <div class="form-group">
            <label>数据库名 *</label>
            <input type="text" id="qDbName" placeholder="新数据库名">
        </div>
    `;
}

function quickCreateTable() {
    quickMode = 'createTable';
    openQuick('新建表 / 集合');
    document.getElementById('quickFormContainer').innerHTML = `
        <div class="form-group">
            <label>表名 *</label>
            <input type="text" id="qTable" placeholder="表名">
        </div>
        <div class="form-group">
            <label>列定义</label>
            <div id="colDefs">
                <div class="col-row">
                    <input class="col-name" placeholder="列名" style="width:110px">
                    <input class="col-type" placeholder="类型" value="VARCHAR(255)" style="width:120px">
                    <label><input type="checkbox" class="col-null"> 可空</label>
                    <label><input type="checkbox" class="col-pk"> 主键</label>
                    <label><input type="checkbox" class="col-auto"> 自增</label>
                    <input class="col-default" placeholder="默认值" style="width:90px">
                </div>
            </div>
            <button class="btn btn-sm" style="margin-top:6px;" onclick="addColRow()">＋ 添加列</button>
        </div>
    `;
    if (getConnType() === 'Mongo') {
        document.getElementById('quickFormContainer').innerHTML = `
            <div class="form-group">
                <label>集合名 *</label>
                <input type="text" id="qTable" placeholder="集合名">
            </div>
            <p style="color:#86909c;font-size:12px;">MongoDB 集合无需预定义列，创建后可直接插入文档。</p>
        `;
    }
}

async function quickViewRows() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    try {
        let url = `${API_BASE}/quick/${currentConnId}/rows?table=${encodeURIComponent(currentTable)}&limit=100`;
        if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
            url += `&db=${encodeURIComponent(currentDb)}`;
        }
        const res = await authFetch(url);
        const data = await res.json();
        if (data.code === 0) {
            const result = data.data;
            document.getElementById('resultInfo').textContent = getConnType() === 'Redis'
                ? `共 ${result.count} 个 key`
                : `共 ${result.count} 条记录`;
            if (result.rows && result.rows.length > 0) {
                currentRows = result.rows;
                renderResultTable(result);
            } else {
                currentRows = [];
                document.getElementById('resultContent').innerHTML = '<div class="empty-result">暂无数据</div>';
            }
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('查询失败：' + e.message, 'error');
    }
}

function quickInsertRow() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    quickMode = 'insertRow';
    openQuick(`插入行 - ${currentTable}`);
    document.getElementById('quickFormContainer').innerHTML = `
        <input type="hidden" id="qTable" value="${currentTable}">
        <p style="color:#86909c;font-size:12px;margin-bottom:8px;">填写字段值，点击"添加字段"可增加更多字段</p>
        <div id="kvRows">
            <div class="kv-row">
                <input class="kv-key" placeholder="字段名" style="width:140px">
                <input class="kv-val" placeholder="值" style="width:200px">
            </div>
        </div>
        <button class="btn btn-sm" style="margin-top:6px;" onclick="addKvRow('kvRows')">＋ 添加字段</button>
    `;
}

function quickUpdateRow() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    quickMode = 'updateRow';
    openQuick(`更新行 - ${currentTable}`);
    document.getElementById('quickFormContainer').innerHTML = `
        <input type="hidden" id="qTable" value="${currentTable}">
        <div class="form-group">
            <label>要更新的字段</label>
            <div id="kvRows">
                <div class="kv-row">
                    <input class="kv-key" placeholder="字段名" style="width:140px">
                    <input class="kv-val" placeholder="新值" style="width:200px">
                </div>
            </div>
            <button class="btn btn-sm" style="margin-top:6px;" onclick="addKvRow('kvRows')">＋ 添加字段</button>
        </div>
        <div class="form-group">
            <label>条件 WHERE</label>
            <div id="kvWhere">
                <div class="kv-row">
                    <input class="kv-key" placeholder="字段名" style="width:140px">
                    <input class="kv-val" placeholder="值" style="width:200px">
                </div>
            </div>
            <button class="btn btn-sm" style="margin-top:6px;" onclick="addKvRow('kvWhere')">＋ 添加条件</button>
        </div>
    `;
}

function quickDeleteRow() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    quickMode = 'deleteRow';
    openQuick(`删除行 - ${currentTable}`);
    document.getElementById('quickFormContainer').innerHTML = `
        <input type="hidden" id="qTable" value="${currentTable}">
        <p style="color:#f53f3f;font-size:12px;margin-bottom:8px;">危险操作！请填写条件，满足条件的记录将被删除</p>
        <div class="form-group">
            <label>条件 WHERE</label>
            <div id="kvWhere">
                <div class="kv-row">
                    <input class="kv-key" placeholder="字段名" style="width:140px">
                    <input class="kv-val" placeholder="值" style="width:200px">
                </div>
            </div>
            <button class="btn btn-sm" style="margin-top:6px;" onclick="addKvRow('kvWhere')">＋ 添加条件</button>
        </div>
    `;
}

function quickCreateIndex() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    quickMode = 'createIndex';
    openQuick(`创建索引 - ${currentTable}`);
    document.getElementById('quickFormContainer').innerHTML = `
        <input type="hidden" id="qTable" value="${currentTable}">
        <div class="form-group">
            <label>索引名（可选，留空自动生成）</label>
            <input type="text" id="qIndexName" placeholder="idx_xxx">
        </div>
        <div class="form-group">
            <label>索引列 *（逗号分隔）</label>
            <input type="text" id="qIndexCols" placeholder="col1,col2">
        </div>
        <div class="form-group">
            <label><input type="checkbox" id="qIndexUnique"> 唯一索引</label>
        </div>
    `;
}

async function quickDropTable() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    if (!confirm(`确定要删除 "${currentTable}" 吗？此操作不可恢复！`)) return;
    try {
        const res = await authFetch(`${API_BASE}/quick/${currentConnId}/table?name=${encodeURIComponent(currentTable)}`, { method: 'DELETE' });
        const data = await res.json();
        if (data.code === 0) {
            showToast('删除成功', 'success');
            currentTable = null;
            loadTables(currentConnId);
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('删除失败：' + e.message, 'error');
    }
}

async function quickSubmit() {
    const qtable = document.getElementById('qTable')?.value?.trim();
    const body = {};
    let url = '';
    let method = 'POST';

    switch (quickMode) {
        case 'createDb': {
            const name = document.getElementById('qDbName').value.trim();
            if (!name) { showToast('请输入数据库名', 'warning'); return; }
            body.name = name;
            url = `${API_BASE}/quick/${currentConnId}/database`;
            break;
        }
        case 'createTable': {
            if (!qtable) { showToast('请输入表名', 'warning'); return; }
            body.table = qtable;
            if (getConnType() === 'Mongo') {
                body.columns = [{ name: '_id', type: 'ObjectId' }];
            } else {
                const cols = [];
                document.querySelectorAll('#colDefs .col-row').forEach(row => {
                    const name = row.querySelector('.col-name').value.trim();
                    const type = row.querySelector('.col-type').value.trim();
                    if (!name) return;
                    cols.push({
                        name, type,
                        nullable: row.querySelector('.col-null').checked,
                        primary_key: row.querySelector('.col-pk').checked,
                        auto_increment: row.querySelector('.col-auto').checked,
                        default_value: row.querySelector('.col-default').value.trim()
                    });
                });
                if (cols.length === 0) { showToast('至少需要一列', 'warning'); return; }
                body.columns = cols;
            }
            url = `${API_BASE}/quick/${currentConnId}/table`;
            break;
        }
        case 'insertRow': {
            body.table = qtable;
            body.data = readKv('kvRows');
            if (Object.keys(body.data).length === 0) { showToast('请至少填写一个字段', 'warning'); return; }
            url = `${API_BASE}/quick/${currentConnId}/row`;
            break;
        }
        case 'updateRow': {
            body.table = qtable;
            body.data = readKv('kvRows');
            body.where = readKv('kvWhere');
            if (Object.keys(body.data).length === 0) { showToast('请填写要更新的字段', 'warning'); return; }
            if (Object.keys(body.where).length === 0) { showToast('请填写更新条件', 'warning'); return; }
            url = `${API_BASE}/quick/${currentConnId}/row`;
            method = 'PUT';
            break;
        }
        case 'deleteRow': {
            body.table = qtable;
            body.where = readKv('kvWhere');
            if (Object.keys(body.where).length === 0) { showToast('请填写删除条件', 'warning'); return; }
            url = `${API_BASE}/quick/${currentConnId}/row`;
            method = 'DELETE';
            break;
        }
        case 'createIndex': {
            const cols = document.getElementById('qIndexCols').value.trim();
            if (!cols) { showToast('请输入索引列', 'warning'); return; }
            body.table = qtable;
            body.index_name = document.getElementById('qIndexName').value.trim();
            body.columns = cols.split(',').map(c => c.trim()).filter(Boolean);
            body.unique = document.getElementById('qIndexUnique').checked;
            url = `${API_BASE}/quick/${currentConnId}/index`;
            break;
        }
        case 'renameTable': {
            const newName = document.getElementById('qNewName').value.trim();
            if (!newName) { showToast('请输入新表名', 'warning'); return; }
            body.table = qtable;
            body.new_name = newName;
            let url2 = `${API_BASE}/quick/${currentConnId}/table/rename`;
            if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
                url2 += `?db=${encodeURIComponent(currentDb)}`;
            }
            url = url2;
            method = 'PUT';
            break;
        }
        case 'execSql': {
            const sql = document.getElementById('qSql').value.trim();
            if (!sql) { showToast('请输入SQL语句', 'warning'); return; }
            url = `${API_BASE}/connections/query`;
            body = { connection_id: currentConnId, sql, collection: '' };
            break;
        }
        case 'redisAddKey': {
            const ktype = document.getElementById('rKeyType').value;
            const key = document.getElementById('rKey').value.trim();
            const ttl = parseInt(document.getElementById('rTTL').value) || 0;
            if (!key) { showToast('请输入 Key', 'warning'); return; }
            if (ktype === 'string') {
                url = `${API_BASE}/quick/${currentConnId}/redis/key`;
                body = { key, value: document.getElementById('rValue').value, ttl };
            } else if (ktype === 'hash') {
                url = `${API_BASE}/quick/${currentConnId}/redis/hash`;
                body = { key, field: document.getElementById('rField').value, value: document.getElementById('rValue').value };
            } else if (ktype === 'list') {
                url = `${API_BASE}/quick/${currentConnId}/redis/list`;
                body = { key, value: document.getElementById('rValue').value, direction: document.getElementById('rDir').value };
            } else if (ktype === 'set') {
                url = `${API_BASE}/quick/${currentConnId}/redis/set`;
                body = { key, value: document.getElementById('rValue').value };
            } else if (ktype === 'zset') {
                url = `${API_BASE}/quick/${currentConnId}/redis/zset`;
                body = { key, member: document.getElementById('rValue').value, score: parseFloat(document.getElementById('rScore').value) || 0 };
            }
            break;
        }
        default:
            return;
    }

    try {
        const res = await authFetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        const data = await res.json();
        if (data.code === 0 && (quickMode === 'execSql' ? (data.data && data.data.success) : true)) {
            showToast(quickMode === 'execSql' ? (data.data.message || '执行成功') : (data.data || '操作成功'), 'success');
            closeQuickModal();
            if (quickMode === 'createDb' || quickMode === 'renameTable' || quickMode === 'createTable') {
                loadTables(currentConnId);
                if (quickMode === 'renameTable') currentTable = null;
            } else if (quickMode === 'execSql' && data.data && data.data.rows && data.data.rows.length > 0) {
                currentRows = data.data.rows;
                renderResultTable(data.data);
            } else {
                quickViewRows();
            }
        } else {
            showToast(data.message, 'error');
        }
    } catch (e) {
        showToast('操作失败：' + e.message, 'error');
    }
}

// ==================== 数据库优化面板 ====================
let currentOptTab = '';

function toggleOptimizePanel() {
    const panel = document.getElementById('optimizePanel');
    if (panel.style.display === 'none') {
        panel.style.display = 'block';
        renderOptimizeTabs();
    } else {
        panel.style.display = 'none';
    }
}

function renderOptimizeTabs() {
    const type = getConnType();
    const tabsDiv = document.getElementById('optimizeTabs');
    const title = document.getElementById('optimizeTitle');
    let tabs = [];
    if (type === 'Redis') {
        title.textContent = 'Redis 优化工具';
        tabs = ['keys', 'info', 'stats', 'flush'];
    } else if (type === 'Mongo') {
        title.textContent = 'MongoDB 优化工具';
        tabs = ['aggregate', 'count', 'distinct', 'collStats', 'dbStats'];
    } else if (type === 'PgSQL') {
        title.textContent = 'PostgreSQL 优化工具';
        tabs = ['schemas', 'explain', 'vacuum', 'analyze', 'reindex', 'sizes', 'sequences'];
    } else if (type === 'SQLite') {
        title.textContent = 'SQLite 优化工具';
        tabs = ['pragmas', 'vacuum', 'reindex', 'explain', 'version'];
    } else {
        title.textContent = 'MySQL 优化工具';
        tabs = ['explain', 'analyze', 'sizes', 'processes'];
    }
    const labels = {
        keys: '📋 Keys管理', info: 'ℹ️ 服务器信息', stats: '📊 DB统计', flush: '🗑 清空DB',
        aggregate: '🔗 聚合管道', count: '🔢 文档统计', distinct: '🎯 去重查询', collStats: '📈 集合统计', dbStats: '💾 数据库统计',
        schemas: '📁 Schema管理', explain: '🔍 执行计划', vacuum: '🧹 VACUUM', analyze: '📊 ANALYZE', reindex: '🔄 REINDEX', sizes: '📏 表大小', sequences: '🔢 序列',
        pragmas: '⚙️ PRAGMA管理', version: '📌 版本信息',
        processes: '🔌 进程列表'
    };
    tabsDiv.innerHTML = tabs.map(t => `<div class="optimize-tab ${t === currentOptTab ? 'active' : ''}" onclick="selectOptTab('${t}')">${labels[t] || t}</div>`).join('');
    if (tabs.length > 0 && !currentOptTab) selectOptTab(tabs[0]);
    else if (currentOptTab) selectOptTab(currentOptTab);
}

function selectOptTab(tab) {
    currentOptTab = tab;
    renderOptimizeTabsActive(tab);
    const content = document.getElementById('optimizeContent');
    content.innerHTML = '';
    const renderers = {
        keys: renderRedisKeys, info: renderRedisInfo, stats: renderRedisStats, flush: renderRedisFlush,
        aggregate: renderMongoAggregate, count: renderMongoCount, distinct: renderMongoDistinct,
        collStats: renderMongoCollStats, dbStats: renderMongoDbStats,
        schemas: renderPGSchemas, explain: renderExplain, vacuum: renderPGVacuum, analyze: renderPGAnalyze,
        reindex: renderPGReindex, sizes: renderPGSizes, sequences: renderPGSequences,
        pragmas: renderSQLitePragmas, version: renderSQLiteVersion,
        processes: renderMySQLProcesses
    };
    if (renderers[tab]) renderers[tab]();
}

function renderOptimizeTabsActive(activeTab) {
    const type = getConnType();
    const tabsDiv = document.getElementById('optimizeTabs');
    let tabs = [];
    if (type === 'Redis') tabs = ['keys', 'info', 'stats', 'flush'];
    else if (type === 'Mongo') tabs = ['aggregate', 'count', 'distinct', 'collStats', 'dbStats'];
    else if (type === 'PgSQL') tabs = ['schemas', 'explain', 'vacuum', 'analyze', 'reindex', 'sizes', 'sequences'];
    else if (type === 'SQLite') tabs = ['pragmas', 'vacuum', 'reindex', 'explain', 'version'];
    else tabs = ['explain', 'analyze', 'sizes', 'processes'];
    const labels = {
        keys: '📋 Keys管理', info: 'ℹ️ 服务器信息', stats: '📊 DB统计', flush: '🗑 清空DB',
        aggregate: '🔗 聚合管道', count: '🔢 文档统计', distinct: '🎯 去重查询', collStats: '📈 集合统计', dbStats: '💾 数据库统计',
        schemas: '📁 Schema管理', explain: '🔍 执行计划', vacuum: '🧹 VACUUM', analyze: '📊 ANALYZE', reindex: '🔄 REINDEX', sizes: '📏 表大小', sequences: '🔢 序列',
        pragmas: '⚙️ PRAGMA管理', version: '📌 版本信息',
        processes: '🔌 进程列表'
    };
    tabsDiv.innerHTML = tabs.map(t => `<div class="optimize-tab ${t === activeTab ? 'active' : ''}" onclick="selectOptTab('${t}')">${labels[t] || t}</div>`).join('');
}

// ---- Redis ----
async function renderRedisKeys() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-row">
            <div class="form-group">
                <label>匹配模式</label>
                <input type="text" id="redisPattern" value="*" placeholder="如 user:*">
            </div>
            <div class="form-group" style="flex:none;align-self:flex-end;">
                <button class="btn btn-primary btn-sm" onclick="loadRedisKeys()">查询</button>
                <button class="btn btn-sm" onclick="openRedisAddKey()">➕ 添加Key</button>
            </div>
        </div>
        <div id="redisKeysResult" class="opt-result">加载中...</div>
    `;
    loadRedisKeys();
}

async function loadRedisKeys() {
    const pattern = document.getElementById('redisPattern').value || '*';
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/keys?pattern=${encodeURIComponent(pattern)}`);
    const data = await res.json();
    const div = document.getElementById('redisKeysResult');
    if (data.code !== 0) { div.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const keys = data.data.keys || [];
    if (keys.length === 0) { div.innerHTML = '暂无 Key'; return; }
    div.innerHTML = `<div style="margin-bottom:8px;color:#86909c;">共 ${data.data.total} 个 Key（显示前 ${keys.length} 个）</div>` +
        keys.map(k => `
        <div style="display:flex;gap:8px;padding:6px 0;border-bottom:1px solid #f2f3f5;align-items:center;">
            <span style="font-family:Consolas,monospace;flex:1;cursor:pointer;color:#3370ff;" onclick="viewRedisKey('${k.key.replace(/'/g,"\\'")}')">${k.key}</span>
            <span class="status-badge status-active" style="font-size:11px;">${k.type}</span>
            <span style="color:#86909c;font-size:11px;">${k.ttl}</span>
            <button class="btn btn-xs btn-danger" onclick="deleteRedisKey('${k.key.replace(/'/g,"\\'")}')">删除</button>
        </div>`).join('');
}

async function viewRedisKey(key) {
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/key?key=${encodeURIComponent(key)}`);
    const data = await res.json();
    if (data.code !== 0) { showToast(data.message, 'error'); return; }
    const d = data.data;
    let valHtml = '';
    if (typeof d.value === 'object') {
        valHtml = JSON.stringify(d.value, null, 2);
    } else {
        valHtml = String(d.value);
    }
    const ttlHint = d.ttl === -1 ? '永久' : (d.ttl + '秒');
    const div = document.getElementById('redisKeysResult');
    div.innerHTML = `
        <div style="margin-bottom:8px;"><b>${d.key}</b> <span class="status-badge status-active">${d.type}</span> TTL: ${ttlHint}</div>
        <div class="form-row">
            <div class="form-group"><label>设置TTL（秒，-1永久）</label>
                <input type="number" id="redisTTL" value="${d.ttl}" style="width:120px;">
                <button class="btn btn-sm" style="margin-top:4px;" onclick="setRedisTTL('${d.key.replace(/'/g,"\\'")}')">应用</button>
            </div>
        </div>
        <pre class="opt-result">${valHtml}</pre>
    `;
}

async function setRedisTTL(key) {
    const ttl = parseInt(document.getElementById('redisTTL').value);
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/ttl`, {
        method: 'PUT', headers: {'Content-Type':'application/json'},
        body: JSON.stringify({key, ttl})
    });
    const data = await res.json();
    showToast(data.data || data.message, data.code === 0 ? 'success' : 'error');
    if (data.code === 0) loadRedisKeys();
}

async function deleteRedisKey(key) {
    if (!confirm('确定删除 Key: ' + key + ' ?')) return;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/key?key=${encodeURIComponent(key)}`, {method:'DELETE'});
    const data = await res.json();
    showToast(data.data || data.message, data.code === 0 ? 'success' : 'error');
    if (data.code === 0) loadRedisKeys();
}

function openRedisAddKey() {
    quickMode = 'redisAddKey';
    openQuick('添加 Redis Key');
    document.getElementById('quickFormContainer').innerHTML = `
        <div class="form-group"><label>Key 类型</label>
            <select id="rKeyType" onchange="onRedisKeyTypeChange()">
                <option value="string">String</option>
                <option value="hash">Hash</option>
                <option value="list">List</option>
                <option value="set">Set</option>
                <option value="zset">ZSet</option>
            </select>
        </div>
        <div class="form-group"><label>Key</label><input type="text" id="rKey" placeholder="key name"></div>
        <div class="form-group"><label>TTL（秒，0=永久）</label><input type="number" id="rTTL" value="0"></div>
        <div id="rValueFields">
            <div class="form-group"><label>Value</label><input type="text" id="rValue" placeholder="value"></div>
        </div>
    `;
}

function onRedisKeyTypeChange() {
    const t = document.getElementById('rKeyType').value;
    const fields = document.getElementById('rValueFields');
    if (t === 'hash') {
        fields.innerHTML = `<div class="form-group"><label>Field</label><input type="text" id="rField" placeholder="field"></div>
            <div class="form-group"><label>Value</label><input type="text" id="rValue" placeholder="value"></div>`;
    } else if (t === 'list') {
        fields.innerHTML = `<div class="form-group"><label>方向</label>
            <select id="rDir"><option value="right">右侧追加 (RPUSH)</option><option value="left">左侧插入 (LPUSH)</option></select></div>
            <div class="form-group"><label>Value</label><input type="text" id="rValue" placeholder="value"></div>`;
    } else if (t === 'set') {
        fields.innerHTML = `<div class="form-group"><label>Member</label><input type="text" id="rValue" placeholder="member"></div>`;
    } else if (t === 'zset') {
        fields.innerHTML = `<div class="form-group"><label>Member</label><input type="text" id="rValue" placeholder="member"></div>
            <div class="form-group"><label>Score</label><input type="number" step="0.01" id="rScore" value="0"></div>`;
    } else {
        fields.innerHTML = `<div class="form-group"><label>Value</label><input type="text" id="rValue" placeholder="value"></div>`;
    }
}

async function renderRedisInfo() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = '<div class="opt-result">加载中...</div>';
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/info`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const info = data.data;
    c.innerHTML = '<div class="pragma-grid">' +
        Object.entries(info).slice(0, 60).map(([k,v]) => `<div class="pragma-item"><div class="pragma-name">${k}</div><div class="pragma-val">${v}</div></div>`).join('')
        + '</div>';
}

async function renderRedisStats() {
    const c = document.getElementById('optimizeContent');
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/dbsize`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    c.innerHTML = `<div class="opt-result">当前数据库 Key 数量：<b>${data.data.dbsize}</b></div>`;
}

function renderRedisFlush() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div style="color:#f53f3f;margin-bottom:12px;">⚠️ 警告：此操作将清空当前数据库的所有 Key，不可恢复！</div>
        <button class="btn btn-danger" onclick="doRedisFlush()">确认清空当前数据库</button>
    `;
}

async function doRedisFlush() {
    if (!confirm('确定清空当前数据库所有 Key？此操作不可恢复！')) return;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/redis/flushdb`, {method:'POST'});
    const data = await res.json();
    showToast(data.data || data.message, data.code === 0 ? 'success' : 'error');
}

// ---- MongoDB ----
function renderMongoAggregate() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-group"><label>集合名</label><input type="text" id="mAggColl" value="${currentTable||''}" placeholder="collection"></div>
        <div class="form-group"><label>聚合管道（JSON数组）</label>
            <textarea id="mAggPipe" placeholder='[{"$match":{"status":"active"}},{"$group":{"_id":"$category","count":{"$sum":1}}}]'></textarea>
        </div>
        <button class="btn btn-primary" onclick="runMongoAggregate()">执行聚合</button>
        <div id="mAggResult" class="opt-result"></div>
    `;
}

async function runMongoAggregate() {
    const coll = document.getElementById('mAggColl').value;
    const pipeText = document.getElementById('mAggPipe').value;
    let pipeline;
    try { pipeline = JSON.parse(pipeText); } catch(e) { showToast('管道JSON格式错误', 'error'); return; }
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/mongo/aggregate`, {
        method:'POST', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({collection: coll, pipeline})
    });
    const data = await res.json();
    const div = document.getElementById('mAggResult');
    if (data.code !== 0) { div.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    div.textContent = JSON.stringify(data.data, null, 2);
}

async function renderMongoCount() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-row">
            <div class="form-group"><label>集合名</label><input type="text" id="mCountColl" value="${currentTable||''}"></div>
            <div class="form-group" style="flex:none;align-self:flex-end;"><button class="btn btn-primary" onclick="runMongoCount()">统计</button></div>
        </div>
        <div id="mCountResult" class="opt-result"></div>
    `;
}

async function runMongoCount() {
    const coll = document.getElementById('mCountColl').value;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/mongo/count?collection=${encodeURIComponent(coll)}`);
    const data = await res.json();
    const div = document.getElementById('mCountResult');
    if (data.code !== 0) { div.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    div.innerHTML = `集合 <b>${data.data.collection}</b> 文档数量：<b>${data.data.count}</b>`;
}

function renderMongoDistinct() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-row">
            <div class="form-group"><label>集合名</label><input type="text" id="mDistColl" value="${currentTable||''}"></div>
            <div class="form-group"><label>字段名</label><input type="text" id="mDistField" placeholder="field"></div>
            <div class="form-group" style="flex:none;align-self:flex-end;"><button class="btn btn-primary" onclick="runMongoDistinct()">查询</button></div>
        </div>
        <div id="mDistResult" class="opt-result"></div>
    `;
}

async function runMongoDistinct() {
    const coll = document.getElementById('mDistColl').value;
    const field = document.getElementById('mDistField').value;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/mongo/distinct?collection=${encodeURIComponent(coll)}&field=${encodeURIComponent(field)}`);
    const data = await res.json();
    const div = document.getElementById('mDistResult');
    if (data.code !== 0) { div.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    div.textContent = JSON.stringify(data.data.values, null, 2);
}

async function renderMongoCollStats() {
    const c = document.getElementById('optimizeContent');
    if (!currentTable) { c.innerHTML = '请先在左侧选择一个集合'; return; }
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/mongo/stats/collection?collection=${encodeURIComponent(currentTable)}`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const s = data.data;
    c.innerHTML = '<div class="pragma-grid">' +
        Object.entries(s).map(([k,v]) => `<div class="pragma-item"><div class="pragma-name">${k}</div><div class="pragma-val">${typeof v==='object'?JSON.stringify(v):v}</div></div>`).join('')
        + '</div>';
}

async function renderMongoDbStats() {
    const c = document.getElementById('optimizeContent');
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/mongo/stats/database`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const s = data.data;
    c.innerHTML = '<div class="pragma-grid">' +
        Object.entries(s).map(([k,v]) => `<div class="pragma-item"><div class="pragma-name">${k}</div><div class="pragma-val">${typeof v==='object'?JSON.stringify(v):v}</div></div>`).join('')
        + '</div>';
}

// ---- PostgreSQL ----
async function renderPGSchemas() {
    const c = document.getElementById('optimizeContent');
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/schemas`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const schemas = data.data.rows || [];
    c.innerHTML = `
        <div class="form-row">
            <div class="form-group"><label>新建 Schema</label><input type="text" id="pgSchemaName" placeholder="schema name"></div>
            <div class="form-group" style="flex:none;align-self:flex-end;"><button class="btn btn-primary" onclick="pgCreateSchema()">创建</button></div>
        </div>
        <div id="pgSchemaList"></div>
    `;
    document.getElementById('pgSchemaList').innerHTML = schemas.map(s => `
        <div style="display:flex;gap:8px;padding:6px 0;border-bottom:1px solid #f2f3f5;align-items:center;">
            <span style="flex:1;font-family:Consolas,monospace;">${s.schema_name}</span>
            <button class="btn btn-xs btn-danger" onclick="pgDropSchema('${s.schema_name}')">删除</button>
        </div>`).join('');
}

async function pgCreateSchema() {
    const name = document.getElementById('pgSchemaName').value;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/schema`, {
        method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({name})
    });
    const data = await res.json();
    showToast(data.data || data.message, data.code===0?'success':'error');
    if (data.code === 0) renderPGSchemas();
}

async function pgDropSchema(name) {
    if (!confirm('确定删除 Schema: ' + name + ' ?（CASCADE）')) return;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/schema?name=${encodeURIComponent(name)}`, {method:'DELETE'});
    const data = await res.json();
    showToast(data.data || data.message, data.code===0?'success':'error');
    if (data.code === 0) renderPGSchemas();
}

function renderExplain() {
    const type = getConnType();
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-group"><label>SQL 语句</label><textarea id="explainSQL" placeholder="SELECT * FROM ..."></textarea></div>
        <button class="btn btn-primary" onclick="runExplain()">分析执行计划</button>
        <div id="explainResult" class="opt-result"></div>
    `;
}

async function runExplain() {
    const sql = document.getElementById('explainSQL').value;
    const type = getConnType();
    let url, body;
    if (type === 'PgSQL') {
        url = `${API_BASE}/quick/${currentConnId}/pg/explain`;
        body = {sql};
    } else if (type === 'SQLite') {
        url = `${API_BASE}/quick/${currentConnId}/sqlite/explain`;
        body = {sql};
    } else {
        // MySQL 使用通用查询接口执行 EXPLAIN
        url = `${API_BASE}/connections/query`;
        body = {connection_id: currentConnId, sql: 'EXPLAIN ' + sql, collection: ''};
    }
    const res = await authFetch(url, {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
    const data = await res.json();
    const div = document.getElementById('explainResult');
    if (data.code !== 0) { div.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    if (type === 'MySQL') {
        if (data.data && data.data.rows) {
            currentRows = data.data.rows;
            div.innerHTML = '';
            renderResultTable(data.data);
            div.innerHTML = document.getElementById('resultContent').innerHTML;
        } else {
            div.textContent = data.data.message || JSON.stringify(data.data);
        }
    } else {
        div.textContent = JSON.stringify(data.data, null, 2);
    }
}

function renderPGVacuum() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-group"><label>表名（留空=所有表）</label><input type="text" id="pgVacTable" placeholder="table_name"></div>
        <div class="form-row">
            <label><input type="checkbox" id="pgVacFull"> FULL（回收空间，锁表）</label>
            <label style="margin-left:16px;"><input type="checkbox" id="pgVacAnalyze"> ANALYZE（更新统计）</label>
        </div>
        <button class="btn btn-primary" onclick="runPGVacuum()">执行 VACUUM</button>
        <div id="pgVacResult" class="opt-result"></div>
    `;
}

async function runPGVacuum() {
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/vacuum`, {
        method:'POST', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({table: document.getElementById('pgVacTable').value, full: document.getElementById('pgVacFull').checked, analyze: document.getElementById('pgVacAnalyze').checked})
    });
    const data = await res.json();
    document.getElementById('pgVacResult').textContent = data.data || data.message;
}

function renderPGAnalyze() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-group"><label>表名（留空=所有表）</label><input type="text" id="pgAnaTable" placeholder="table_name"></div>
        <button class="btn btn-primary" onclick="runPGAnalyze()">执行 ANALYZE</button>
        <div id="pgAnaResult" class="opt-result"></div>
    `;
}

async function runPGAnalyze() {
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/analyze`, {
        method:'POST', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({table: document.getElementById('pgAnaTable').value})
    });
    const data = await res.json();
    document.getElementById('pgAnaResult').textContent = data.data || data.message;
}

function renderPGReindex() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = `
        <div class="form-group"><label>表名（留空=整个数据库）</label><input type="text" id="pgReTable" placeholder="table_name"></div>
        <button class="btn btn-primary" onclick="runPGReindex()">执行 REINDEX</button>
        <div id="pgReResult" class="opt-result"></div>
    `;
}

async function runPGReindex() {
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/reindex`, {
        method:'POST', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({table: document.getElementById('pgReTable').value})
    });
    const data = await res.json();
    document.getElementById('pgReResult').textContent = data.data || data.message;
}

async function renderPGSizes() {
    const c = document.getElementById('optimizeContent');
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/sizes`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const rows = data.data.rows || [];
    c.innerHTML = `<table class="admin-table"><thead><tr><th>表名</th><th>总大小</th><th>表大小</th><th>索引大小</th><th>行数</th></tr></thead><tbody>` +
        rows.map(r => `<tr><td>${r.table_name}</td><td>${r.total_size}</td><td>${r.table_size}</td><td>${r.index_size}</td><td>${r.row_count}</td></tr>`).join('') +
        `</tbody></table>`;
}

async function renderPGSequences() {
    const c = document.getElementById('optimizeContent');
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/pg/sequences`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const rows = data.data.rows || [];
    c.innerHTML = `<table class="admin-table"><thead><tr><th>Schema</th><th>序列名</th><th>类型</th><th>起始值</th><th>步长</th></tr></thead><tbody>` +
        rows.map(r => `<tr><td>${r.sequence_schema}</td><td>${r.sequence_name}</td><td>${r.data_type}</td><td>${r.start_value}</td><td>${r.increment}</td></tr>`).join('') +
        `</tbody></table>`;
}

// ---- SQLite ----
async function renderSQLitePragmas() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = '<div class="opt-result">加载中...</div>';
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/sqlite/pragmas`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    const pragmas = data.data;
    c.innerHTML = '<div class="pragma-grid">' +
        Object.entries(pragmas).map(([k,v]) => `
            <div class="pragma-item">
                <div class="pragma-name">${k}</div>
                <input class="pragma-val" value="${v}" id="pragma_${k}" style="border:1px solid #e5e6eb;border-radius:4px;padding:2px 6px;width:100%;box-sizing:border-box;">
                <button class="btn btn-xs" style="margin-top:4px;width:100%;" onclick="setSQLitePragma('${k}')">设置</button>
            </div>`).join('') + '</div>';
}

async function setSQLitePragma(name) {
    const value = document.getElementById('pragma_' + name).value;
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/sqlite/pragma`, {
        method:'PUT', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({name, value})
    });
    const data = await res.json();
    showToast(data.data || data.message, data.code===0?'success':'error');
}

async function renderSQLiteVersion() {
    const c = document.getElementById('optimizeContent');
    const res = await authFetch(`${API_BASE}/quick/${currentConnId}/sqlite/version`);
    const data = await res.json();
    if (data.code !== 0) { c.innerHTML = '<span style="color:#f53f3f;">' + data.message + '</span>'; return; }
    c.innerHTML = `<div class="opt-result">SQLite 版本：<b>${data.data.rows[0].version}</b></div>`;
}

// ---- MySQL (补充) ----
async function renderMySQLProcesses() {
    const c = document.getElementById('optimizeContent');
    c.innerHTML = '<div class="opt-result">加载中...</div>';
    const res = await authFetch(`${API_BASE}/connections/query`, {
        method:'POST', headers:{'Content-Type':'application/json'},
        body: JSON.stringify({connection_id: currentConnId, sql: 'SHOW PROCESSLIST', collection: ''})
    });
    const data = await res.json();
    if (data.code !== 0 || !data.data.success) {
        c.innerHTML = '<span style="color:#f53f3f;">' + (data.data?.message || data.message) + '</span>';
        return;
    }
    if (data.data.rows && data.data.rows.length > 0) {
        currentRows = data.data.rows;
        c.innerHTML = '';
        const tempDiv = document.createElement('div');
        document.getElementById('resultContent').innerHTML = '';
        renderResultTable(data.data);
        c.innerHTML = document.getElementById('resultContent').innerHTML;
    } else {
        c.innerHTML = '<div class="opt-result">无活动进程</div>';
    }
}
