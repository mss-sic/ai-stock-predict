import { useState, useEffect } from 'react';
import { Input, Button, Select, Message } from '@arco-design/web-react';
import { Settings, Key, Globe, Cpu, CheckCircle, XCircle, Loader2, RefreshCw } from 'lucide-react';

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
        const res = await fetch('http://127.0.0.1:8080/api/v1/settings/ai');
        const json = await res.json();
        const d = json.data || {};
        if (d.provider) setProvider(d.provider);
        if (d.baseURL) setBaseURL(d.baseUrl || d.baseURL);
        if (d.modelName) setModelName(d.modelName);
        if (d.apiKey) setApiKey(d.apiKey);
        if (d.isActive !== undefined) setIsActive(d.isActive);
      } catch (_) {}
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
  };

  // Fetch available models from provider
  const handleFetchModels = async () => {
    if (!baseURL || !apiKey) {
      Message.warning('请先填写 API 地址和 Key');
      return;
    }
    setFetchingModels(true);
    try {
      const res = await fetch('http://127.0.0.1:8080/api/v1/settings/ai/models', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ baseUrl: baseURL, apiKey }),
      });
      const json = await res.json();
      if (json.data?.length > 0) {
        setModelList(json.data);
        Message.success(`获取到 ${json.data.length} 个模型`);
      } else {
        Message.error(json.error || '未获取到模型列表');
      }
    } catch (e: any) {
      Message.error('网络错误: ' + e.message);
    }
    setFetchingModels(false);
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await fetch('http://127.0.0.1:8080/api/v1/settings/ai/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider, apiKey, modelName, baseUrl: baseURL }),
      });
      const json = await res.json();
      if (json.success) {
        setTestResult({ ok: true, msg: json.message });
        setIsActive(true);
      } else {
        setTestResult({ ok: false, msg: json.message });
      }
    } catch (e: any) {
      setTestResult({ ok: false, msg: '网络错误: ' + e.message });
    }
    setTesting(false);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetch('http://127.0.0.1:8080/api/v1/settings/ai', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider, apiKey, modelName, baseUrl: baseURL }),
      });
      Message.success('保存成功');
    } catch (_) {
      Message.error('保存失败');
    }
    setSaving(false);
  };

  return (
    <div>
      <div className="page-header">
        <h2 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Settings size={20} color="#165dff" />
          <span style={{ fontSize: 16, fontWeight: 700 }}>系统设置</span>
        </h2>
        <span className="muted" style={{ fontSize: 13 }}>配置AI模型以启用智能分析功能</span>
      </div>

      <div style={{ maxWidth: 640 }}>
        <div style={{
          marginBottom: 20, padding: '14px 18px', borderRadius: 8,
          background: isActive ? '#e8ffea' : '#fff7e8',
          border: `1px solid ${isActive ? '#00b42a' : '#ff7d00'}`,
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          {isActive
            ? <CheckCircle size={18} color="#00b42a" />
            : <XCircle size={18} color="#ff7d00" />
          }
          <div>
            <div style={{ fontWeight: 600, fontSize: 14, color: isActive ? '#009a29' : '#6b5900' }}>
              {isActive ? 'AI分析已启用' : 'AI分析未启用'}
            </div>
            <div style={{ fontSize: 12, color: isActive ? '#00b42a' : '#ff7d00' }}>
              {isActive ? '系统将在榜单更新后自动进行AI分析' : '请配置并测试AI模型后开启'}
            </div>
          </div>
        </div>

        <div className="card mb16">
          <div className="card-header">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Cpu size={14} />
              <span style={{ fontWeight: 600, fontSize: 14 }}>AI模型配置</span>
            </span>
          </div>
          <div className="card-body">
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

              <div>
                <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 6, color: '#1d2129' }}>服务商</div>
                <Select
                  value={provider}
                  onChange={handleProviderChange}
                  style={{ width: '100%' }}
                  options={providers.map(p => ({ label: p.label, value: p.value }))}
                />
              </div>

              <div>
                <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 6, color: '#1d2129' }}>
                  <Globe size={12} style={{ marginRight: 4, verticalAlign: -1 }} />API地址
                </div>
                <Input value={baseURL} onChange={setBaseURL} placeholder="https://api.deepseek.com" />
              </div>

              <div>
                <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 6, color: '#1d2129' }}>
                  <Key size={12} style={{ marginRight: 4, verticalAlign: -1 }} />API Key
                </div>
                <Input.Password value={apiKey} onChange={setApiKey} placeholder="sk-..." />
              </div>

              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                  <span style={{ fontSize: 13, fontWeight: 500, color: '#1d2129' }}>
                    <Cpu size={12} style={{ marginRight: 4, verticalAlign: -1 }} />模型选择
                  </span>
                  <Button
                    size="mini"
                    type="text"
                    icon={<RefreshCw size={12} className={fetchingModels ? 'spin' : ''} />}
                    onClick={handleFetchModels}
                    loading={fetchingModels}
                    style={{ fontSize: 11, color: '#165dff' }}
                  >
                    拉取模型列表
                  </Button>
                </div>
                {modelList.length > 0 ? (
                  <Select
                    value={modelName}
                    onChange={setModelName}
                    style={{ width: '100%' }}
                    options={modelList.map(m => ({ label: m, value: m }))}
                    allowSearch
                    placeholder="选择模型"
                  />
                ) : (
                  <Input value={modelName} onChange={setModelName} placeholder="手动输入模型名称，或点击上方拉取" />
                )}
              </div>

              {testResult && (
                <div style={{
                  padding: '10px 14px', borderRadius: 6, fontSize: 13,
                  background: testResult.ok ? '#e8ffea' : '#ffece8',
                  color: testResult.ok ? '#009a29' : '#cb272d',
                  border: `1px solid ${testResult.ok ? '#00b42a' : '#f53f3f'}`,
                  display: 'flex', alignItems: 'center', gap: 6,
                }}>
                  {testResult.ok ? <CheckCircle size={14} /> : <XCircle size={14} />}
                  {testResult.msg}
                </div>
              )}

              <div style={{ display: 'flex', gap: 10 }}>
                <Button type="outline" onClick={handleTest} loading={testing}>
                  测试连通
                </Button>
                <Button type="primary" onClick={handleSave} loading={saving}>
                  保存配置
                </Button>
              </div>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <span style={{ fontWeight: 600, fontSize: 14 }}>快速指南</span>
          </div>
          <div className="card-body" style={{ fontSize: 13, color: '#4e5969', lineHeight: 1.8 }}>
            <p><b>1.</b> 填写 API 地址和 Key，点击「拉取模型列表」获取可用模型</p>
            <p><b>2.</b> 从下拉列表选择合适的模型</p>
            <p><b>3.</b> 点击「测试连通」验证可用性</p>
            <p><b>4.</b> 测试通过后点击「保存配置」，系统将在榜单更新时自动进行AI分析</p>
            <p style={{ color: '#86909c', marginTop: 8, fontSize: 12 }}>
              支持 OpenAI / DeepSeek / Moonshot 等兼容接口。DeepSeek v3 性价比最优，百万token仅需2元。
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
