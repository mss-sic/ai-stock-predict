import { useState, useEffect, useCallback } from 'react';
import { fetchApiKeys, createApiKey, updateApiKey, deleteApiKey } from '../services/api';
import {
  Key, Plus, Trash2, Copy, Check, Shield, AlertTriangle,
  Power, PowerOff, RefreshCw, BookOpen, ChevronDown, ChevronRight,
  Terminal, Server,
} from 'lucide-react';
import { Modal } from '@arco-design/web-react';
import { showToast } from './Toast';

interface ApiKeyRecord {
  id: number;
  keyPrefix: string;
  teamName: string;
  description: string;
  permissions: string;
  isActive: boolean;
  lastUsedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

const DATA_TYPES = ['prediction', 'kline', 'indicator', 'profile', 'signal'] as const;

const DATA_TYPE_LABELS: Record<string, string> = {
  prediction: '预测数据',
  kline: 'K线数据',
  indicator: '技术指标',
  profile: '个股研报',
  signal: '交易信号',
};

const DATA_TYPE_DESC: Record<string, string> = {
  prediction: '模型预测价格 / 置信区间，写入 predictions 表',
  kline: '日K线 OHLCV 行情数据，写入 stocks_daily_k 表',
  indicator: '技术指标值（PE/PB/ROE 等），写入 stocks_daily_indicator 表',
  profile: '个股研报 / 分析 Markdown，写入 stock_profiles 表',
  signal: '交易信号评分，写入 stock_signals 表',
};

export default function ApiKeyManagement() {
  const [keys, setKeys] = useState<ApiKeyRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newTeam, setNewTeam] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [newPerms, setNewPerms] = useState<string[]>(['prediction']);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ApiKeyRecord | null>(null);
  const [showDocs, setShowDocs] = useState(true);

  const loadKeys = useCallback(async () => {
    setLoading(true);
    try {
      const { data } = await fetchApiKeys();
      setKeys(data.data || []);
    } catch {}
    setLoading(false);
  }, []);

  useEffect(() => { loadKeys(); }, [loadKeys]);

  const handleCreate = async () => {
    if (!newTeam.trim()) { showToast('warning', '请输入团队名称'); return; }
    try {
      const { data } = await createApiKey(newTeam.trim(), newDesc.trim(), newPerms);
      const result = data.data;
      setCreatedKey(result.apiKey);
      setShowCreate(false);
      setNewTeam('');
      setNewDesc('');
      setNewPerms(['prediction']);
      loadKeys();
    } catch (err: any) {
      showToast('error', err.response?.data?.message || '创建失败');
    }
  };

  const handleToggle = async (key: ApiKeyRecord) => {
    try {
      await updateApiKey(key.id, { isActive: !key.isActive });
      showToast('success', key.isActive ? '已停用' : '已启用');
      loadKeys();
    } catch (err: any) {
      showToast('error', err.response?.data?.message || '操作失败');
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteApiKey(deleteTarget.id);
      showToast('success', '已删除');
      setDeleteTarget(null);
      loadKeys();
    } catch (err: any) {
      showToast('error', err.response?.data?.message || '删除失败');
    }
  };

  const handleCopy = (text: string, id: number) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const togglePerm = (perm: string) => {
    setNewPerms(prev =>
      prev.includes(perm) ? prev.filter(p => p !== perm) : [...prev, perm]
    );
  };

  const formatTime = (t: string | null) => {
    if (!t) return '从未使用';
    return new Date(t).toLocaleString('zh-CN', {
      month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    });
  };

  const parsePermissions = (permsJson: string): string[] => {
    try { return JSON.parse(permsJson); } catch { return []; }
  };

  const API_BASE = window.location.origin + '/api/v1';

  // ── Styles ──
  const cardStyle: React.CSSProperties = {
    background: 'var(--color-bg-2)',
    borderRadius: 10,
    border: '1px solid var(--color-border-2)',
    padding: '20px 24px',
    marginBottom: 12,
  };

  const badgeActive: React.CSSProperties = {
    display: 'inline-flex', alignItems: 'center', gap: 4,
    padding: '2px 10px', borderRadius: 10, fontSize: 11,
    background: 'rgba(0,180,42,0.1)', color: '#00B42A', fontWeight: 500,
  };

  const badgeInactive: React.CSSProperties = {
    ...badgeActive,
    background: 'rgba(245,63,63,0.1)', color: '#F53F3F',
  };

  return (
    <div>
      {/* ═══ API 接入文档 ═══ */}
      <div style={{ ...cardStyle, marginBottom: 24 }}>
        <div
          onClick={() => setShowDocs(!showDocs)}
          style={{
            display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer',
            userSelect: 'none',
          }}
        >
          {showDocs ? <ChevronDown size={16} style={{ color: 'var(--color-text-2)' }} /> : <ChevronRight size={16} style={{ color: 'var(--color-text-2)' }} />}
          <BookOpen size={16} style={{ color: 'var(--color-primary)' }} />
          <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>API 接入文档</span>
          <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>— 外部团队数据导入接口说明</span>
        </div>

        {showDocs && (
          <div style={{ marginTop: 16 }}>
            {/* 概览 */}
            <div style={{
              padding: '12px 16px', borderRadius: 8,
              background: 'rgba(22,93,255,0.04)', marginBottom: 16,
              border: '1px solid rgba(22,93,255,0.1)',
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                <Server size={14} style={{ color: 'var(--color-primary)' }} />
                <code style={{
                  fontSize: 13, fontWeight: 600, fontFamily: "'SF Mono', monospace",
                  color: 'var(--color-primary)',
                }}>
                  POST {API_BASE}/data/import
                </code>
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: 1.8 }}>
                统一数据导入入口，通过 <code style={codeInline}>X-API-Key</code> 请求头认证。
                支持 5 种数据类型，由请求体 <code style={codeInline}>type</code> 字段区分。
              </div>
            </div>

            {/* 请求格式 */}
            <div style={{ marginBottom: 16 }}>
              <h4 style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', margin: '0 0 10px' }}>
                通用请求格式
              </h4>
              <pre style={{
                background: 'var(--color-fill-1)', borderRadius: 8, padding: '14px 16px',
                fontSize: 12, fontFamily: "'SF Mono', monospace", color: 'var(--color-text-2)',
                lineHeight: 1.7, margin: 0, overflow: 'auto',
              }}>{`POST /api/v1/data/import
Content-Type: application/json
X-API-Key: <your-api-key>

{
  "type": "prediction",       // 必填: prediction | kline | indicator | profile | signal
  "data": { ... },            // 必填: 对象或数组，结构因 type 而异（prediction为对象，其他为数组）
  "source": "my-system-v2"    // 可选: 自定义来源标识
}`}</pre>
            </div>

            {/* 数据类型说明 */}
            <h4 style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', margin: '0 0 10px' }}>
              数据类型结构
            </h4>
            {(['prediction', 'kline', 'indicator', 'profile', 'signal'] as const).map(dt => (
              <details key={dt} style={{ marginBottom: 10 }}>
                <summary style={{
                  cursor: 'pointer', fontSize: 12, fontWeight: 600,
                  color: 'var(--color-text-2)', padding: '6px 10px',
                  borderRadius: 6, background: 'var(--color-fill-1)',
                }}>
                  {DATA_TYPE_LABELS[dt]}
                  <span style={{ fontSize: 10, color: 'var(--color-text-3)', marginLeft: 8, fontWeight: 400 }}>
                    type=&quot;{dt}&quot; — {DATA_TYPE_DESC[dt]}
                  </span>
                </summary>
                <pre style={{
                  background: 'var(--color-fill-1)', borderRadius: 0, padding: '12px 16px',
                  fontSize: 11, fontFamily: "'SF Mono', monospace", color: 'var(--color-text-2)',
                  lineHeight: 1.7, margin: 0, overflow: 'auto', borderBottomLeftRadius: 8, borderBottomRightRadius: 8,
                }}>{getExample(dt)}</pre>
              </details>
            ))}

            {/* curl 示例 */}
            <h4 style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', margin: '16px 0 10px' }}>
              <Terminal size={13} style={{ marginRight: 6, verticalAlign: -2 }} />
              快速测试
            </h4>
            <pre style={{
              background: '#1e1e2e', borderRadius: 8, padding: '14px 16px',
              fontSize: 11, fontFamily: "'SF Mono', monospace", color: '#cdd6f4',
              lineHeight: 1.7, margin: 0, overflow: 'auto',
            }}>{`# 导入预测数据 (与数据管理→文件导入→导入预测数据JSON 格式完全一致)
curl -X POST ${API_BASE}/data/import \\
  -H "Content-Type: application/json" \\
  -H "X-API-Key: ak-xxx..." \\
  -d '{
    "type": "prediction",
    "data": {
      "total_units_number": 100,
      "kdis": 6,
      "max_predict_day": 20,
      "data_units": [{
        "index": 1,
        "stock_code": "601279",
        "stock_name": "英利汽车",
        "confidence": "1.00",
        "today_wave": "0.85",
        "today_trade_money": "0.38",
        "today_trade_rate": "0.67",
        "real_wave": [0.0, ...],
        "kdistributed_data": [[-0.43, -0.72, ...], [...]]
      }]
    }
  }'

# 返回示例
# {"code":0,"data":{"imported":600,"skipped":5,"total":100},"message":"ok"}`}</pre>

            <div style={{
              marginTop: 12, padding: '8px 14px', borderRadius: 8,
              background: 'rgba(247,105,0,0.06)', border: '1px solid rgba(247,105,0,0.12)',
            }}>
              <AlertTriangle size={13} style={{ color: '#F76900', marginRight: 6, verticalAlign: -2 }} />
              <span style={{ fontSize: 11, color: '#F76900' }}>
                错误响应格式：<code style={codeInline}>{`{"code":403,"message":"该 API Key 没有导入 xxx 类型数据的权限"}`}</code>
              </span>
            </div>
          </div>
        )}
      </div>

      {/* ═══ 密钥管理 ═══ */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600, color: 'var(--color-text-1)' }}>
            <Key size={18} style={{ marginRight: 8, verticalAlign: -3, color: 'var(--color-primary)' }} />
            API 密钥管理
          </h3>
          <p style={{ margin: '4px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>
            为外部团队创建数据导入密钥，每个密钥可限定可导入的数据类型
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          style={{ ...primaryBtnSm, background: 'linear-gradient(135deg, #165DFF, #722ED1)' }}
        >
          <Plus size={15} /> 创建密钥
        </button>
      </div>

      {/* Key list */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 40, color: 'var(--color-text-3)' }}>
          <RefreshCw size={20} style={{ animation: 'spin 1s linear infinite' }} />
          <div style={{ marginTop: 8 }}>加载中...</div>
        </div>
      ) : keys.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)' }}>
          <Shield size={40} style={{ opacity: 0.3 }} />
          <div style={{ marginTop: 12, fontSize: 14 }}>暂无 API 密钥</div>
          <div style={{ marginTop: 4, fontSize: 12 }}>点击「创建密钥」为外部团队生成数据导入凭证</div>
        </div>
      ) : (
        keys.map(k => {
          const perms = parsePermissions(k.permissions);
          return (
            <div key={k.id} style={cardStyle}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
                    <span style={{ fontSize: 15, fontWeight: 600, color: 'var(--color-text-1)' }}>
                      {k.teamName}
                    </span>
                    <span style={k.isActive ? badgeActive : badgeInactive}>
                      {k.isActive ? <><Power size={11} /> 启用</> : <><PowerOff size={11} /> 停用</>}
                    </span>
                  </div>

                  {k.description && (
                    <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 8 }}>
                      {k.description}
                    </div>
                  )}

                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                    <code style={{
                      fontSize: 12, fontFamily: "'SF Mono', monospace",
                      padding: '3px 10px', borderRadius: 6,
                      background: 'var(--color-fill-1)', color: 'var(--color-text-2)',
                    }}>
                      {k.keyPrefix}•••••••••
                    </code>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                      ID: {k.id}
                    </span>
                  </div>

                  {/* Permissions with Chinese labels */}
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                    {perms.map(p => (
                      <span key={p} style={{
                        padding: '2px 10px', borderRadius: 8, fontSize: 11,
                        background: 'var(--color-fill-2)', color: 'var(--color-text-2)',
                        fontWeight: 500,
                      }}>
                        {DATA_TYPE_LABELS[p] || p}
                      </span>
                    ))}
                    {perms.length === 0 && (
                      <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>无权限</span>
                    )}
                  </div>

                  <div style={{ display: 'flex', gap: 16, marginTop: 10, fontSize: 11, color: 'var(--color-text-3)' }}>
                    <span>创建: {formatTime(k.createdAt)}</span>
                    <span>最后使用: {formatTime(k.lastUsedAt)}</span>
                  </div>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <button
                    onClick={() => handleToggle(k)}
                    style={{
                      ...btnSmStyle,
                      color: k.isActive ? '#F53F3F' : '#00B42A',
                      borderColor: k.isActive ? 'rgba(245,63,63,0.3)' : 'rgba(0,180,42,0.3)',
                    }}
                  >
                    {k.isActive ? <PowerOff size={13} /> : <Power size={13} />}
                    {k.isActive ? '停用' : '启用'}
                  </button>
                  <button
                    onClick={() => setDeleteTarget(k)}
                    style={{ ...btnSmStyle, color: '#F53F3F', borderColor: 'rgba(245,63,63,0.3)' }}
                  >
                    <Trash2 size={13} /> 删除
                  </button>
                </div>
              </div>
            </div>
          );
        })
      )}

      {/* ═══ Create Modal ═══ */}
      {showCreate && (
        <div style={overlayStyle} onClick={() => setShowCreate(false)}>
          <div style={modalStyle} onClick={e => e.stopPropagation()}>
            <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 20, color: 'var(--color-text-1)' }}>
              <Plus size={18} style={{ marginRight: 8, verticalAlign: -4 }} />
              创建 API 密钥
            </div>

            <div style={{ marginBottom: 16 }}>
              <label style={labelStyle}>团队名称 *</label>
              <input type="text" value={newTeam} onChange={e => setNewTeam(e.target.value)}
                placeholder="如：算法团队、风控系统" style={inputStyle} autoFocus />
            </div>

            <div style={{ marginBottom: 16 }}>
              <label style={labelStyle}>描述（可选）</label>
              <input type="text" value={newDesc} onChange={e => setNewDesc(e.target.value)}
                placeholder="如：每日预测数据导入" style={inputStyle} />
            </div>

            <div style={{ marginBottom: 20 }}>
              <label style={labelStyle}>数据权限</label>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 8 }}>
                {DATA_TYPES.map(dt => (
                  <button key={dt} onClick={() => togglePerm(dt)}
                    style={{
                      padding: '6px 14px', borderRadius: 8, fontSize: 12,
                      border: newPerms.includes(dt)
                        ? '1.5px solid var(--color-primary)'
                        : '1px solid var(--color-border-2)',
                      background: newPerms.includes(dt)
                        ? 'rgba(22,93,255,0.08)'
                        : 'var(--color-fill-1)',
                      color: newPerms.includes(dt)
                        ? 'var(--color-primary)'
                        : 'var(--color-text-2)',
                      cursor: 'pointer', fontWeight: 500,
                      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2,
                    }}
                    title={DATA_TYPE_DESC[dt]}
                  >
                    <span>{DATA_TYPE_LABELS[dt]}</span>
                    <span style={{ fontSize: 10, opacity: 0.6 }}>{dt}</span>
                  </button>
                ))}
              </div>
            </div>

            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button onClick={() => setShowCreate(false)} style={cancelBtn}>取消</button>
              <button onClick={handleCreate} style={primaryBtn}>创建</button>
            </div>
          </div>
        </div>
      )}

      {/* ═══ Created Key Modal ═══ */}
      {createdKey && (
        <div style={overlayStyle} onClick={() => setCreatedKey(null)}>
          <div style={{ ...modalStyle, maxWidth: 560 }} onClick={e => e.stopPropagation()}>
            <div style={{ textAlign: 'center', marginBottom: 16 }}>
              <Check size={40} style={{ color: '#00B42A' }} />
              <div style={{ fontSize: 16, fontWeight: 600, marginTop: 8, color: 'var(--color-text-1)' }}>
                密钥创建成功
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 4 }}>
                请立即复制保存，此密钥仅显示一次
              </div>
            </div>

            <div style={{
              background: 'var(--color-fill-1)', padding: '14px 16px',
              borderRadius: 8, marginBottom: 12, position: 'relative',
            }}>
              <code style={{
                fontSize: 12, fontFamily: "'SF Mono', monospace",
                color: 'var(--color-text-1)', wordBreak: 'break-all',
              }}>
                {createdKey}
              </code>
              <button
                onClick={() => handleCopy(createdKey, -1)}
                style={{
                  position: 'absolute', right: 8, top: 8,
                  background: 'var(--color-fill-2)', border: 'none',
                  borderRadius: 6, padding: 6, cursor: 'pointer',
                  color: copiedId === -1 ? '#00B42A' : 'var(--color-text-2)',
                }}
              >
                {copiedId === -1 ? <Check size={14} /> : <Copy size={14} />}
              </button>
            </div>

            <div style={{
              padding: '10px 14px', borderRadius: 8,
              background: 'rgba(247,105,0,0.06)', marginBottom: 16,
              border: '1px solid rgba(247,105,0,0.12)',
            }}>
              <AlertTriangle size={14} style={{ color: '#F76900', marginRight: 6, verticalAlign: -2 }} />
              <span style={{ fontSize: 12, color: '#F76900', lineHeight: 1.6 }}>
                请求时在 Header 中添加 <code style={codeInline}>X-API-Key: {createdKey.substring(0, 20)}...</code>
              </span>
            </div>

            <div style={{ textAlign: 'right' }}>
              <button onClick={() => setCreatedKey(null)} style={primaryBtn}>我已保存</button>
            </div>
          </div>
        </div>
      )}

      {/* ═══ Delete Confirmation ═══ */}
      <Modal
        title="确认删除"
        visible={deleteTarget !== null}
        onOk={handleDelete}
        onCancel={() => setDeleteTarget(null)}
        okText="确认删除"
        cancelText="取消"
        okButtonProps={{ status: 'danger' }}
      >
        <div style={{ fontSize: 14, color: 'var(--color-text-2)' }}>
          确定要删除 <strong>{deleteTarget?.teamName}</strong> 的 API 密钥吗？
          <br />
          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
            删除后该密钥将立即失效，使用该密钥的外部团队将无法导入数据。
          </span>
        </div>
      </Modal>
    </div>
  );
}

// ── Helpers ──

function getExample(dt: string): string {
  switch (dt) {
    case 'prediction':
      return `{
  "type": "prediction",
  "data": {
    "total_units_number": 100,
    "kdis": 6,
    "max_predict_day": 20,
    "data_units": [{
      "index": 1,
      "stock_code": "601279",
      "stock_name": "英利汽车",
      "confidence": "1.00",
      "today_wave": "0.85",
      "today_trade_money": "0.38",
      "today_trade_rate": "0.67",
      "real_wave": [0.0, 0.0, 0.0, ...],
      "kdistributed_data": [[
        -0.43, -0.72, -0.43, -0.07, 0.69, 0.94,
        1.23, 1.96, 2.33, 2.88, 3.35, 4.00,
        4.27, 4.30, 3.05, 2.85, 3.07, 3.26,
        3.95, 4.10
      ], [...], ...]
    }]
  }
}`;
    case 'kline':
      return `{
  "type": "kline",
  "data": [{
    "code": "000001",
    "klines": [{
      "date": "2026-07-10",
      "open": 12.00, "high": 12.50, "low": 11.80,
      "close": 12.30, "volume": 1000000,
      "amount": 12300000, "turnoverRate": 2.5
    }]
  }]
}`;
    case 'indicator':
      return `{
  "type": "indicator",
  "data": [{
    "code": "000001",
    "date": "2026-07-10",
    "indicators": [
      {"name": "pe_ttm", "value": 8.5},
      {"name": "pb", "value": 0.9},
      {"name": "roe", "value": 12.3}
    ]
  }]
}`;
    case 'profile':
      return `{
  "type": "profile",
  "data": [{
    "stock_code": "301176.SZ",
    "raw_code": "301176",
    "company_name": "逸豪新材",
    "raw_name": "逸豪新材",
    "market": "在深圳证券交易所创业板上市",
    "analysis_date": "2026-06-12 18:54:55",
    "analysis_content": "### 一、核心特征总结\\n**逸豪新材是一家专注于电子电路铜箔...**"
  }]
}`;
    case 'signal':
      return `{
  "type": "signal",
  "data": [
    {"code": "000001", "signalValue": 0.85},
    {"code": "000002", "signalValue": -0.32}
  ]
}`;
    default:
      return '';
  }
}

// ── Styles ──
const overlayStyle: React.CSSProperties = {
  position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
  background: 'rgba(0,0,0,0.5)', display: 'flex',
  alignItems: 'center', justifyContent: 'center', zIndex: 1000,
};

const modalStyle: React.CSSProperties = {
  background: 'var(--color-bg-1)', borderRadius: 12,
  padding: '24px 28px', maxWidth: 460, width: '90%',
  boxShadow: '0 8px 40px rgba(0,0,0,0.18)',
};

const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 12, fontWeight: 600,
  color: 'var(--color-text-2)', marginBottom: 6,
};

const inputStyle: React.CSSProperties = {
  width: '100%', padding: '8px 12px', borderRadius: 6,
  border: '1px solid var(--color-border-2)', fontSize: 13,
  outline: 'none', background: 'var(--color-fill-1)',
  color: 'var(--color-text-1)', boxSizing: 'border-box',
};

const cancelBtn: React.CSSProperties = {
  padding: '8px 20px', background: 'var(--color-fill-2)',
  color: 'var(--color-text-2)', border: 'none', borderRadius: 6,
  fontSize: 13, cursor: 'pointer',
};

const primaryBtn: React.CSSProperties = {
  padding: '8px 20px', background: 'var(--color-primary)',
  color: '#fff', border: 'none', borderRadius: 6,
  fontSize: 13, cursor: 'pointer', fontWeight: 500,
};

const primaryBtnSm: React.CSSProperties = {
  display: 'inline-flex', alignItems: 'center', gap: 6,
  padding: '8px 16px', border: 'none', borderRadius: 8,
  fontSize: 13, color: '#fff', cursor: 'pointer', fontWeight: 500,
};

const btnSmStyle: React.CSSProperties = {
  display: 'inline-flex', alignItems: 'center', gap: 4,
  padding: '5px 12px', border: '1px solid', borderRadius: 6,
  fontSize: 11, background: 'transparent', cursor: 'pointer',
  fontWeight: 500, whiteSpace: 'nowrap',
};

const codeInline: React.CSSProperties = {
  background: 'var(--color-fill-2)', padding: '1px 6px',
  borderRadius: 4, fontFamily: "'SF Mono', monospace",
  fontSize: 11,
};
