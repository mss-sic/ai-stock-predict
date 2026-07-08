import { useState, useEffect, useMemo } from 'react';
import { Drawer, Tabs } from '@arco-design/web-react';
import { BookOpen, Gauge, TrendingDown, AlertTriangle, Layers, Droplets, FileText, UserCheck, HelpCircle } from 'lucide-react';
import { fetchRiskRules } from '../services/api';

const dimMeta: Record<string, { label: string; icon: any; color: string }> = {
  market:    { label: '市场', icon: TrendingDown, color: '#165DFF' },
  stock:     { label: '个股', icon: AlertTriangle, color: '#722ED1' },
  portfolio: { label: '组合', icon: Layers, color: '#ff7d00' },
  liquidity: { label: '流动性', icon: Droplets, color: '#0fc6c2' },
  event:     { label: '事件', icon: FileText, color: '#f53f3f' },
  behavior:  { label: '行为', icon: UserCheck, color: '#86909c' },
};

const levelBadge: Record<string, { label: string; color: string; bg: string }> = {
  high:   { label: '高风险', color: '#f53f3f', bg: '#f53f3f12' },
  medium: { label: '中风险', color: '#ff7d00', bg: '#ff7d0012' },
  low:    { label: '低风险', color: '#00b42a', bg: '#00b42a12' },
};

function parseDesc(desc: string) {
  const parts: Record<string, string> = {};
  const keys = ['数据来源', '计算方式', '风控意义', '阈值'];
  let remaining = desc || '';
  for (const k of keys) {
    const prefix = k + ':';
    const idx = remaining.indexOf(prefix);
    if (idx === -1) { parts[k] = remaining.trim(); remaining = ''; break; }
    const next = remaining.substring(idx + prefix.length);
    let end = next.length;
    for (const nk of keys) { const nidx = next.indexOf(nk + ':'); if (nidx !== -1 && nidx < end) end = nidx; }
    parts[k] = next.substring(0, end).trim();
    remaining = next.substring(end).trim();
  }
  return parts;
}

interface Props {
  visible: boolean;
  onClose: () => void;
  initialSection?: string; // 'scoring' | 'rules'
}

export default function HelpDrawer({ visible, onClose, initialSection }: Props) {
  const [rules, setRules] = useState<any[]>([]);
  const [activeTab, setActiveTab] = useState(initialSection === 'rules' ? 'rules' : 'scoring');

  useEffect(() => {
    if (initialSection) setActiveTab(initialSection === 'rules' ? 'rules' : 'scoring');
  }, [initialSection, visible]);

  useEffect(() => {
    if (!visible) return;
    fetchRiskRules().then((res: any) => setRules(res.data?.data || [])).catch(() => {});
  }, [visible]);

  const grouped: Record<string, any[]> = useMemo(() => {
    const g: Record<string, any[]> = {};
    for (const r of rules) { const d = r.dimension || 'stock'; if (!g[d]) g[d] = []; g[d].push(r); }
    return g;
  }, [rules]);

  const dimOrder = ['market', 'stock', 'portfolio', 'liquidity', 'event', 'behavior'];

  return (
    <Drawer
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{
            width: 32, height: 32, borderRadius: 8,
            background: 'linear-gradient(135deg, #165DFF, #722ED1)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <BookOpen size={16} color="#fff" />
          </div>
          <span style={{ fontWeight: 600, fontSize: 15 }}>帮助文档</span>
        </div>
      }
      visible={visible}
      onCancel={onClose}
      footer={null}
      width={640}
    >
      <Tabs activeTab={activeTab} onChange={setActiveTab} style={{ marginTop: -8 }}>
        {/* ═══ Tab 1: Scoring ═══ */}
        <Tabs.TabPane key="scoring" title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Gauge size={14} /> 评分说明
          </span>
        }>
          <div style={{ padding: '8px 0' }}>
            <h4 style={{ margin: '0 0 12px', fontSize: 15, fontWeight: 600 }}>市场风险综合评分（0-100）</h4>
            <p style={{ fontSize: 13, color: 'var(--color-text-2)', lineHeight: 1.8, margin: '0 0 20px' }}>
              综合评分由 <strong>4 个因子</strong> 加权求和得出，反映当前持仓组合面临的整体风险水平。
            </p>

            {[
              { name: '① 市场预警', max: 45, formula: '高×15 + 中×8 + 低×2', desc: '当前活跃的市场维度风险告警加权计分。市场维度告警反映系统性风险。', color: '#f53f3f' },
              { name: '② 高风险占比', max: 30, formula: '高险持仓数 ÷ 总持仓 × 30', desc: '高风险个股在你的持仓中的覆盖比例。4只持仓全部高风险=30分满分。', color: '#ff7d00' },
              { name: '③ 中等风险量', max: 15, formula: '中等告警数 ÷ 3 × 5', desc: '中等风险告警的密度。多条中等告警可能预示风险正在累积。', color: '#722ED1' },
              { name: '④ 预警覆盖率', max: 10, formula: '总告警数 ÷ 总持仓 × 10', desc: '告警在持仓中的覆盖率。每只持仓都有告警=10分满分。', color: '#0fc6c2' },
            ].map((f, i) => (
              <div key={i} style={{
                background: 'var(--color-bg-2)', borderRadius: 8,
                border: `1px solid var(--color-border-1)`, padding: '12px 16px', marginBottom: 10,
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: f.color }}>{f.name}</span>
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                    上限 {f.max} 分
                  </span>
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', fontFamily: 'monospace', marginBottom: 4 }}>
                  公式: {f.formula}
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: 1.6 }}>
                  {f.desc}
                </div>
              </div>
            ))}

            <h4 style={{ margin: '20px 0 10px', fontSize: 15, fontWeight: 600 }}>风险等级划分</h4>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {[
                { range: '0-24', level: '低风险', color: '#00b42a', desc: '正常操作', advice: '保持当前策略，持续监控' },
                { range: '25-49', level: '中风险', color: '#ff7d00', desc: '谨慎加仓', advice: '控制仓位在70%以内，检查高风险持仓' },
                { range: '50-74', level: '高风险', color: '#f53f3f', desc: '减仓观望', advice: '减仓至50%以下，优先降低高β值持仓' },
                { range: '75-100', level: '危险', color: '#cb2ecb', desc: '暂停买入', advice: '暂停所有买入操作，检查止损位' },
              ].map((l, i) => (
                <div key={i} style={{
                  flex: '1 1 130px', minWidth: 120,
                  background: `${l.color}0d`, borderRadius: 8,
                  border: `1px solid ${l.color}20`, padding: '10px 12px',
                  textAlign: 'center',
                }}>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace', marginBottom: 4 }}>
                    {l.range} 分
                  </div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: l.color, marginBottom: 2 }}>{l.level}</div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-2)' }}>{l.desc}</div>
                  <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 4, lineHeight: 1.4 }}>
                    💡 {l.advice}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </Tabs.TabPane>

        {/* ═══ Tab 2: Rules ═══ */}
        <Tabs.TabPane key="rules" title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <AlertTriangle size={14} /> 风险规则 ({rules.length})
          </span>
        }>
          <div style={{ padding: '8px 0' }}>
            <p style={{ fontSize: 13, color: 'var(--color-text-2)', lineHeight: 1.7, margin: '0 0 16px' }}>
              共 <strong>{rules.length}</strong> 条风险检测规则，覆盖 6 大风险维度。
              每条规则标注了数据来源、计算方式和风控意义。
            </p>

            {dimOrder.map(dim => {
              const dr = grouped[dim] || [];
              if (dr.length === 0) return null;
              const meta = dimMeta[dim];
              return (
                <div key={dim} style={{ marginBottom: 16 }}>
                  <div style={{
                    display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8,
                    padding: '6px 10px', borderRadius: 6,
                    background: `${meta.color}0d`,
                  }}>
                    <meta.icon size={14} color={meta.color} />
                    <span style={{ fontSize: 13, fontWeight: 600, color: meta.color }}>{meta.label}风险</span>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{dr.length} 条</span>
                  </div>
                  {dr.map((rule: any) => {
                    const lv = levelBadge[rule.defaultLevel] || levelBadge.medium;
                    const parts = parseDesc(rule.description || '');
                    return (
                      <div key={rule.ruleKey} style={{
                        background: 'var(--color-bg-2)', borderRadius: 6,
                        border: '1px solid var(--color-border-1)',
                        padding: '8px 12px', marginBottom: 6,
                        opacity: rule.enabled ? 1 : 0.45,
                      }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                          <span style={{ fontSize: 13, fontWeight: 600 }}>{rule.name}</span>
                          <span style={{ fontSize: 10, color: lv.color, background: lv.bg, padding: '1px 6px', borderRadius: 6, fontWeight: 600 }}>
                            {lv.label}
                          </span>
                          <span style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                            w={rule.weight?.toFixed(2)}
                          </span>
                        </div>
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6 }}>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                            <span style={{ color: '#86909c' }}>来源:</span> {parts['数据来源'] || '—'}
                          </div>
                          <div style={{ fontSize: 11, color: lv.color }}>
                            <span style={{ color: '#86909c' }}>意义:</span> {parts['风控意义'] || '—'}
                          </div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                            <span style={{ color: '#86909c' }}>方式:</span> {parts['计算方式'] || '—'}
                          </div>
                          <div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                            {parts['阈值'] || ''}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              );
            })}
          </div>
        </Tabs.TabPane>
      </Tabs>
    </Drawer>
  );
}
