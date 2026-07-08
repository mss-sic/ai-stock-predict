import { useState, useEffect } from 'react';
import { Switch, Button, Drawer, InputNumber, Message, Tooltip, Tag } from '@arco-design/web-react';
import { Settings, TrendingDown, AlertTriangle, Layers, Droplets, FileText, UserCheck, BookOpen, ChevronDown, ChevronRight, Gauge } from 'lucide-react';
import { fetchRiskRules, updateRiskRule } from '../services/api';

const dimMeta: Record<string, { label: string; icon: any; color: string; bg: string; desc: string }> = {
  market:    { label: '市场',   icon: TrendingDown,   color: '#165DFF', bg: '#165DFF10', desc: '系统性风险，影响全市场所有股票' },
  stock:     { label: '个股',   icon: AlertTriangle,  color: '#722ED1', bg: '#722ED110', desc: '单只股票的技术面、估值与基本面' },
  portfolio: { label: '组合',   icon: Layers,         color: '#ff7d00', bg: '#ff7d0010', desc: '持仓组合的集中度与风险分散' },
  liquidity: { label: '流动性', icon: Droplets,       color: '#0fc6c2', bg: '#0fc6c210', desc: '成交活跃度、变现能力与封板风险' },
  event:     { label: '事件',   icon: FileText,       color: '#f53f3f', bg: '#f53f3f10', desc: '公告减持、诉讼、除权等突发事件' },
  behavior:  { label: '行为',   icon: UserCheck,      color: '#86909c', bg: '#86909c10', desc: '交易频率、止损执行、策略偏离' },
};

const levelMeta: Record<string, { label: string; color: string; bg: string }> = {
  high:   { label: '高风险', color: '#f53f3f', bg: '#f53f3f12' },
  medium: { label: '中风险', color: '#ff7d00', bg: '#ff7d0012' },
  low:    { label: '低风险', color: '#00b42a', bg: '#00b42a12' },
};

/** Parse DB description (格式: 数据来源:...。计算方式:...。风控意义:...。阈值:...) */
function parseDesc(desc: string) {
  const parts: Record<string, string> = {};
  const keys = ['数据来源', '计算方式', '风控意义', '阈值'];
  let remaining = desc || '';
  for (const k of keys) {
    const prefix = k + ':';
    const idx = remaining.indexOf(prefix);
    if (idx === -1) {
      parts[k] = remaining.trim();
      remaining = '';
      break;
    }
    const next = remaining.substring(idx + prefix.length);
    // Find next key
    let end = next.length;
    for (const nk of keys) {
      const nidx = next.indexOf(nk + ':');
      if (nidx !== -1 && nidx < end) end = nidx;
    }
    parts[k] = next.substring(0, end).trim();
    remaining = next.substring(end).trim();
  }
  return parts;
}

export default function RiskRules() {
  const [rules, setRules] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [collapsedDims, setCollapsedDims] = useState<Record<string, boolean>>({});
  const [editRule, setEditRule] = useState<any>(null);
  const [editWeight, setEditWeight] = useState(0);
  const [editThresholds, setEditThresholds] = useState('');

  useEffect(() => {
    (async () => {
      try {
        const res: any = await fetchRiskRules();
        setRules(res.data?.data || []);
      } catch (e) { console.error(e); }
      finally { setLoading(false); }
    })();
  }, []);

  const grouped: Record<string, any[]> = {};
  for (const r of rules) {
    const d = r.dimension || 'stock';
    if (!grouped[d]) grouped[d] = [];
    grouped[d].push(r);
  }

  const dimOrder = ['market', 'stock', 'portfolio', 'liquidity', 'event', 'behavior'];
  const totalRules = rules.length;
  const enabledRules = rules.filter(r => r.enabled).length;

  const handleToggle = async (key: string, enabled: boolean) => {
    try {
      await updateRiskRule(key, { enabled });
      setRules(prev => prev.map(r => r.ruleKey === key ? { ...r, enabled } : r));
      Message.success(enabled ? '已启用' : '已停用');
    } catch { Message.error('操作失败'); }
  };

  const openEdit = (rule: any) => {
    setEditRule(rule);
    setEditWeight(rule.weight || 0);
    setEditThresholds(JSON.stringify(rule.thresholds || {}, null, 2));
  };

  const handleSave = async () => {
    if (!editRule) return;
    try {
      let thresh: any;
      try { thresh = JSON.parse(editThresholds); } catch { Message.error('阈值JSON格式错误'); return; }
      await updateRiskRule(editRule.ruleKey, { weight: editWeight, thresholds: thresh });
      setRules(prev => prev.map(r => r.ruleKey === editRule.ruleKey ? { ...r, weight: editWeight, thresholds: thresh } : r));
      Message.success('已保存');
      setEditRule(null);
    } catch { Message.error('保存失败'); }
  };

  return (
    <div>
      {/* ═══ Page Header ═══ */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        marginBottom: 20, flexWrap: 'wrap', gap: 12,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{
            width: 42, height: 42, borderRadius: 12,
            background: 'linear-gradient(135deg, #165DFF, #722ED1)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <BookOpen size={21} color="#fff" />
          </div>
          <div>
            <div style={{ fontSize: 18, fontWeight: 700 }}>风险规则手册</div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
              共 {totalRules} 条规则 · {enabledRules} 条启用 · 覆盖 6 大风险维度
            </div>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          {dimOrder.map(d => {
            const cnt = grouped[d]?.length || 0;
            const meta = dimMeta[d];
            if (!meta) return null;
            return (
              <Tag key={d} style={{
                background: meta.bg, color: meta.color, border: 'none',
                borderRadius: 8, fontSize: 11, cursor: 'pointer',
              }}>
                {meta.label} {cnt}
              </Tag>
            );
          })}
        </div>
      </div>

      {/* ═══ Scoring Explanation ═══ */}
      <div style={{
        background: 'var(--color-bg-2)', borderRadius: 12,
        border: '1px solid var(--color-border-2)',
        padding: 16, marginBottom: 20,
        display: 'flex', alignItems: 'flex-start', gap: 14,
      }}>
        <Gauge size={20} color="#165DFF" style={{ marginTop: 2 }} />
        <div>
          <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 6 }}>综合评分计算说明</div>
          <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: 1.8 }}>
            市场风险综合评分（0-100）由 <strong>4 个因子</strong> 加权计算：
            <br />
            <span style={{ color: '#f53f3f' }}>① 市场预警</span>（上限45分）：市场维度高×15 + 中×8 + 低×2 计分 
            · <span style={{ color: '#ff7d00' }}>② 高风险占比</span>（上限30分）：高风险持仓数 ÷ 总持仓数 × 30
            <br />
            <span style={{ color: '#722ED1' }}>③ 中等风险量</span>（上限15分）：中等告警数 ÷ 3 × 5 
            · <span style={{ color: '#0fc6c2' }}>④ 预警覆盖率</span>（上限10分）：总告警 ÷ 总持仓 × 10
            <br />
            每条规则的 <strong>权重(weight)</strong> 影响规则本身在因子中的贡献度，权重越高告警越容易被触发。
          </div>
        </div>
      </div>

      {/* ═══ Rules by Dimension ═══ */}
      {dimOrder.map(dim => {
        const dimRules = grouped[dim] || [];
        if (dimRules.length === 0) return null;
        const meta = dimMeta[dim];
        const collapsed = collapsedDims[dim] || false;
        const enabledCount = dimRules.filter(r => r.enabled).length;

        return (
          <div key={dim} style={{ marginBottom: 16 }}>
            {/* Dimension header */}
            <div
              onClick={() => setCollapsedDims(prev => ({ ...prev, [dim]: !prev[dim] }))}
              style={{
                display: 'flex', alignItems: 'center', gap: 10, padding: '10px 0',
                cursor: 'pointer', userSelect: 'none',
              }}>
              <div style={{
                width: 32, height: 32, borderRadius: 8,
                background: meta.bg, display: 'flex',
                alignItems: 'center', justifyContent: 'center',
              }}>
                <meta.icon size={16} color={meta.color} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 15, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
                  {meta.label}风险
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontWeight: 400 }}>
                    {dimRules.length} 条规则 · {enabledCount} 启用
                  </span>
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{meta.desc}</div>
              </div>
              {collapsed ? <ChevronRight size={16} color="var(--color-text-3)" /> : <ChevronDown size={16} color="var(--color-text-3)" />}
            </div>

            {!collapsed && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {dimRules.map((rule: any) => {
                  const lv = levelMeta[rule.defaultLevel] || levelMeta.medium;
                  const parts = parseDesc(rule.description || '');
                  const enabled = rule.enabled;

                  return (
                    <div key={rule.ruleKey} style={{
                      background: 'var(--color-bg-2)',
                      borderRadius: 10,
                      border: `1px solid ${enabled ? 'var(--color-border-2)' : 'var(--color-border-1)'}`,
                      opacity: enabled ? 1 : 0.5,
                      overflow: 'hidden',
                    }}>
                      {/* Rule header */}
                      <div style={{
                        display: 'flex', alignItems: 'center', gap: 10,
                        padding: '12px 16px',
                        borderBottom: `1px solid var(--color-border-1)`,
                      }}>
                        <div style={{ width: 4, height: 18, borderRadius: 2, background: lv.color, flexShrink: 0 }} />
                        <span style={{ fontSize: 14, fontWeight: 600, flex: 1 }}>{rule.name}</span>
                        <span style={{
                          fontSize: 10, fontWeight: 600, color: lv.color,
                          background: lv.bg, padding: '2px 8px', borderRadius: 8,
                        }}>
                          {lv.label}
                        </span>
                        <Tooltip content="权重：影响该规则在评分中的贡献度">
                          <span style={{
                            fontSize: 10, color: 'var(--color-text-3)',
                            fontFamily: 'monospace', background: 'var(--color-fill-1)',
                            padding: '2px 6px', borderRadius: 6,
                          }}>
                            w={rule.weight?.toFixed(2) || '0.00'}
                          </span>
                        </Tooltip>
                        <Switch size="small" checked={enabled}
                          onChange={(c: boolean) => handleToggle(rule.ruleKey, c)} />
                        <Tooltip content="编辑阈值">
                          <Button size="mini" type="text" icon={<Settings size={13} />}
                            onClick={e => { e.stopPropagation(); openEdit(rule); }} />
                        </Tooltip>
                      </div>

                      {/* Rule body: structured criteria */}
                      <div style={{ padding: '12px 16px' }}>
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                          {/* Left column */}
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            {parts['数据来源'] && (
                              <div>
                                <div style={{ fontSize: 10, color: '#86909c', fontWeight: 600, marginBottom: 2 }}>📊 数据来源</div>
                                <div style={{ fontSize: 11, color: 'var(--color-text-1)', fontFamily: 'monospace', lineHeight: 1.5 }}>
                                  {parts['数据来源']}
                                </div>
                              </div>
                            )}
                            {parts['计算方式'] && (
                              <div>
                                <div style={{ fontSize: 10, color: '#86909c', fontWeight: 600, marginBottom: 2 }}>⚙️ 计算方式</div>
                                <div style={{ fontSize: 11, color: 'var(--color-text-1)', lineHeight: 1.6 }}>
                                  {parts['计算方式']}
                                </div>
                              </div>
                            )}
                          </div>
                          {/* Right column */}
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                            {parts['风控意义'] && (
                              <div>
                                <div style={{ fontSize: 10, color: '#86909c', fontWeight: 600, marginBottom: 2 }}>⚠️ 风控意义</div>
                                <div style={{ fontSize: 11, color: lv.color, lineHeight: 1.6, fontWeight: 500, background: lv.bg, padding: '6px 10px', borderRadius: 6 }}>
                                  {parts['风控意义']}
                                </div>
                              </div>
                            )}
                            {parts['阈值'] && (
                              <div>
                                <div style={{ fontSize: 10, color: '#86909c', fontWeight: 600, marginBottom: 2 }}>🎯 触发阈值</div>
                                <div style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace', background: 'var(--color-fill-1)', padding: '4px 8px', borderRadius: 4 }}>
                                  {parts['阈值']}
                                </div>
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}

      {/* Edit Drawer */}
      <Drawer
        title={<div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Settings size={16} />编辑规则 — {editRule?.name || ''}
        </div>}
        visible={!!editRule}
        onCancel={() => setEditRule(null)}
        onOk={handleSave}
        width={460}
      >
        {editRule && (
          <div>
            <div style={{
              padding: 12, borderRadius: 8, marginBottom: 16,
              background: 'var(--color-fill-1)', border: '1px solid var(--color-border-1)',
              fontSize: 13, color: 'var(--color-text-2)', lineHeight: 1.7,
            }}>
              {editRule.description || '暂无说明'}
            </div>
            <div style={{ marginBottom: 12 }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6 }}>权重</div>
              <InputNumber value={editWeight} min={0} max={1} step={0.001}
                onChange={(v: number) => setEditWeight(v)} style={{ width: '100%' }} />
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 4 }}>
                权重影响该规则在综合风险评分中的贡献度
              </div>
            </div>
            <div>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6 }}>阈值参数 (JSON)</div>
              <textarea value={editThresholds}
                onChange={e => setEditThresholds(e.target.value)}
                style={{
                  width: '100%', height: 200, fontFamily: 'monospace', fontSize: 12,
                  padding: 8, borderRadius: 6,
                  border: '1px solid var(--color-border-2)',
                  background: 'var(--color-fill-1)', color: 'var(--color-text-1)',
                }} />
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
