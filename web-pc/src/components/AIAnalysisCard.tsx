import ReactMarkdown from 'react-markdown';

interface Signal {
  type: 'up' | 'down';
  title: string;
  desc: string;
}

interface Risk {
  title: string;
  desc: string;
}

interface AnalysisData {
  summary: string;
  label: string;
  signals: Signal[];
  risks: Risk[];
  support: number;
  resistance: number;
  suggestion: string;
  position: number;
}

function SignalRow({ type, title, desc }: Signal) {
  const isUp = type === 'up';
  return (
    <div style={{
      display: 'flex', alignItems: 'flex-start', gap: 10,
      padding: '8px 12px', borderRadius: 6,
      background: isUp ? 'rgba(245,63,63,0.06)' : 'rgba(0,180,42,0.06)',
      border: `1px solid ${isUp ? 'rgba(245,63,63,0.15)' : 'rgba(0,180,42,0.15)'}`,
    }}>
      <span style={{ fontSize: 14, flexShrink: 0, marginTop: 1 }}>
        {isUp ? '📈' : '📉'}
      </span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{
          fontSize: 13, fontWeight: 600,
          color: isUp ? 'var(--stock-up)' : 'var(--stock-down)',
        }}>{title}</div>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>{desc}</div>
      </div>
      <span style={{
        fontSize: 10, fontWeight: 700, padding: '2px 8px', borderRadius: 4,
        background: isUp ? 'rgba(245,63,63,0.12)' : 'rgba(0,180,42,0.12)',
        color: isUp ? 'var(--stock-up)' : 'var(--stock-down)',
      }}>{isUp ? '看多' : '看空'}</span>
    </div>
  );
}

function RiskRow({ title, desc }: Risk) {
  return (
    <div style={{
      display: 'flex', alignItems: 'flex-start', gap: 10,
      padding: '6px 12px', borderRadius: 4,
      background: 'rgba(255,125,0,0.06)',
    }}>
      <span style={{ fontSize: 13, flexShrink: 0 }}>⚠️</span>
      <div style={{ flex: 1 }}>
        <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-warning-text)' }}>{title}</span>
        <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8 }}>{desc}</span>
      </div>
    </div>
  );
}

function getLabelStyle(label: string) {
  const bullish = ['强烈看多', '短线看多', '看多'];
  const bearish = ['强烈看空', '短线看空', '看空'];
  if (bullish.includes(label)) return { bg: 'rgba(245,63,63,0.12)', color: '#F53F3F' };
  if (bearish.includes(label)) return { bg: 'rgba(0,180,42,0.12)', color: '#00B42A' };
  return { bg: 'var(--color-fill-2)', color: 'var(--color-text-2)' };
}

export default function AIAnalysisCard({ data }: { data: AnalysisData }) {
  const lbl = getLabelStyle(data.label);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      {/* Summary block */}
      <div style={{
        background: 'linear-gradient(135deg, rgba(22,93,255,0.06), rgba(114,46,209,0.06))',
        borderRadius: 8, padding: '12px 14px',
        border: '1px solid var(--color-border-1)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <span style={{ fontSize: 14 }}>🎯</span>
          <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--color-text-1)' }}>综合判断</span>
          <span style={{
            marginLeft: 'auto', fontSize: 12, fontWeight: 700,
            padding: '2px 10px', borderRadius: 4,
            background: lbl.bg, color: lbl.color,
          }}>{data.label}</span>
        </div>
        <div style={{ fontSize: 13, lineHeight: '22px', color: 'var(--color-text-2)' }}>
          {data.summary}
        </div>
      </div>

      {/* Key signals */}
      <div>
        <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 8, color: 'var(--color-text-1)' }}>
          📌 关键信号
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {data.signals.map((s, i) => (
            <SignalRow key={i} {...s} />
          ))}
        </div>
      </div>

      {/* Risks */}
      {data.risks.length > 0 && (
        <div>
          <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, color: 'var(--color-text-1)' }}>
            ⚠️ 风险提示
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            {data.risks.map((r, i) => (
              <RiskRow key={i} {...r} />
            ))}
          </div>
        </div>
      )}

      {/* Trading plan */}
      <div style={{
        display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10,
        background: 'var(--color-fill-2)', borderRadius: 8, padding: '12px 14px',
      }}>
        <div>
          <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>支撑位</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: '#00B42A' }}>{data.support?.toFixed(2)}</div>
        </div>
        <div>
          <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>压力位</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: '#F53F3F' }}>{data.resistance?.toFixed(2)}</div>
        </div>
        <div style={{ gridColumn: '1 / -1', fontSize: 12, color: 'var(--color-text-2)', marginTop: 4 }}>
          💡 {data.suggestion} · 建议仓位 <b style={{ color: 'var(--color-text-1)' }}>{data.position}%</b>
        </div>
      </div>

      <div style={{ fontSize: 10, color: 'var(--color-text-3)', textAlign: 'center' }}>
        ⚠️ 以上分析由AI生成，不构成投资建议
      </div>
    </div>
  );
}

// Helper to try parsing AI response as structured JSON
export function tryParseAnalysis(text: string): AnalysisData | null {
  try {
    // Try direct parse
    const d = JSON.parse(text);
    if (d.summary && d.signals && d.label) {
      return d as AnalysisData;
    }
  } catch {}
  // Try extracting JSON from markdown code block
  const match = text.match(/```(?:json)?\s*\n?([\s\S]*?)\n?```/);
  if (match) {
    try {
      const d = JSON.parse(match[1]);
      if (d.summary && d.signals && d.label) return d as AnalysisData;
    } catch {}
  }
  return null;
}
