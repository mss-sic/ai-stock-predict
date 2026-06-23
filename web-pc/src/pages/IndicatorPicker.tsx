import React, { useMemo, useState } from 'react';
import { Select, Input } from '@arco-design/web-react';
import { Search } from 'lucide-react';

interface IndicatorMeta {
  key: string;
  label: string;
  category?: string;
  type?: string;
  desc?: string;
  dataNote?: string;
  backtestSafe?: boolean;
}

interface Props {
  value: string;
  onChange: (key: string) => void;
  indicators: IndicatorMeta[];
  size?: 'small' | 'mini' | 'default';
  style?: React.CSSProperties;
}

const CAT_ICONS: Record<string, string> = {
  '榜单与评分': '🏆',
  'AI评分': '🤖',
  '技术面-趋势': '📈',
  '技术面-超买超卖': '📊',
  '技术面-量价': '📉',
  '技术面-形态': '🔍',
  '估值': '💰',
  '基本面': '🏢',
  '资金面': '💵',
  '预测': '🔮',
};

const CAT_ORDER = ['榜单与评分', 'AI评分', '技术面-趋势', '技术面-超买超卖', '技术面-量价', '技术面-形态', '估值', '基本面', '资金面', '预测'];

/** Categorized indicator picker with search. */
const IndicatorPicker: React.FC<Props> = ({ value, onChange, indicators, size = 'small', style }) => {
  const [search, setSearch] = useState('');
  const [open, setOpen] = useState(false);

  const grouped = useMemo(() => {
    const map: Record<string, IndicatorMeta[]> = {};
    for (const ind of indicators) {
      const cat = ind.category || '其他';
      if (!map[cat]) map[cat] = [];
      map[cat].push(ind);
    }
    // Sort categories
    const ordered: [string, IndicatorMeta[]][] = [];
    for (const c of CAT_ORDER) {
      if (map[c]) ordered.push([c, map[c]]);
    }
    for (const c of Object.keys(map)) {
      if (!CAT_ORDER.includes(c)) ordered.push([c, map[c]]);
    }
    return ordered;
  }, [indicators]);

  const filtered = useMemo(() => {
    if (!search.trim()) return grouped;
    const q = search.toLowerCase();
    return grouped.map(([cat, items]) => [
      cat,
      items.filter(i => i.label.toLowerCase().includes(q) || i.key.toLowerCase().includes(q)),
    ] as [string, IndicatorMeta[]]).filter(([, items]) => items.length > 0);
  }, [grouped, search]);

  const selectedLabel = indicators.find(i => i.key === value)?.label || value;

  return (
    <Select
      value={value}
      onChange={onChange}
      style={style}
      size={size as any}
      placeholder="选择指标"
      triggerProps={{ autoAlignPopupWidth: false }}
      onVisibleChange={v => { setOpen(v); if (!v) setSearch(''); }}
      renderFormat={() => selectedLabel}
    >
      {open && (
        <div style={{ padding: '0 4px' }}>
          <Input
            prefix={<Search size={12} style={{ color: 'var(--color-text-3)' }} />}
            placeholder="搜索指标..."
            value={search}
            onChange={setSearch}
            size="mini"
            style={{ marginBottom: 6 }}
            allowClear
          />
        </div>
      )}
      {filtered.length === 0 ? (
        <Select.Option value="" disabled>无匹配指标</Select.Option>
      ) : (
        filtered.map(([cat, items]) => (
          <Select.OptGroup key={cat} label={`${CAT_ICONS[cat] || '📌'} ${cat}`}>
            {items.map(ind => (
              <Select.Option key={ind.key} value={ind.key}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span style={{ fontSize: 10 }}>
                    {ind.backtestSafe ? '🟢' : (ind.dataNote?.startsWith('🚫') ? '🚫' : '🟡')}
                  </span>
                  <span>{ind.label}</span>
                  {ind.desc && (
                    <span style={{ fontSize: 10, color: 'var(--color-text-3)', marginLeft: 4, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      — {ind.desc}
                    </span>
                  )}
                </span>
              </Select.Option>
            ))}
          </Select.OptGroup>
        ))
      )}
    </Select>
  );
};

export default IndicatorPicker;
