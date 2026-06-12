import { useState, useEffect } from 'react';
import { Button } from '@arco-design/web-react';
import { Settings, Globe, Cpu, CheckCircle, XCircle, RefreshCw } from 'lucide-react';
import { fetchAIConfig, saveAIConfig, testAIConnection, listAIModels } from '../services/api';

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
      <h2 style={{ color: 'var(--color-text-1)', fontSize: 18, fontWeight: 700, marginBottom: 20, display: 'flex', alignItems: 'center', gap: 8 }}>
        <Settings size={20} color="#165dff" /> AI 模型配置
      </h2>

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
    </div>
  );
}

const label: React.CSSProperties = { fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4, display: 'block', fontWeight: 500 };
