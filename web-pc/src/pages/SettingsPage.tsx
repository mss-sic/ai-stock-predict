import { useState, useEffect } from 'react';
import { Button, Tabs } from '@arco-design/web-react';
import { Settings, Globe, Cpu, CheckCircle, XCircle, RefreshCw } from 'lucide-react';
import { fetchAIConfig, saveAIConfig, testAIConnection, listAIModels, authFetch } from '../services/api';

const providers = [
  { label: 'DeepSeek (推荐)', value: 'deepseek', baseURL: 'https://api.deepseek.com', model: 'deepseek-chat' },
  { label: 'OpenAI', value: 'openai', baseURL: 'https://api.openai.com', model: 'gpt-4o' },
  { label: 'Moonshot', value: 'moonshot', baseURL: 'https://api.moonshot.cn', model: 'moonshot-v1-8k' },
  { label: '自定义', value: 'custom', baseURL: '', model: '' },
];

export default function SettingsPage() {
  const [provider, setProvider] = useState('deepseek');
  const [baseURL, setBaseURL] = useState('https://api.deepseek.com');
  const [apiKey, setApiKey] = useState('');
  const [modelName, setModelName] = useState('deepseek-chat');
  const [isActive, setIsActive] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [saving, setSaving] = useState(false);

  // ── AI System Configs ──
  interface SysCfg { scene: string; name: string; systemPrompt: string; modelName: string; temperature: number; maxTokens: number; enableSearch: boolean; enableTools: boolean; agentModelName: string; agentBaseURL: string; agentAPIKey: string; }
  const [sysConfigs, setSysConfigs] = useState<SysCfg[]>([]);
  const [editingScene, setEditingScene] = useState<string | null>(null);
  const [editCfg, setEditCfg] = useState<SysCfg | null>(null);
  const [savingSys, setSavingSys] = useState(false);

  useEffect(() => {
    authFetch('/api/v1/ai/system-configs').then(r => r.json()).then(j => {
      if (j.data) setSysConfigs(j.data);
    }).catch(() => {});
  }, []);

  const openEditSys = (cfg: SysCfg) => {
    setEditingScene(cfg.scene);
    setEditCfg({ ...cfg });
  };
  const saveSys = async () => {
    if (!editCfg) return;
    setSavingSys(true);
    try {
      const res = await authFetch(`/api/v1/ai/system-configs/${editCfg.scene}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: editCfg.name,
          systemPrompt: editCfg.systemPrompt,
          modelName: editCfg.modelName,
          temperature: editCfg.temperature,
          maxTokens: editCfg.maxTokens,
          enableSearch: editCfg.enableSearch,
          enableTools: editCfg.enableTools,
          agentModelName: editCfg.agentModelName,
          agentBaseURL: editCfg.agentBaseURL,
          agentAPIKey: editCfg.agentAPIKey,
        }),
      });
      const json = await res.json();
      if (json.code !== undefined && json.code !== 0) {
        throw new Error(json.message || '保存失败');
      }
      setSysConfigs(prev => prev.map(c => c.scene === editCfg.scene ? editCfg : c));
      setEditingScene(null);
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '提示词保存成功' } }));
    } catch (err: any) {
      console.error('[Settings] saveSys failed:', err);
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: err.message || '保存失败' } }));
    }
    setSavingSys(false);
  };
  const [modelList, setModelList] = useState<string[]>([]);
  const [fetchingModels, setFetchingModels] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const { data } = await fetchAIConfig();
        const d = data.data || {};
        if (d.provider) setProvider(d.provider);
        if (d.baseUrl) setBaseURL(d.baseUrl);
        if (d.modelName) setModelName(d.modelName);
        if (d.apiKey) setApiKey(d.apiKey);
        if (d.isActive !== undefined) setIsActive(d.isActive);
      } catch {}
    })();
  }, []);

  const handleProviderChange = (v: string) => {
    setProvider(v);
    const p = providers.find(p => p.value === v);
    if (p && v !== 'custom') {
      setBaseURL(p.baseURL);
      setModelName(p.model);
    }
    setModelList([]);
    setTestResult(null);
  };

  const handleFetchModels = async () => {
    if (!baseURL || !apiKey) { window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'warning', message: '请先填写 API 地址和 Key' } })); return; }
    setFetchingModels(true);
    try {
      const { data } = await listAIModels(baseURL, apiKey);
      if (data.data?.length) {
        setModelList(data.data);
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: `获取到 ${data.data.length} 个模型` } }));
      } else {
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: data.error || '未获取到模型列表' } }));
      }
    } catch {
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: '获取模型列表失败' } }));
    }
    setFetchingModels(false);
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const { data } = await testAIConnection({ provider, baseUrl: baseURL, apiKey, modelName });
      setTestResult({ ok: data.success, msg: data.message });
    } catch (err: any) {
      setTestResult({ ok: false, msg: err.response?.data?.message || '测试失败' });
    }
    setTesting(false);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await saveAIConfig({ provider, baseUrl: baseURL, apiKey, modelName });
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '配置已保存' } }));
    } catch {
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: '保存失败' } }));
    }
    setSaving(false);
  };

  const inp: React.CSSProperties = {
    width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--color-border-1)',
    background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 14, outline: 'none', boxSizing: 'border-box',
  };

  const sel: React.CSSProperties = { ...inp, cursor: 'pointer' };

  return (
    <div style={{ padding: '0 0 40px', maxWidth: 720 }}>
      <Tabs defaultActiveTab="model" style={{ marginTop: -8 }}>
        <Tabs.TabPane key="model" title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Settings size={14} /> AI 模型
          </span>
        }>

      {/* AI Config */}
      <div className="card">
        <div className="card-body">
          {/* Provider */}
          <div style={{ marginBottom: 16 }}>
            <label style={label}>AI 服务商</label>
            <select value={provider} onChange={e => handleProviderChange(e.target.value)} style={sel}>
              {providers.map(p => (
                <option key={p.value} value={p.value}>{p.label}</option>
              ))}
            </select>
          </div>

          {/* Base URL */}
          <div style={{ marginBottom: 16 }}>
            <label style={label}>API 地址</label>
            <input
              value={baseURL}
              onChange={e => setBaseURL(e.target.value)}
              placeholder="https://api.deepseek.com"
              style={inp}
            />
          </div>

          {/* API Key */}
          <div style={{ marginBottom: 16 }}>
            <label style={label}>API Key</label>
            <input
              type="password"
              value={apiKey}
              onChange={e => setApiKey(e.target.value)}
              placeholder="sk-..."
              style={inp}
            />
          </div>

          {/* Model */}
          <div style={{ marginBottom: 16 }}>
            <label style={label}>模型</label>
            <div style={{ display: 'flex', gap: 8 }}>
              {modelList.length > 0 ? (
                <select value={modelName} onChange={e => setModelName(e.target.value)} style={{ ...sel, flex: 1 }}>
                  {modelList.map(m => <option key={m} value={m}>{m}</option>)}
                </select>
              ) : (
                <input value={modelName} onChange={e => setModelName(e.target.value)} style={{ ...inp, flex: 1 }} placeholder="deepseek-chat" />
              )}
              <Button onClick={handleFetchModels} loading={fetchingModels} icon={<RefreshCw size={14} />} size="small">
                从上游拉取
              </Button>
            </div>
          </div>

          {/* Test result */}
          {testResult && (
            <div style={{
              marginBottom: 16, padding: '10px 16px', borderRadius: 8, fontSize: 13,
              background: testResult.ok ? 'var(--color-success-bg)' : 'var(--color-danger-bg)',
              color: testResult.ok ? 'var(--color-success)' : 'var(--color-danger)',
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              {testResult.ok ? <CheckCircle size={16} /> : <XCircle size={16} />}
              {testResult.msg}
            </div>
          )}

          {/* Actions */}
          <div style={{ display: 'flex', gap: 10 }}>
            <Button onClick={handleTest} loading={testing} icon={<Globe size={14} />}>
              测试连通
            </Button>
            <Button onClick={handleSave} loading={saving} type="primary" icon={<Cpu size={14} />}>
              保存配置
            </Button>
          </div>
        </div>
      </div>

      {/* Guide */}
      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-header" style={{ fontSize: 14, fontWeight: 600 }}>使用说明</div>
        <div className="card-body" style={{ fontSize: 13, color: 'var(--color-text-3)', lineHeight: 1.8 }}>
          <p>1. 填写 API 地址和 Key，点击「从上游拉取」获取可用模型</p>
          <p>2. 选择合适的模型后点击「测试连通」验证</p>
          <p>3. 测试通过后保存配置，系统将在 AI 分析时使用你的个人 Key</p>
          <p style={{ color: 'var(--color-text-3)', marginTop: 8, fontSize: 12 }}>
            支持 OpenAI / DeepSeek / Moonshot 等兼容接口。每个用户独立配置，互不影响。
          </p>
        </div>
      </div>

        </Tabs.TabPane>
        <Tabs.TabPane key="prompt" title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Cpu size={14} /> 提示词
          </span>
        }>

      {/* ── AI System Configs ── */}
      <div>
        {sysConfigs.map(cfg => (
          <div key={cfg.scene} className="card" style={{ marginBottom: 12 }}>
            <div className="card-body" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>{cfg.name}</div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>
                  scene: {cfg.scene} · temp: {cfg.temperature} · max_tokens: {cfg.maxTokens} · search: {cfg.enableSearch ? '✅' : '❌'}
                </div>
              </div>
              <Button size="small" onClick={() => openEditSys(cfg)}>编辑</Button>
            </div>
          </div>
        ))}
      </div>

        </Tabs.TabPane>
      </Tabs>

      {/* Edit Modal */}
      {editingScene && editCfg && (
        <div style={{
          position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.4)', zIndex: 9999,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }} onClick={() => setEditingScene(null)}>
          <div style={{
            background: 'var(--color-bg-1)', borderRadius: 12, padding: 24, width: 640, maxHeight: '80vh',
            overflow: 'auto', boxShadow: '0 8px 32px rgba(0,0,0,0.18)',
          }} onClick={e => e.stopPropagation()}>
            <h3 style={{ margin: '0 0 16px', fontSize: 16 }}>编辑 {editCfg.name}</h3>

            <label style={label}>场景标识</label>
            <input value={editCfg.scene} disabled style={{ ...inp, marginBottom: 12, color: 'var(--color-text-3)' }} />

            <label style={label}>名称</label>
            <input value={editCfg.name} onChange={e => setEditCfg({...editCfg, name: e.target.value})} style={{ ...inp, marginBottom: 12 }} />

            <label style={label}>系统提示词（支持 %s 占位符）</label>
            <textarea value={editCfg.systemPrompt} onChange={e => setEditCfg({...editCfg, systemPrompt: e.target.value})}
              style={{ ...inp, marginBottom: 12, minHeight: 160, fontFamily: 'monospace', fontSize: 11, resize: 'vertical' }} />

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginBottom: 12 }}>
              <div>
                <label style={label}>Temperature</label>
                <input type="number" step="0.1" min="0" max="2" value={editCfg.temperature}
                  onChange={e => setEditCfg({...editCfg, temperature: parseFloat(e.target.value) || 0})} style={inp} />
              </div>
              <div>
                <label style={label}>Max Tokens</label>
                <input type="number" min="100" max="8192" value={editCfg.maxTokens}
                  onChange={e => setEditCfg({...editCfg, maxTokens: parseInt(e.target.value) || 0})} style={inp} />
              </div>
              <div>
                <label style={label}>联网搜索</label>
                <select value={editCfg.enableSearch ? '1' : '0'}
                  onChange={e => setEditCfg({...editCfg, enableSearch: e.target.value === '1'})} style={sel}>
                  <option value="1">✅ 开启</option>
                  <option value="0">❌ 关闭</option>
                </select>
              </div>
              <div>
                <label style={label}>Agent工具调用</label>
                <select value={editCfg.enableTools ? '1' : '0'}
                  onChange={e => setEditCfg({...editCfg, enableTools: e.target.value === '1'})} style={sel}>
                  <option value="1">✅ 开启</option>
                  <option value="0">❌ 关闭</option>
                </select>
              </div>
            </div>


            {editCfg.enableTools && <>
              <label style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, display: 'block', color: 'var(--color-text-2)' }}>工具模式专用模型（可选，如 Kimi）</label>
              <input value={editCfg.agentModelName} onChange={e => setEditCfg({...editCfg, agentModelName: e.target.value})}
                placeholder="moonshot-v1-8k" style={{ ...inp, marginBottom: 8 }} />
              <input value={editCfg.agentBaseURL} onChange={e => setEditCfg({...editCfg, agentBaseURL: e.target.value})}
                placeholder="https://api.moonshot.cn" style={{ ...inp, marginBottom: 8 }} />
              <input type="password" value={editCfg.agentAPIKey} onChange={e => setEditCfg({...editCfg, agentAPIKey: e.target.value})}
                placeholder="Agent API Key" style={{ ...inp, marginBottom: 12 }} />
            </>}
            <label style={label}>模型覆盖（空=用用户配置）</label>
            <input value={editCfg.modelName} onChange={e => setEditCfg({...editCfg, modelName: e.target.value})}
              placeholder="留空则使用用户配置的模型" style={{ ...inp, marginBottom: 16 }} />

            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <Button onClick={() => setEditingScene(null)}>取消</Button>
              <Button type="primary" onClick={saveSys} loading={savingSys}>保存</Button>
            </div>
          </div>
        </div>
      )}

    </div>
  );
}

// ── styles ──
const label: React.CSSProperties = { fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4, display: 'block', fontWeight: 500 };
