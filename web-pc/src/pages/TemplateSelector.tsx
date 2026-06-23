import React from 'react';
import { TrendingUp, BarChart4, GitBranch, Zap, Gem, Shield } from 'lucide-react';

export interface StrategyTemplate {
  label: string;
  desc: string;
  icon: string;
  buyConds: any[];
  sellConds: any[];
  regimes: any;
}

export const STRATEGY_TEMPLATES: Record<string, StrategyTemplate> = {
  momentum: {
    label: '动量追击', desc: '追涨强势股，适合牛市主升浪', icon: '🚀',
    buyConds: [
      { condType: 'buy', logicGroup: 1, indicator: 'momentum_20', operator: 'gt', value: 5, weight: 3, lookbackDays: 5 },
      { condType: 'buy', logicGroup: 1, indicator: 'volume_ratio', operator: 'gt', value: 1.5, weight: 2 },
      { condType: 'buy', logicGroup: 2, indicator: 'rsi', operator: 'gt', value: 55, weight: 1.5 },
    ],
    sellConds: [
      { condType: 'sell', logicGroup: 1, indicator: 'momentum_20', operator: 'lt', value: -3, weight: 2 },
      { condType: 'reduce', logicGroup: 1, indicator: 'rsi', operator: 'gt', value: 80, weight: 1 },
    ],
    regimes: { policyAggressive: { buyPct: 25, addPct: 20, stopProfit: 30, stopLoss: -10, allowAdd: true },
               policyDefensive: { buyPct: 10, addPct: 0, stopProfit: 15, stopLoss: -6, allowAdd: false },
               policyCash: { buyPct: 0, addPct: 0, stopProfit: 8, stopLoss: -3, allowAdd: false } },
  },
  trend: {
    label: '趋势跟随', desc: '均线多头排列+MACD确认，适合趋势明朗市场', icon: '📈',
    buyConds: [
      { condType: 'buy', logicGroup: 1, indicator: 'ma_cross', operator: 'cross_up', value: 5.020, weight: 3 },
      { condType: 'buy', logicGroup: 1, indicator: 'macd', operator: 'cross_up', value: 0, weight: 2 },
      { condType: 'buy', logicGroup: 1, indicator: 'adx', operator: 'gt', value: 25, weight: 1.5 },
    ],
    sellConds: [
      { condType: 'sell', logicGroup: 1, indicator: 'ma_cross', operator: 'cross_down', value: 5.020, weight: 3 },
      { condType: 'sell', logicGroup: 1, indicator: 'macd', operator: 'cross_down', value: 0, weight: 2 },
    ],
    regimes: { policyAggressive: { buyPct: 20, addPct: 15, stopProfit: 25, stopLoss: -8, allowAdd: true },
               policyDefensive: { buyPct: 10, addPct: 0, stopProfit: 12, stopLoss: -5, allowAdd: false },
               policyCash: { buyPct: 0, addPct: 0, stopProfit: 8, stopLoss: -3, allowAdd: false } },
  },
  reversal: {
    label: '均值回归', desc: '超跌反弹+RSI超卖，适合震荡市', icon: '🔄',
    buyConds: [
      { condType: 'buy', logicGroup: 1, indicator: 'ma_deviation', operator: 'lt', value: -8, weight: 3 },
      { condType: 'buy', logicGroup: 1, indicator: 'rsi', operator: 'lt', value: 35, weight: 2 },
      { condType: 'buy', logicGroup: 2, indicator: 'boll_position', operator: 'lt', value: 0.2, weight: 1.5 },
    ],
    sellConds: [
      { condType: 'sell', logicGroup: 1, indicator: 'ma_deviation', operator: 'gt', value: 5, weight: 2 },
      { condType: 'reduce', logicGroup: 1, indicator: 'rsi', operator: 'gt', value: 65, weight: 1 },
    ],
    regimes: { policyAggressive: { buyPct: 15, addPct: 10, stopProfit: 15, stopLoss: -8, allowAdd: false },
               policyDefensive: { buyPct: 10, addPct: 0, stopProfit: 10, stopLoss: -5, allowAdd: false },
               policyCash: { buyPct: 0, addPct: 0, stopProfit: 5, stopLoss: -3, allowAdd: false } },
  },
  breakout: {
    label: '放量突破', desc: '量价齐升+新高突破，适合突破行情', icon: '💥',
    buyConds: [
      { condType: 'buy', logicGroup: 1, indicator: 'volume_ratio', operator: 'gt', value: 2, weight: 3 },
      { condType: 'buy', logicGroup: 1, indicator: 'new_high_20', operator: 'eq', value: 1, weight: 2.5 },
      { condType: 'buy', logicGroup: 1, indicator: 'daily_change', operator: 'gt', value: 3, weight: 1.5 },
    ],
    sellConds: [
      { condType: 'sell', logicGroup: 1, indicator: 'volume_ratio', operator: 'lt', value: 0.5, weight: 2 },
      { condType: 'reduce', logicGroup: 1, indicator: 'daily_change', operator: 'lt', value: -3, weight: 1.5 },
    ],
    regimes: { policyAggressive: { buyPct: 20, addPct: 15, stopProfit: 20, stopLoss: -8, allowAdd: true },
               policyDefensive: { buyPct: 10, addPct: 0, stopProfit: 12, stopLoss: -5, allowAdd: false },
               policyCash: { buyPct: 0, addPct: 0, stopProfit: 8, stopLoss: -3, allowAdd: false } },
  },
  value: {
    label: '价值精选', desc: '低PE+高ROE蓝筹，适合价值投资', icon: '💎',
    buyConds: [
      { condType: 'buy', logicGroup: 1, indicator: 'pe', operator: 'lt', value: 15, weight: 2.5, industryRelative: true },
      { condType: 'buy', logicGroup: 1, indicator: 'pb', operator: 'lt', value: 2, weight: 2 },
      { condType: 'buy', logicGroup: 1, indicator: 'roe', operator: 'gt', value: 10, weight: 2 },
    ],
    sellConds: [
      { condType: 'sell', logicGroup: 1, indicator: 'pe', operator: 'gt', value: 40, weight: 2 },
      { condType: 'reduce', logicGroup: 1, indicator: 'roe', operator: 'lt', value: 5, weight: 1.5 },
    ],
    regimes: { policyAggressive: { buyPct: 15, addPct: 10, stopProfit: 25, stopLoss: -8, allowAdd: true },
               policyDefensive: { buyPct: 10, addPct: 0, stopProfit: 15, stopLoss: -5, allowAdd: false },
               policyCash: { buyPct: 0, addPct: 0, stopProfit: 10, stopLoss: -3, allowAdd: false } },
  },
  bluechip: {
    label: '保守蓝筹', desc: '大市值低波动，适合防御配置', icon: '🏛️',
    buyConds: [
      { condType: 'buy', logicGroup: 1, indicator: 'total_market_cap', operator: 'gt', value: 50000000000, weight: 2 },
      { condType: 'buy', logicGroup: 1, indicator: 'pe', operator: 'lt', value: 20, weight: 2 },
      { condType: 'buy', logicGroup: 1, indicator: 'daily_change', operator: 'lt', value: 3, weight: 1 },
    ],
    sellConds: [
      { condType: 'sell', logicGroup: 1, indicator: 'daily_change', operator: 'lt', value: -5, weight: 2 },
    ],
    regimes: { policyAggressive: { buyPct: 12, addPct: 8, stopProfit: 20, stopLoss: -6, allowAdd: false },
               policyDefensive: { buyPct: 8, addPct: 0, stopProfit: 12, stopLoss: -4, allowAdd: false },
               policyCash: { buyPct: 0, addPct: 0, stopProfit: 8, stopLoss: -2, allowAdd: false } },
  },
};

const TEMPLATE_ICONS: Record<string, React.ReactNode> = {
  momentum: <TrendingUp size={18} color="#165DFF" />,
  trend: <BarChart4 size={18} color="#722ED1" />,
  reversal: <GitBranch size={18} color="#0FC6C2" />,
  breakout: <Zap size={18} color="#F7BA1E" />,
  value: <Gem size={18} color="#F53F3F" />,
  bluechip: <Shield size={18} color="#14C9C9" />,
};

interface Props {
  selected: string | null;
  onSelect: (key: string | null) => void;
}

/** Template selector grid — 3×2 cards for quick strategy creation. */
const TemplateSelector: React.FC<Props> = ({ selected, onSelect }) => (
  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
    <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>选择策略模板</div>
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 10 }}>
      {Object.entries(STRATEGY_TEMPLATES).map(([key, tmpl]) => {
        const isSelected = selected === key;
        return (
          <div
            key={key}
            onClick={() => onSelect(isSelected ? null : key)}
            style={{
              padding: '12px 10px',
              borderRadius: 8,
              border: isSelected ? '2px solid #165DFF' : '1px solid var(--color-border-2)',
              background: isSelected ? 'rgba(22,93,255,0.06)' : 'var(--color-bg-2)',
              cursor: 'pointer',
              transition: 'all 0.15s',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 6,
              textAlign: 'center',
            }}
          >
            <div style={{ width: 32, height: 32, borderRadius: 8, background: 'var(--color-fill-1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              {TEMPLATE_ICONS[key]}
            </div>
            <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-1)' }}>{tmpl.label}</div>
            <div style={{ fontSize: 10, color: 'var(--color-text-3)', lineHeight: 1.4 }}>{tmpl.desc}</div>
          </div>
        );
      })}
    </div>
  </div>
);

export default TemplateSelector;
