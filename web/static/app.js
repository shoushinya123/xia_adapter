// API 基础 URL
const API_BASE = '/api/v1';

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', () => {
    loadConfig();
    loadStatus();
    setupTabs();
    setupForm();
    
    // 每5秒刷新一次状态
    setInterval(loadStatus, 5000);
});

// 设置标签页切换
function setupTabs() {
    const tabBtns = document.querySelectorAll('.tab-btn');
    const tabContents = document.querySelectorAll('.tab-content');

    tabBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            const targetTab = btn.getAttribute('data-tab');

            // 移除所有活动状态
            tabBtns.forEach(b => b.classList.remove('active'));
            tabContents.forEach(c => c.classList.remove('active'));

            // 添加活动状态
            btn.classList.add('active');
            document.getElementById(`${targetTab}-tab`).classList.add('active');
        });
    });
}

// 设置表单
function setupForm() {
    const form = document.getElementById('config-form');
    const reloadBtn = document.getElementById('reload-btn');

    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        await saveConfig();
    });

    reloadBtn.addEventListener('click', () => {
        loadConfig();
        showMessage('配置已重新加载', 'success');
    });
}

// 加载配置
async function loadConfig() {
    try {
        const response = await fetch(`${API_BASE}/config`);
        const result = await response.json();

        if (result.success) {
            const config = result.data;
            fillForm(config);
        } else {
            showMessage('加载配置失败: ' + result.error, 'error');
        }
    } catch (error) {
        showMessage('加载配置失败: ' + error.message, 'error');
    }
}

// 填充表单（兼容大小写）
function fillForm(config) {
    // 兼容大小写字段名
    const server = config.Server || config.server || {};
    const platform = config.Platform || config.platform || {};
    const agent = config.Agent || config.agent || {};
    const lark = platform.Lark || platform.lark || {};
    const wecom = platform.WeCom || platform.wecom || {};
    const dify = agent.Dify || agent.dify || {};
    const coze = agent.Coze || agent.coze || {};

    // 服务器配置
    document.getElementById('server-host').value = server.Host || server.host || '';
    document.getElementById('server-port').value = server.Port || server.port || '';

    // 飞书配置
    document.getElementById('lark-enabled').checked = lark.Enabled !== undefined ? lark.Enabled : (lark.enabled || false);
    document.getElementById('lark-app-id').value = lark.AppID || lark.app_id || '';
    document.getElementById('lark-app-secret').value = lark.AppSecret || lark.app_secret || '';
    document.getElementById('lark-domain').value = lark.Domain || lark.domain || 'feishu.cn';
    document.getElementById('lark-bot-name').value = lark.BotName || lark.bot_name || '';

    // 企微配置
    document.getElementById('wecom-enabled').checked = wecom.Enabled !== undefined ? wecom.Enabled : (wecom.enabled || false);
    document.getElementById('wecom-corp-id').value = wecom.CorpID || wecom.corp_id || '';
    document.getElementById('wecom-secret').value = wecom.Secret || wecom.secret || '';
    document.getElementById('wecom-token').value = wecom.Token || wecom.token || '';
    document.getElementById('wecom-aes-key').value = wecom.EncodingAESKey || wecom.encoding_aes_key || '';
    document.getElementById('wecom-host').value = wecom.Host || wecom.host || '';
    document.getElementById('wecom-port').value = wecom.Port || wecom.port || '';
    document.getElementById('wecom-agent-id').value = wecom.AgentID || wecom.agent_id || '';

    // Dify 配置
    document.getElementById('dify-enabled').checked = dify.Enabled !== undefined ? dify.Enabled : (dify.enabled || false);
    document.getElementById('dify-api-key').value = dify.APIKey || dify.api_key || '';
    document.getElementById('dify-api-base').value = dify.APIBase || dify.api_base || '';
    document.getElementById('dify-app-id').value = dify.AppID || dify.app_id || '';
    document.getElementById('dify-user-id').value = dify.UserID || dify.user_id || '';

    // Coze 配置
    document.getElementById('coze-enabled').checked = coze.Enabled !== undefined ? coze.Enabled : (coze.enabled || false);
    document.getElementById('coze-api-key').value = coze.APIKey || coze.api_key || '';
    document.getElementById('coze-api-base').value = coze.APIBase || coze.api_base || '';
    document.getElementById('coze-bot-id').value = coze.BotID || coze.bot_id || '';
    document.getElementById('coze-user-id').value = coze.UserID || coze.user_id || '';
}

// 保存配置
async function saveConfig() {
    const config = collectFormData();

    try {
        const response = await fetch(`${API_BASE}/config`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(config),
        });

        const result = await response.json();

        if (result.success) {
            showMessage('配置保存成功！请重启服务使配置生效。', 'success');
            loadStatus();
        } else {
            showMessage('保存配置失败: ' + result.error, 'error');
        }
    } catch (error) {
        showMessage('保存配置失败: ' + error.message, 'error');
    }
}

// 收集表单数据
function collectFormData() {
    return {
        server: {
            host: document.getElementById('server-host').value,
            port: parseInt(document.getElementById('server-port').value) || 8080,
        },
        platform: {
            lark: {
                enabled: document.getElementById('lark-enabled').checked,
                app_id: document.getElementById('lark-app-id').value,
                app_secret: document.getElementById('lark-app-secret').value,
                domain: document.getElementById('lark-domain').value,
                bot_name: document.getElementById('lark-bot-name').value,
            },
            wecom: {
                enabled: document.getElementById('wecom-enabled').checked,
                corp_id: document.getElementById('wecom-corp-id').value,
                secret: document.getElementById('wecom-secret').value,
                token: document.getElementById('wecom-token').value,
                encoding_aes_key: document.getElementById('wecom-aes-key').value,
                host: document.getElementById('wecom-host').value,
                port: parseInt(document.getElementById('wecom-port').value) || 8888,
                agent_id: parseInt(document.getElementById('wecom-agent-id').value) || 0,
            },
        },
        agent: {
            dify: {
                enabled: document.getElementById('dify-enabled').checked,
                api_key: document.getElementById('dify-api-key').value,
                api_base: document.getElementById('dify-api-base').value,
                app_id: document.getElementById('dify-app-id').value,
                user_id: document.getElementById('dify-user-id').value,
            },
            coze: {
                enabled: document.getElementById('coze-enabled').checked,
                api_key: document.getElementById('coze-api-key').value,
                api_base: document.getElementById('coze-api-base').value,
                bot_id: document.getElementById('coze-bot-id').value,
                user_id: document.getElementById('coze-user-id').value,
            },
        },
    };
}

// 加载状态
async function loadStatus() {
    try {
        const response = await fetch(`${API_BASE}/status`);
        const result = await response.json();

        if (result.success) {
            const status = result.data;
            updateStatusBadge('lark-status', status.lark?.enabled);
            updateStatusBadge('wecom-status', status.wecom?.enabled);
            updateStatusBadge('dify-status', status.dify?.enabled);
            updateStatusBadge('coze-status', status.coze?.enabled);
        }
    } catch (error) {
        console.error('加载状态失败:', error);
    }
}

// 更新状态徽章
function updateStatusBadge(id, enabled) {
    const badge = document.getElementById(id);
    if (badge) {
        badge.textContent = enabled ? '已启用' : '已禁用';
        badge.className = 'status-badge ' + (enabled ? 'enabled' : 'disabled');
    }
}

// 显示消息
function showMessage(message, type) {
    const messageEl = document.getElementById('message');
    messageEl.textContent = message;
    messageEl.className = `message ${type}`;

    setTimeout(() => {
        messageEl.className = 'message';
    }, 5000);
}

