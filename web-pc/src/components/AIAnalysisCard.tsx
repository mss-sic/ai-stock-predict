// ============================================================================
// AIAnalysisCard — 流式混合输出 Widget 系统
// 支持类型: signal / risk / plan / list / alert / panel / summary
// ============================================================================

// ── Widget 类型定义 ──

interface SignalWidget {
  w: 'signal'; u: boolean; h: string; d: string;
}
interface RiskWidget {
  w: 'risk'; h: string; d: string;
}
interface PlanWidget {
  w: 'plan'; s: number; r: number; tip: string; pos: number;
}
interface ListWidget {
  w: 'list'; t?: string; items: string[];
}
interface AlertWidget {
  w: 'alert'; level: 'info' | 'warning' | 'danger'; title: string; body: string;
}
interface PanelWidget {
  w: 'panel'; t?: string; rows: { k: string; v: string }[];
}
interface SummaryWidget {
  w: 'summary'; label: string; text: string;
}

type Widget = SignalWidget | RiskWidget | PlanWidget | ListWidget | AlertWidget | PanelWidget | SummaryWidget;

// ── Widget 组件 ──

function SignalCard({ w }: { w: SignalWidget }) {
  const { u, h, d } = w;
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10, padding: '8px 12px', borderRadius: 6,
      background: u ? 'rgba(245,63,63,0.06)' : 'rgba(0,180,42,0.06)',
      border: `1px solid ${u ? 'rgba(245,63,63,0.15)' : 'rgba(0,180,42,0.15)'}` }}>
      <span style={{ fontSize: 14, flexShrink: 0 }}>{u ? '📈' : '📉'}</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: u ? 'var(--stock-up)' : 'var(--stock-down)' }}>{h}</div>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>{d}</div>
      </div>
      <span style={{ fontSize: 10, fontWeight: 700, padding: '2px 6px', borderRadius: 4,
        background: u ? 'rgba(245,63,63,0.12)' : 'rgba(0,180,42,0.12)',
        color: u ? 'var(--stock-up)' : 'var(--stock-down)' }}>{u ? '看多' : '看空'}</span>
    </div>
  );
}

function RiskCard({ w }: { w: RiskWidget }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10, padding: '6px 12px', borderRadius: 4,
      background: 'rgba(255,125,0,0.06)' }}>
      <span style={{ fontSize: 13, flexShrink: 0 }}>⚠️</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-warning-text)' }}>{w.h}</span>
        <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8 }}>{w.d}</span>
      </div>
    </div>
  );
}

function PlanCard({ w }: { w: PlanWidget }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10,
      background: 'var(--color-fill-2)', borderRadius: 8, padding: '12px 14px' }}>
      <div><div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>支撑位</div>
        <div style={{ fontSize: 18, fontWeight: 700, color: '#00B42A' }}>{w.s.toFixed(2)}</div></div>
      <div><div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>压力位</div>
        <div style={{ fontSize: 18, fontWeight: 700, color: '#F53F3F' }}>{w.r.toFixed(2)}</div></div>
      <div style={{ gridColumn: '1 / -1', fontSize: 12, color: 'var(--color-text-2)', marginTop: 4 }}>
        💡 {w.tip} · 建议仓位 <b style={{ color: 'var(--color-text-1)' }}>{w.pos}%</b></div>
      <div style={{ gridColumn: '1 / -1', fontSize: 10, color: 'var(--color-text-3)', textAlign: 'center' }}>
        ⚠️ 以上分析由AI生成，不构成投资建议</div>
    </div>
  );
}

function ListCard({ w }: { w: ListWidget }) {
  return (
    <div style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 14px',
      border: '1px solid var(--color-border-1)' }}>
      {w.t && <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 6 }}>📋 {w.t}</div>}
      <ul style={{ margin: 0, paddingLeft: 18, display: 'flex', flexDirection: 'column', gap: 3 }}>
        {w.items.map((item, i) => (
          <li key={i} style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: '20px' }}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

function AlertCard({ w }: { w: AlertWidget }) {
  const colors: Record<string, { bg: string; border: string; icon: string; text: string }> = {
    info:    { bg: 'rgba(22,93,255,0.06)', border: 'rgba(22,93,255,0.18)', icon: 'ℹ️', text: 'var(--arcoblue-6)' },
    warning: { bg: 'rgba(255,125,0,0.06)',  border: 'rgba(255,125,0,0.18)',  icon: '⚠️', text: 'var(--color-warning-text)' },
    danger:  { bg: 'rgba(245,63,63,0.06)',  border: 'rgba(245,63,63,0.18)',  icon: '🚨', text: 'var(--stock-down)' },
  };
  const c = colors[w.level] || colors.info;
  return (
    <div style={{ background: c.bg, border: `1px solid ${c.border}`, borderRadius: 8, padding: '10px 14px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
        <span>{c.icon}</span>
        <span style={{ fontSize: 13, fontWeight: 600, color: c.text }}>{w.title}</span>
      </div>
      <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: '20px' }}>{w.body}</div>
    </div>
  );
}

function PanelCard({ w }: { w: PanelWidget }) {
  return (
    <div style={{ background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)', overflow: 'hidden' }}>
      {w.t && <div style={{ padding: '8px 14px', fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)',
        background: 'var(--color-fill-2)', borderBottom: '1px solid var(--color-border-1)' }}>📊 {w.t}</div>}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(120px, 1fr))', gap: 1 }}>
        {w.rows.map((row, i) => (
          <div key={i} style={{ padding: '8px 14px', textAlign: 'center' }}>
            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>{row.k}</div>
            <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>{row.v}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function SummaryCard({ w }: { w: SummaryWidget }) {
  const bullish = ['看多', '强烈看多', '短线看多'];
  const bearish = ['看空', '强烈看空', '短线看空'];
  const isUp = bullish.includes(w.label);
  const isDown = bearish.includes(w.label);
  const accent = isUp ? '#F53F3F' : isDown ? '#00B42A' : 'var(--color-text-2)';
  const bg = isUp ? 'rgba(245,63,63,0.06)' : isDown ? 'rgba(0,180,42,0.06)' : 'var(--color-fill-2)';
  return (
    <div style={{ background: `linear-gradient(135deg, ${bg}, var(--color-fill-1))`, borderRadius: 8,
      padding: '12px 14px', border: `1px solid var(--color-border-1)` }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
        <span style={{ fontSize: 14 }}>🎯</span>
        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>综合判断</span>
        <span style={{ marginLeft: 'auto', fontSize: 11, fontWeight: 700, padding: '2px 8px', borderRadius: 4,
          background: isUp ? 'rgba(245,63,63,0.12)' : isDown ? 'rgba(0,180,42,0.12)' : 'var(--color-fill-3)',
          color: accent }}>{w.label}</span>
      </div>
      <div style={{ fontSize: 13, lineHeight: '22px', color: 'var(--color-text-2)' }}>{w.text}</div>
    </div>
  );
}

// ── Widget 路由 ──

function WidgetRenderer({ w }: { w: Widget }) {
  switch (w.w) {
    case 'signal':  return <SignalCard w={w} />;
    case 'risk':    return <RiskCard w={w} />;
    case 'plan':    return <PlanCard w={w} />;
    case 'list':    return <ListCard w={w} />;
    case 'alert':   return <AlertCard w={w} />;
    case 'panel':   return <PanelCard w={w} />;
    case 'summary': return <SummaryCard w={w} />;
    default: return null;
  }
}

// ── 流式解析器 ──

interface Section {
  type: 'text' | 'widget';
  content: string;
  key: string;
}

export function parseStreamSections(text: string, _prev: number): Section[] {
  const sections: Section[] = [];
  // Match all widget JSON types
  const widgetRe = /\{(?=[^{]*"w"\s*:\s*"(signal|risk|plan|list|alert|panel|summary)")[^}]*\}/g;
  const matches: { start: number; end: number; json: string }[] = [];
  let m;
  while ((m = widgetRe.exec(text)) !== null) {
    matches.push({ start: m.index, end: m.index + m[0].length, json: m[0] });
  }

  let pos = 0, idx = 0;
  for (const match of matches) {
    const before = text.slice(pos, match.start).trim();
    if (before) sections.push({ type: 'text', content: before, key: `t-${idx++}` });
    const w = tryParseWidget(match.json);
    if (w) {
      sections.push({ type: 'widget', content: match.json, key: `w-${idx++}` });
    } else {
      sections.push({ type: 'text', content: match.json, key: `t-${idx++}` });
    }
    pos = match.end;
  }
  const remaining = text.slice(pos).trim();
  if (remaining) sections.push({ type: 'text', content: remaining, key: `t-${idx++}` });
  return sections;
}

export function tryParseWidget(json: string): Widget | null {
  try {
    const obj = JSON.parse(json);
    switch (obj.w) {
      case 'signal':
        if (typeof obj.u === 'boolean' && obj.h && obj.d) return obj as SignalWidget;
        break;
      case 'risk':
        if (obj.h && obj.d) return obj as RiskWidget;
        break;
      case 'plan':
        if (typeof obj.s === 'number' && typeof obj.r === 'number') return obj as PlanWidget;
        break;
      case 'list':
        if (Array.isArray(obj.items)) return obj as ListWidget;
        break;
      case 'alert':
        if (obj.level && obj.title && obj.body) return obj as AlertWidget;
        break;
      case 'panel':
        if (Array.isArray(obj.rows)) return obj as PanelWidget;
        break;
      case 'summary':
        if (obj.label && obj.text) return obj as SummaryWidget;
        break;
    }
  } catch {}
  return null;
}

export { WidgetRenderer };
