// ============================================================================
// AIAnalysisCard — 流式混合输出 Widget 系统（金融专业版）
// 支持类型: signal / risk / plan / list / alert / panel / summary
// ============================================================================

import ReactMarkdown from 'react-markdown';

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

// ── 共享样式常量 ──

const cardBase: React.CSSProperties = {
  borderRadius: 8,
  overflow: 'hidden',
};

const mutedLabel: React.CSSProperties = {
  fontSize: 11, fontWeight: 500, color: 'var(--color-text-3)',
  textTransform: 'uppercase', letterSpacing: '0.5px',
};

const accentDot = (color: string): React.CSSProperties => ({
  width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
  background: color,
});

// ── Widget 组件 ──

function SignalCard({ w }: { w: SignalWidget }) {
  const { u, h, d } = w;
  const color = u ? 'var(--stock-up)' : 'var(--stock-down)';
  const softBg = u ? 'var(--stock-up-soft)' : 'var(--stock-down-soft)';
  const borderColor = u ? 'rgba(245,63,63,0.18)' : 'rgba(0,180,42,0.18)';
  return (
    <div style={{
      ...cardBase,
      background: softBg,
      border: `1px solid ${borderColor}`,
      borderLeft: `3px solid ${color}`,
      padding: '10px 14px',
      display: 'flex', alignItems: 'center', gap: 12,
    }}>
      <div style={{
        width: 36, height: 36, borderRadius: 8,
        background: u ? 'linear-gradient(135deg, rgba(245,63,63,0.15), rgba(245,63,63,0.05))'
                       : 'linear-gradient(135deg, rgba(0,180,42,0.15), rgba(0,180,42,0.05))',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 16, lineHeight: 1 }}>
          {u ? '↑' : '↓'}
        </span>
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 14, fontWeight: 700, color, lineHeight: '20px' }}>{h}</div>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', lineHeight: '18px', marginTop: 1 }}>{d}</div>
      </div>
      <span style={{
        fontSize: 11, fontWeight: 700, padding: '3px 10px', borderRadius: 20,
        background: u ? 'rgba(245,63,63,0.15)' : 'rgba(0,180,42,0.15)',
        color, whiteSpace: 'nowrap', flexShrink: 0,
        border: `1px solid ${u ? 'rgba(245,63,63,0.3)' : 'rgba(0,180,42,0.3)'}`,
      }}>{u ? '看多' : '看空'}</span>
    </div>
  );
}

function RiskCard({ w }: { w: RiskWidget }) {
  const accent = '#F59E0B';
  return (
    <div style={{
      ...cardBase,
      background: 'var(--color-warning-bg)',
      border: '1px solid var(--color-warning-border)',
      borderLeft: `3px solid ${accent}`,
      padding: '10px 14px',
      display: 'flex', alignItems: 'flex-start', gap: 10,
    }}>
      <div style={{
        width: 22, height: 22, borderRadius: '50%',
        background: 'rgba(245,158,11,0.15)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 11, fontWeight: 700, color: accent }}>!</span>
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-warning-text)', lineHeight: '19px' }}>{w.h}</div>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', lineHeight: '18px', marginTop: 2 }}>{w.d}</div>
      </div>
    </div>
  );
}

function PlanCard({ w }: { w: PlanWidget }) {
  const posPct = Math.min(100, Math.max(0, w.pos));
  const barColor = posPct > 70 ? '#F53F3F' : posPct > 40 ? '#F59E0B' : '#00B42A';
  return (
    <div style={{
      ...cardBase,
      background: 'var(--color-fill-1)',
      border: '1px solid var(--color-border-2)',
      padding: '14px 16px',
    }}>
      {/* Title */}
      <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 12, display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ width: 4, height: 14, borderRadius: 2, background: 'var(--color-primary)', display: 'inline-block' }} />
        交易计划参考
      </div>
      {/* Support / Resistance */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, marginBottom: 12 }}>
        <div style={{
          background: 'rgba(0,180,42,0.04)', borderRadius: 8, padding: '10px 14px',
          border: '1px solid rgba(0,180,42,0.1)',
        }}>
          <div style={mutedLabel}>支撑位</div>
          <div style={{ fontSize: 22, fontWeight: 800, color: '#00B42A', lineHeight: '28px' }}>{w.s.toFixed(2)}</div>
        </div>
        <div style={{
          background: 'rgba(245,63,63,0.04)', borderRadius: 8, padding: '10px 14px',
          border: '1px solid rgba(245,63,63,0.1)',
        }}>
          <div style={mutedLabel}>压力位</div>
          <div style={{ fontSize: 22, fontWeight: 800, color: '#F53F3F', lineHeight: '28px' }}>{w.r.toFixed(2)}</div>
        </div>
      </div>
      {/* Position bar */}
      <div style={{ marginBottom: 10 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 5 }}>
          <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{w.tip}</span>
          <span style={{ fontSize: 12, fontWeight: 700, color: barColor }}>建议仓位 {w.pos}%</span>
        </div>
        <div style={{
          height: 6, borderRadius: 3, background: 'var(--color-fill-2)',
          overflow: 'hidden',
        }}>
          <div style={{
            height: '100%', width: `${posPct}%`, borderRadius: 3,
            background: `linear-gradient(90deg, ${barColor}, ${barColor}cc)`,
            transition: 'width 0.4s ease',
          }} />
        </div>
      </div>
      {/* Disclaimer */}
      <div style={{ fontSize: 10, color: 'var(--color-text-3)', textAlign: 'center', opacity: 0.7 }}>
        ⚠ 以上分析由AI生成，不构成投资建议
      </div>
    </div>
  );
}

function ListCard({ w }: { w: ListWidget }) {
  return (
    <div style={{
      ...cardBase,
      background: 'var(--color-fill-1)',
      border: '1px solid var(--color-border-1)',
      padding: '12px 16px',
    }}>
      {w.t && (
        <div style={{
          fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)',
          marginBottom: 10, display: 'flex', alignItems: 'center', gap: 6,
        }}>
          <span style={accentDot('var(--color-primary)')} />
          {w.t}
        </div>
      )}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {w.items.map((item, i) => (
          <div key={i} style={{
            display: 'flex', alignItems: 'flex-start', gap: 8,
            fontSize: 13, color: 'var(--color-text-2)', lineHeight: '20px',
          }}>
            <span style={{
              flexShrink: 0, width: 18, height: 18, borderRadius: '50%',
              background: 'var(--color-fill-2)', fontSize: 10, fontWeight: 600,
              color: 'var(--color-text-3)', display: 'flex', alignItems: 'center',
              justifyContent: 'center', marginTop: 1,
            }}>{i + 1}</span>
            <span>{item}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function AlertCard({ w }: { w: AlertWidget }) {
  const config: Record<string, { bg: string; border: string; accent: string; icon: string }> = {
    info:    { bg: 'rgba(59,130,246,0.04)', border: 'rgba(59,130,246,0.15)', accent: '#3B82F6', icon: 'i' },
    warning: { bg: 'rgba(245,158,11,0.04)', border: 'rgba(245,158,11,0.15)', accent: '#F59E0B', icon: '!' },
    danger:  { bg: 'rgba(239,68,68,0.04)',  border: 'rgba(239,68,68,0.15)',  accent: '#EF4444', icon: '!' },
  };
  const style = config[w.level] || config.info;
  return (
    <div style={{
      ...cardBase,
      background: style.bg,
      border: `1px solid ${style.border}`,
      borderLeft: `3px solid ${style.accent}`,
      padding: '10px 14px',
      display: 'flex', gap: 10,
    }}>
      <div style={{
        width: 24, height: 24, borderRadius: '50%',
        background: `${style.accent}22`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        flexShrink: 0,
      }}>
        <span style={{ fontSize: 12, fontWeight: 800, color: style.accent }}>{style.icon}</span>
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', lineHeight: '19px' }}>{w.title}</div>
        <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: '18px', marginTop: 3 }}>{w.body}</div>
      </div>
    </div>
  );
}

function PanelCard({ w }: { w: PanelWidget }) {
  return (
    <div style={{
      ...cardBase,
      background: 'var(--color-fill-1)',
      border: '1px solid var(--color-border-1)',
    }}>
      {w.t && (
        <div style={{
          fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)',
          padding: '10px 16px', borderBottom: '1px solid var(--color-border-1)',
          display: 'flex', alignItems: 'center', gap: 6,
        }}>
          <span style={accentDot('var(--color-primary)')} />
          {w.t}
        </div>
      )}
      <div style={{ padding: '4px 0' }}>
        {w.rows.map((row, i) => (
          <div key={i} style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            padding: '7px 16px',
            background: i % 2 === 0 ? 'transparent' : 'var(--color-fill-2)',
          }}>
            <span style={{ fontSize: 12, color: 'var(--color-text-3)', fontWeight: 500 }}>{row.k}</span>
            <span style={{
              fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)',
              fontFamily: "'SF Mono', 'Menlo', 'Monaco', monospace",
            }}>{row.v}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function SummaryCard({ w }: { w: SummaryWidget }) {
  const isUp = w.label.includes('看多') || w.label.includes('买入') || w.label.includes('增持');
  const isDown = w.label.includes('看空') || w.label.includes('卖出') || w.label.includes('减持');
  const accent = isUp ? 'var(--stock-up)' : isDown ? 'var(--stock-down)' : 'var(--color-primary)';
  const bg = isUp ? 'var(--stock-up-soft)' : isDown ? 'var(--stock-down-soft)' : 'var(--color-fill-2)';
  return (
    <div style={{
      ...cardBase,
      background: `linear-gradient(135deg, ${bg}, var(--color-fill-1))`,
      border: `1px solid var(--color-border-2)`,
      padding: '14px 16px',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
        <div style={{
          width: 32, height: 32, borderRadius: 8,
          background: `${accent}18`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <span style={{ fontSize: 16 }}>{isUp ? '↑' : isDown ? '↓' : '◆'}</span>
        </div>
        <span style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-1)' }}>综合判断</span>
        <span style={{
          marginLeft: 'auto', fontSize: 11, fontWeight: 700, padding: '4px 10px', borderRadius: 20,
          background: `${accent}20`, color: accent,
          border: `1px solid ${accent}40`,
        }}>{w.label}</span>
      </div>
      <div style={{
        fontSize: 13, lineHeight: '22px', color: 'var(--color-text-2)',
        padding: '8px 12px', borderRadius: 6, background: 'var(--color-fill-2)',
        border: '1px solid var(--color-border-1)',
      }}>{w.text}</div>
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
  const matches: { start: number; end: number; json: string }[] = [];
  const widgetTagRe = /"(?:w|type)"\s*:\s*"(signal|risk|plan|list|alert|panel|summary)"/g;
  let tagMatch;
  while ((tagMatch = widgetTagRe.exec(text)) !== null) {
    let start = tagMatch.index;
    while (start > 0 && text[start] !== '{') start--;
    if (text[start] !== '{') continue;
    let depth = 0;
    let end = start;
    for (let i = start; i < text.length; i++) {
      if (text[i] === '{') depth++;
      else if (text[i] === '}') {
        depth--;
        if (depth === 0) { end = i + 1; break; }
      }
    }
    if (end > start) {
      const json = text.slice(start, end);
      if (!matches.some(m => m.start === start)) {
        matches.push({ start, end, json });
      }
    }
  }
  matches.sort((a, b) => a.start - b.start);

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
    // Normalize: AI may use "type" instead of "w"
    if (!obj.w && obj.type) obj.w = obj.type;
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
