// API 基础地址
const API_BASE = '/api';

// 当前选中的连接
let currentConnId = null;
let currentTable = null;
let currentDb = '';
let currentRows = [];
let isEditing = false;
let quickMode = '';

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', () => {
    loadConnections();
    
    // Ctrl+Enter 执行查询
    document.getElementById('sqlEditor').addEventListener('keydown', (e) => {
        if (e.ctrlKey && e.key === 'Enter') {
            executeQuery();
        }
    });
});

// 加载连接列表
async function loadConnections() {
    try {
        const res = await fetch(`${API_BASE}/connections`);
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
    
    // 同步顶部快速切换下拉
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
    const labels = {
        mysql: 'MySQL',
        postgres: 'PgSQL',
        sqlite: 'SQLite',
        mongodb: 'Mongo',
        redis: 'Redis'
    };
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
    loadConnections(); // 更新选中状态
    
    // 重置右侧内容（快速切换语义：清空旧连接残留）
    document.getElementById('sqlEditor').value = '';
    document.getElementById('resultInfo').textContent = '执行结果';
    document.getElementById('resultContent').innerHTML = '<div class="empty-result">正在加载连接...</div>';
    
    try {
        const res = await fetch(`${API_BASE}/connections/${id}`);
        const data = await res.json();
        if (data.code === 0) {
            const conn = data.data;
            document.getElementById('currentConnName').textContent = conn.name;
            document.getElementById('currentConnType').textContent = getTypeLabel(conn.type);
            document.getElementById('toolbar').style.display = 'flex';
            document.getElementById('welcome').style.display = 'none';
            document.getElementById('queryPanel').style.display = 'flex';
            
            // MongoDB显示集合输入框
            const collectionInput = document.getElementById('collectionInput');
            if (conn.type === 'mongodb') {
                collectionInput.style.display = 'inline-block';
            } else {
                collectionInput.style.display = 'none';
            }
            
            // 快捷操作按钮可见性
            document.getElementById('btnNewDb').style.display = (conn.type === 'mysql' || conn.type === 'postgres') ? 'inline-block' : 'none';
            document.getElementById('btnNewTable').style.display = (conn.type === 'redis') ? 'none' : 'inline-block';
            currentTable = null;
            
            // 连接加载完成，右侧占位
            document.getElementById('resultInfo').textContent = '执行结果';
            document.getElementById('resultContent').innerHTML = '<div class="empty-result">执行查询后显示结果</div>';
            
            // 加载数据库列表（MySQL/PostgreSQL）
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
        const res = await fetch(`${API_BASE}/quick/${connId}/databases`);
        const data = await res.json();
        if (data.code === 0) {
            const dbs = data.data;
            if (dbs.length === 0) {
                select.innerHTML = '<option value="">(无)</option>';
                currentDb = '';
            } else {
                select.innerHTML = dbs.map(db => `<option value="${db}">${db}</option>`).join('');
                // 默认选中连接配置的数据库，否则选第一个
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

// 切换数据库
function onDbChange() {
    currentDb = document.getElementById('dbSelect').value;
    currentTable = null;
    loadTables(currentConnId);
}

// 加载表列表
async function loadTables(connId) {
    const tableList = document.getElementById('tableList');
    tableList.innerHTML = '<div class="loading">加载中...</div>';
    
    try {
        let url = `${API_BASE}/connections/${connId}/tables`;
        if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
            url = `${API_BASE}/quick/${connId}/tables?db=${encodeURIComponent(currentDb)}`;
        }
        const res = await fetch(url);
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

// 刷新表列表
function refreshTables() {
    if (currentConnId) {
        loadTables(currentConnId);
    }
}

// 插入表名到SQL编辑器
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

// 执行查询
async function executeQuery() {
    if (!currentConnId) {
        showToast('请先选择一个连接', 'warning');
        return;
    }
    
    const sql = document.getElementById('sqlEditor').value.trim();
    if (!sql) {
        showToast('请输入查询语句', 'warning');
        return;
    }
    
    const collection = document.getElementById('collectionInput').value.trim();
    const resultContent = document.getElementById('resultContent');
    const resultInfo = document.getElementById('resultInfo');
    
    resultInfo.textContent = '执行中...';
    resultContent.innerHTML = '<div class="empty-result">执行中...</div>';
    
    const startTime = Date.now();
    
    try {
        const res = await fetch(`${API_BASE}/connections/query`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                connection_id: currentConnId,
                sql: sql,
                collection: collection
            })
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

// 渲染结果表格
function renderResultTable(result) {
    const resultContent = document.getElementById('resultContent');
    const columns = result.columns || Object.keys(result.rows[0] || {});
    // 仅 SQL 类数据库显示行操作列
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

// SQL值转义
function quoteSqlVal(v) {
    if (v === null || v === undefined) return 'NULL';
    if (typeof v === 'number') return String(v);
    return "'" + String(v).replace(/'/g, "''") + "'";
}

// 获取当前连接信息
function getCurrentConn() {
    const items = [...document.querySelectorAll('.conn-item')];
    const active = items.find(i => i.classList.contains('active'));
    return active ? { name: active.textContent.trim().split('\n')[1].trim() } : null;
}

// 获取表列信息（含主键）
async function getTableColumns(table) {
    let url = `${API_BASE}/quick/${currentConnId}/columns?table=${encodeURIComponent(table)}`;
    if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
        url += `&db=${encodeURIComponent(currentDb)}`;
    }
    const res = await fetch(url);
    const data = await res.json();
    if (data.code !== 0) throw new Error(data.message);
    const rows = data.data.rows || [];
    // 提取主键列
    let pk = null;
    for (const row of rows) {
        if (row.Key === 'PRI' || (typeof row.pk === 'number' && row.pk > 0)) {
            pk = row.Field || row.name;
            break;
        }
    }
    return { columns: rows, pk };
}

// 编辑行（弹出UPDATE语句，主键为条件）
async function editRow(idx) {
    const row = currentRows[idx];
    if (!row) return;
    const table = currentTable;
    try {
        const { columns, pk } = await getTableColumns(table);
        if (!pk) {
            showToast('该表无主键，无法生成更新条件', 'warning');
            return;
        }
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

// 删除行（二次确认）
async function deleteRow(idx) {
    const row = currentRows[idx];
    if (!row) return;
    const table = currentTable;
    try {
        const { pk } = await getTableColumns(table);
        if (!pk) {
            showToast('该表无主键，无法生成删除条件', 'warning');
            return;
        }
        const where = `\`${pk}\` = ${quoteSqlVal(row[pk])}`;
        if (!confirm(`确定要删除该记录吗？\n\nDELETE FROM ${table} WHERE ${where}`)) return;
        const sql = `DELETE FROM \`${table}\` WHERE ${where};`;
        const res = await fetch(`${API_BASE}/connections/query`, {
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

// 表改名
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

// 格式化值
function formatValue(val) {
    if (val === null || val === undefined) return '<span style="color: #86909c;">NULL</span>';
    if (typeof val === 'object') return JSON.stringify(val);
    return String(val).length > 200 ? String(val).substring(0, 200) + '...' : String(val);
}

// 打开添加弹窗
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

// 编辑连接
async function editConnection(id) {
    try {
        const res = await fetch(`${API_BASE}/connections/${id}`);
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
            document.getElementById('connPassword').value = ''; // 密码不回填
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

// 类型改变时调整表单
function onTypeChange() {
    const type = document.getElementById('connType').value;
    const hostPortRow = document.getElementById('hostPortRow');
    const dbGroup = document.getElementById('dbGroup');
    const fileGroup = document.getElementById('fileGroup');
    const dbIndexGroup = document.getElementById('dbIndexGroup');
    
    // 默认端口
    const ports = {
        mysql: 3306,
        postgres: 5432,
        sqlite: 0,
        mongodb: 27017,
        redis: 6379
    };
    if (ports[type]) {
        document.getElementById('connPort').value = ports[type];
    }
    
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

// 关闭弹窗
function closeModal() {
    document.getElementById('connModal').classList.remove('show');
}

// 测试连接
async function testConnection() {
    const connData = getFormData();
    
    try {
        const res = await fetch(`${API_BASE}/connections/test`, {
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

// 测试当前连接
async function testCurrentConnection() {
    if (!currentConnId) return;
    
    try {
        const res = await fetch(`${API_BASE}/connections/${currentConnId}`);
        const data = await res.json();
        if (data.code === 0) {
            const conn = data.data;
            const testRes = await fetch(`${API_BASE}/connections/test`, {
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

// 获取表单数据
function getFormData() {
    const type = document.getElementById('connType').value;
    const data = {
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
    return data;
}

// 保存连接
async function saveConnection() {
    const connData = getFormData();
    
    if (!connData.name) {
        showToast('请输入连接名称', 'error');
        return;
    }
    
    const url = isEditing ? `${API_BASE}/connections/${connData.id}` : `${API_BASE}/connections`;
    const method = isEditing ? 'PUT' : 'POST';
    
    try {
        const res = await fetch(url, {
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

// 删除当前连接
async function deleteCurrentConnection() {
    if (!currentConnId) return;
    
    if (!confirm('确定要删除这个连接吗？')) return;
    
    try {
        const res = await fetch(`${API_BASE}/connections/${currentConnId}`, {
            method: 'DELETE'
        });
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

// 显示Toast提示
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type} show`;
    setTimeout(() => {
        toast.classList.remove('show');
    }, 2500);
}

// 点击弹窗外部关闭
document.getElementById('connModal').addEventListener('click', (e) => {
    if (e.target.id === 'connModal') {
        closeModal();
    }
});

// ================= 快捷操作 =================

// 选择表
function selectTable(table) {
    currentTable = table;
    loadTables(currentConnId);
    quickViewRows();
}

// 获取当前连接类型
function getConnType() {
    return document.getElementById('currentConnType').textContent;
}

// 打开快捷操作弹窗
function openQuick(title) {
    document.getElementById('quickModalTitle').textContent = title;
    document.getElementById('quickModal').classList.add('show');
}

function closeQuickModal() {
    document.getElementById('quickModal').classList.remove('show');
}

// 动态添加列定义行
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

// 动态添加键值对
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

// 读取键值对
function readKv(containerId) {
    const obj = {};
    document.querySelectorAll(`#${containerId} .kv-row`).forEach(row => {
        const k = row.querySelector('.kv-key').value.trim();
        const v = row.querySelector('.kv-val').value.trim();
        if (k) obj[k] = v;
    });
    return obj;
}

// 新建数据库
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

// 新建表
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

// 查看数据
async function quickViewRows() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    try {
        let url = `${API_BASE}/quick/${currentConnId}/rows?table=${encodeURIComponent(currentTable)}&limit=100`;
        if ((getConnType() === 'MySQL' || getConnType() === 'PgSQL') && currentDb) {
            url += `&db=${encodeURIComponent(currentDb)}`;
        }
        const res = await fetch(url);
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

// 插入行
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

// 更新行
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

// 删除行
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

// 创建索引
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

// 删除表
async function quickDropTable() {
    if (!currentTable) { showToast('请先选择一个表', 'warning'); return; }
    if (!confirm(`确定要删除 "${currentTable}" 吗？此操作不可恢复！`)) return;
    try {
        const res = await fetch(`${API_BASE}/quick/${currentConnId}/table?name=${encodeURIComponent(currentTable)}`, {
            method: 'DELETE'
        });
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

// 提交快捷操作
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
                        name,
                        type,
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
        default:
            return;
    }

    try {
        const res = await fetch(url, {
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
