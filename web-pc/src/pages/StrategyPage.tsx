import { useState } from 'react';
import { InputNumber, Button, Table } from '@arco-design/web-react';
import { Target, BarChart4, TrendingUp, Shield } from 'lucide-react';

export default function StrategyPage() {
  const [tab, setTab] = useState<'plan' | 'backtest'>('plan');
  const [params, setParams] = useState({ stopProfit: 15, stopLoss: -8, maxHold: 20, position: 15 });

  const expectedReturn = ((params.stopProfit * 0.55 + params.stopLoss * 0.45)).toFixed(1);
  const kelly = Math.max(0, (0.55 - (1 - 0.55) / (params.stopProfit / Math.abs(params.stopLoss))) * 100).toFixed(0);

  return (
    <div>
      <div className="page-header">
        <h2><Target size={20} style={{ marginRight: 4 }} />交易策略中心</h2>
        <div className="seg">
          <button className={tab === 'plan' ? 'active' : ''} onClick={() => setTab('plan')}><Target size={13} style={{ marginRight: 4 }} />交易计划</button>
          <button className={tab === 'backtest' ? 'active' : ''} onClick={() => setTab('backtest')}><BarChart4 size={13} style={{ marginRight: 4 }} />策略回测</button>
        </div>
      </div>

      {tab === 'plan' ? (
        <>
          <div className="card mb16">
            <div className="card-header"><span className="card-title">策略参数</span></div>
            <div className="card-body">
              <div className="row gap16" style={{ flexWrap: 'wrap' }}>
                <div className="col gap8" style={{ flex: 1, minWidth: 140 }}>
                  <label className="muted">止盈 (%)</label>
                  <InputNumber value={params.stopProfit} onChange={(v) => setParams({ ...params, stopProfit: v as number })} style={{ width: '100%' }} />
                </div>
                <div className="col gap8" style={{ flex: 1, minWidth: 140 }}>
                  <label className="muted">止损 (%)</label>
                  <InputNumber value={params.stopLoss} onChange={(v) => setParams({ ...params, stopLoss: v as number })} style={{ width: '100%' }} />
                </div>
                <div className="col gap8" style={{ flex: 1, minWidth: 140 }}>
                  <label className="muted">最大持有 (天)</label>
                  <InputNumber value={params.maxHold} onChange={(v) => setParams({ ...params, maxHold: v as number })} style={{ width: '100%' }} />
                </div>
                <div className="col gap8" style={{ flex: 1, minWidth: 140 }}>
                  <label className="muted">单笔仓位 (%)</label>
                  <InputNumber value={params.position} onChange={(v) => setParams({ ...params, position: v as number })} style={{ width: '100%' }} />
                </div>
              </div>
              <div style={{ marginTop: 16 }}>
                <Button type="primary" icon={<TrendingUp size={14} />}>计算预期收益</Button>
              </div>
            </div>
          </div>

          <div className="stat-grid">
            <div className="stat-card">
              <div className="stat-label">预期收益率</div>
              <div className="stat-value">{expectedReturn}<span style={{ fontSize: 14, color: 'var(--color-text-2)' }}>%</span></div>
              <div className="stat-sub muted">盈亏比 1.88 : 1</div>
            </div>
            <div className="stat-card">
              <div className="stat-label">凯利公式仓位</div>
              <div className="stat-value">{kelly}<span style={{ fontSize: 14, color: 'var(--color-text-2)' }}>%</span></div>
              <div className="stat-sub muted">最优理论仓位</div>
            </div>
            <div className="stat-card">
              <div className="stat-label">胜率预估</div>
              <div className="stat-value">55<span style={{ fontSize: 14, color: 'var(--color-text-2)' }}>%</span></div>
              <div className="stat-sub muted">基于历史榜单数据</div>
            </div>
            <div className="stat-card">
              <div className="stat-label">策略评价</div>
              <div className="stat-value" style={{ fontSize: 16, color: Number(kelly) > 10 ? 'var(--green-6)' : 'var(--orange-6)' }}>
                {Number(kelly) > 10 ? '正期望 ✅' : '需调整 ⚠️'}
              </div>
            </div>
          </div>
        </>
      ) : (
        <div>
          <div className="card mb16">
            <div className="card-header"><span className="card-title">回测设置</span></div>
            <div className="card-body">
              <div className="row gap16" style={{ marginBottom: 16 }}>
                <InputNumber placeholder="起始资金" defaultValue={100000} style={{ width: 160 }} />
                <InputNumber placeholder="滑点" defaultValue={0.1} style={{ width: 120 }} />
                <Button type="primary" icon={<BarChart4 size={14} />}>运行回测</Button>
              </div>
            </div>
          </div>

          <div className="backtest-metrics mb16">
            <div className="metric">
              <div className="val up">+23.5%</div>
              <div className="lbl">累计收益</div>
            </div>
            <div className="metric">
              <div className="val">2.15</div>
              <div className="lbl">夏普比率</div>
            </div>
            <div className="metric">
              <div className="val down">-8.2%</div>
              <div className="lbl">最大回撤</div>
            </div>
            <div className="metric">
              <div className="val">62%</div>
              <div className="lbl">胜率</div>
            </div>
          </div>

          <div className="card">
            <div className="card-header"><span className="card-title">交易明细</span></div>
            <Table
              columns={[
                { title: '日期', dataIndex: 'date' },
                { title: '方向', dataIndex: 'dir' },
                { title: '价格', dataIndex: 'price' },
                { title: '收益%', dataIndex: 'ret' },
                { title: '原因', dataIndex: 'reason' },
              ]}
              data={[
                { id: 1, date: '2026-05-20', dir: '买入', price: '¥1850', ret: '—', reason: '上榜信号' },
                { id: 2, date: '2026-05-28', dir: '卖出', price: '¥1920', ret: '+3.78%', reason: '触发止盈' },
                { id: 3, date: '2026-06-01', dir: '买入', price: '¥152', ret: '—', reason: '连榜3日' },
                { id: 4, date: '2026-06-04', dir: '卖出', price: '¥148', ret: '-2.63%', reason: '触发止损' },
              ]}
              rowKey="id"
              pagination={false}
            />
          </div>
        </div>
      )}
    </div>
  );
}
