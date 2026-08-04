import { useState, useMemo } from 'react';
import { Table, Tag, Select, Input, Tooltip } from '@arco-design/web-react';
import { Search } from 'lucide-react';

const { Option } = Select;

interface IndicatorItem {
  key: string;
  label: string;
  category: string;
  value: number;
  unit: string;
  desc: string;
  zero: boolean;
  noData: boolean;
}

interface Props {
  data: IndicatorItem[];
  loading?: boolean;
  dates?: string[];
  selectedDate?: string;
  onDateChange?: (date: string) => void;
  currentDate?: string;
}

const CATEGORY_COLORS: Record<string, string> = {
  '榜单与评分': '#F53F3F',
  'AI评分': '#722ED1',
  '技术面-趋势': '#165DFF',
  '技术面-超买超卖': '#0FC6C2',
  '技术面-量价': '#FF7D00',
  '基本面': '#00B42A',
  '资金面': '#F7BA1E',
  '预测': '#F5319D',
};

const INDICATOR_SIGNAL_MAP: Record<string, (v: number) => { signal: string; color: string }> = {
  rsi: (v) => v > 70 ? { signal: '超买', color: '#F53F3F' } : v < 30 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  rsi_6: (v) => v > 80 ? { signal: '超买', color: '#F53F3F' } : v < 20 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  rsi_12: (v) => v > 70 ? { signal: '超买', color: '#F53F3F' } : v < 30 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  rsi_24: (v) => v > 70 ? { signal: '超买', color: '#F53F3F' } : v < 30 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  williams_r: (v) => v > -20 ? { signal: '超买', color: '#F53F3F' } : v < -80 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  cci: (v) => v > 100 ? { signal: '超买', color: '#F53F3F' } : v < -100 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  kdj_k: (v) => v > 80 ? { signal: '超买', color: '#F53F3F' } : v < 20 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
  kdj_j: (v) => v > 100 ? { signal: '钝化', color: '#F53F3F' } : v < 0 ? { signal: '钝化', color: '#00B42A' } : { signal: '正常', color: 'var(--color-text-3)' },
  macd_dif: (v) => v > 0 ? { signal: '多头', color: '#F53F3F' } : { signal: '空头', color: '#00B42A' },
  trend_strength: (v) => v > 0.5 ? { signal: '强趋势', color: '#F53F3F' } : v > 0.2 ? { signal: '弱趋势', color: '#FF7D00' } : { signal: '震荡', color: 'var(--color-text-3)' },
  adx: (v) => v > 40 ? { signal: '强趋势', color: '#F53F3F' } : v > 20 ? { signal: '弱趋势', color: '#FF7D00' } : { signal: '无趋势', color: 'var(--color-text-3)' },
  ma_cross: (v) => v > 0 ? { signal: '金叉', color: '#F53F3F' } : v < 0 ? { signal: '死叉', color: '#00B42A' } : { signal: '无信号', color: 'var(--color-text-3)' },
  ema_cross: (v) => v > 0 ? { signal: '金叉', color: '#F53F3F' } : v < 0 ? { signal: '死叉', color: '#00B42A' } : { signal: '无信号', color: 'var(--color-text-3)' },
  volume_ratio: (v) => v > 2 ? { signal: '放量', color: '#F53F3F' } : v < 0.5 ? { signal: '缩量', color: 'var(--color-text-3)' } : { signal: '正常', color: 'var(--color-text-3)' },
  volume_trend: (v) => v > 0 ? { signal: '放量', color: '#F53F3F' } : v < 0 ? { signal: '缩量', color: '#00B42A' } : { signal: '持平', color: 'var(--color-text-3)' },
  psy_12: (v) => v > 75 ? { signal: '超买', color: '#F53F3F' } : v < 25 ? { signal: '超卖', color: '#00B42A' } : { signal: '中性', color: 'var(--color-text-3)' },
};

function getSignal(key: string, value: number): { signal: string; color: string } | null {
  if (INDICATOR_SIGNAL_MAP[key]) return INDICATOR_SIGNAL_MAP[key](value);
  return null;
}

const CATEGORY_ORDER = ['榜单与评分', 'AI评分', '技术面-趋势', '技术面-超买超卖', '技术面-量价', '基本面', '资金面', '预测'];

function formatValue(item: IndicatorItem): string {
  if (item.noData) return '—';
  const v = item.value;
  const u = item.unit;
  if (u === '%') return (v >= 0 ? '+' : '') + v.toFixed(2) + '%';
  if (u === '元') return v.toFixed(2);
  if (u === '亿') return (v / 1e8).toFixed(2) + '亿';
  if (u === '分') return v.toFixed(1);
  if (u === '次数') return v.toFixed(0);
  if (u === '比值' || u === '率') return v.toFixed(4);
  return v >= 10 ? v.toFixed(0) : v.toFixed(2);
}

export default function IndicatorTable({ data, loading, dates = [], selectedDate, onDateChange, currentDate }: Props) {
  const [category, setCategory] = useState<string>('');
  const [search, setSearch] = useState('');

  const categories = useMemo(() => {
    const cats = new Set(data.map(d => d.category));
    return CATEGORY_ORDER.filter(c => cats.has(c));
  }, [data]);

  const filtered = useMemo(() => {
    let list = data;
    if (category) list = list.filter(d => d.category === category);
    if (search) {
      const s = search.toLowerCase();
      list = list.filter(d => d.key.includes(s) || d.label.includes(s) || d.desc.includes(s));
    }
    return list;
  }, [data, category, search]);

  const columns = [
    {
      title: '指标名称',
      dataIndex: 'label',
      width: 130,
      ellipsis: true,
      render: (_: any, item: IndicatorItem) => (
        <Tooltip content={item.desc}>
          <span
            style={{
              
              fontWeight: 400,
              color: 'var(--color-text-1)',
              fontSize: 12,
            }}
            
          >
            {item.label}
          </span>
        </Tooltip>
      ),
    },
    {
      title: '当前值',
      dataIndex: 'value',
      width: 110,
      render: (_: any, item: IndicatorItem) => {
        if (item.noData) return <span style={{ color: 'var(--color-text-4)', fontSize: 11 }}>NO DATA</span>;
        if (item.zero) return <span style={{ color: 'var(--color-text-4)', fontSize: 11 }}>0</span>;
        const v = item.value;
        const color = v > 0 ? '#F53F3F' : v < 0 ? '#00B42A' : 'var(--color-text-2)';
        return (
          <span style={{ fontWeight: 600, color, fontSize: 12, fontFamily: "'SF Mono', monospace" }}>
            {formatValue(item)}
          </span>
        );
      },
    },
    {
      title: '信号',
      dataIndex: 'signal',
      width: 80,
      render: (_: any, item: IndicatorItem) => {
        if (item.noData || item.zero) return null;
        const sig = getSignal(item.key, item.value);
        if (!sig) return <span style={{ fontSize: 10, color: 'var(--color-text-4)' }}>—</span>;
        return (
          <Tag
            color={sig.color === '#F53F3F' ? 'red' : sig.color === '#00B42A' ? 'green' : 'gray'}
            style={{ fontSize: 10, padding: '0 6px', lineHeight: '18px', borderRadius: 4 }}
          >
            {sig.signal}
          </Tag>
        );
      },
    },
    {
      title: '分类',
      dataIndex: 'category',
      width: 110,
      render: (_: any, item: IndicatorItem) => (
        <span style={{ fontSize: 10, color: CATEGORY_COLORS[item.category] || 'var(--color-text-3)', fontWeight: 500 }}>
          {item.category}
        </span>
      ),
    },

  ];

  return (
    <div>
      <div style={{ display: 'flex', gap: 12, marginBottom: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <Select
          size="small"
          placeholder="选择日期"
          style={{ width: 140 }}
          value={selectedDate || ''}
          onChange={(v) => onDateChange?.(v)}
        >
          {dates.map(d => (
            <Option key={d} value={d}>{d}{d === dates[0] ? ' (最新)' : ''}</Option>
          ))}
        </Select>
        <Select
          size="small"
          placeholder="全部分类"
          style={{ width: 140 }}
          value={category}
          onChange={setCategory}
          allowClear
        >
          {categories.map(c => (
            <Option key={c} value={c}>{c}</Option>
          ))}
        </Select>
        <Input
          size="small"
          placeholder="搜索指标..."
          style={{ width: 160 }}
          prefix={<Search size={12} />}
          value={search}
          onChange={setSearch}
          allowClear
        />
        <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>
          {filtered.length} / {data.length} 个指标
          {currentDate ? <span style={{ marginLeft: 8, fontWeight: 500 }}>{currentDate}</span> : null}
        </span>
      </div>
      <Table
        data={filtered}
        columns={columns}
        size="small"
        loading={loading}
        pagination={false}
        scroll={{ y: 400 }}
        rowKey="key"
        style={{ borderRadius: 8 }}
      />
    </div>
  );
}
