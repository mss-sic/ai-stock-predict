import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Table, Button, Modal, Input, InputNumber, Popconfirm, Tag } from '@arco-design/web-react';
import { Briefcase, Plus, Edit, Trash2, Search, Wallet, TrendingUp } from 'lucide-react';
import { fetchHoldingsSummary, fetchHoldings, createHolding, updateHolding, deleteHolding, fetchAccount, updateAccount } from '../services/api';
import { searchStock } from '../services/api';

interface Holding {
  id: number; stockCode: string; stockName: string;
  costPrice: number; quantity: number; totalCost: number;
  buyDate: string; curPrice: number; priceDate: string; prevClose: number;
  dailyChg: number; dailyChgPct: number; dailyPnl: number; dailyPnlPct: number;
  marketVal: number; pnl: number; pnlPct: number; holdDays: number;
}

interface Summary {
  initialCapital: number; availableCash: number;
  totalMarketValue: number; totalCost: number;
  totalEquity: number; totalPnl: number; totalPnlPct: number;
  totalDailyPnl: number; positionCount: number; upCount: number; downCount: number;
}

export default function HoldingsPage() {
  const [data, setData] = useState<Holding[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const [modalOpen, setModalOpen] = useState(false);
  const [fundModalOpen, setFundModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formCode, setFormCode] = useState('');
  const [formCost, setFormCost] = useState<number>(0);
  const [formQty, setFormQty] = useState<number>(0);
  const [formBuyDate, setFormBuyDate] = useState('');
  const [formName, setFormName] = useState('');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [searching, setSearching] = useState(false);
  const [fundAction, setFundAction] = useState<'deposit' | 'withdraw'>('deposit');
  const [fundAmount, setFundAmount] = useState<number>(0);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [{ data: r }, { data: s }] = await Promise.all([
        fetchHoldings(),
        fetchHoldingsSummary(),
      ]);
      setData(r.data || []);
      setSummary(s.data || null);
    } catch {}
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleSearch = async (kw: string) => {
    setFormCode(kw);
    if (kw.length < 2) { setSearchResults([]); return; }
    setSearching(true);
    try {
      const { data: r } = await searchStock(kw);
      setSearchResults((r.data || []).slice(0, 8));
    } catch { setSearchResults([]); }
    setSearching(false);
  };

  const selectStock = (s: any) => {
    setFormCode(s.code);
    setFormName(s.name);
    setSearchResults([]);
  };

  const openCreate = () => {
    setEditingId(null);
    setFormCode('');
    setFormName('');
    setFormCost(0);
    setFormQty(0);
    setFormBuyDate(new Date().toISOString().slice(0, 10));
    setSearchResults([]);
    setModalOpen(true);
  };

  const openEdit = (h: Holding) => {
    setEditingId(h.id);
    setFormCode(h.stockCode);
    setFormName(h.stockName);
    setFormCost(h.costPrice);
    setFormQty(h.quantity);
    setFormBuyDate(h.buyDate || '');
    setSearchResults([]);
    setModalOpen(true);
  };

  const handleSave = async () => {
    if (!formCode || formCost <= 0 || formQty <= 0) {
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'warning', message: '请填写完整信息' } }));
      return;
    }
    try {
      if (editingId) {
        await updateHolding(editingId, formCost, formQty, formBuyDate);
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '修改成功' } }));
      } else {
        await createHolding(formCode, formCost, formQty, formBuyDate);
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '买入成功' } }));
      }
      setModalOpen(false);
      load();
    } catch (err: any) {}
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteHolding(id);
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '已卖出，资金已退回账户' } }));
      load();
    } catch {}
  };

  const handleFundUpdate = async () => {
    if (fundAmount <= 0) return;
    try {
      await updateAccount(fundAction, fundAmount);
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: fundAction === 'deposit' ? '入金成功' : '出金成功' } }));
      setFundModalOpen(false);
      load();
    } catch {}
  };

  const inp: React.CSSProperties = {
    width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--color-border-1)',
    background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 14, outline: 'none', boxSizing: 'border-box',
  };

  return (
    <div>
      <div className="page-header">
        <h2><Briefcase size={20} style={{ marginRight: 4 }} />持仓管理</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <Button icon={<Wallet size={14} />} onClick={() => setFundModalOpen(true)}>资金管理</Button>
          <Button type="primary" icon={<Plus size={14} />} onClick={openCreate}>新增持仓</Button>
        </div>
      </div>

      {/* Account Summary Cards */}
      {summary && (
        <div className="stat-grid mb16">
          <div className="stat-card">
            <div className="stat-label">总资产 · 可用余额</div>
            <div className="stat-value" style={{ color: 'var(--color-primary)' }}>
              ¥{summary.totalEquity.toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </div>
            <div style={{ display: 'flex', gap: 16, marginTop: 2 }}>
              <div className="stat-sub">权益</div>
              <div className="stat-sub" style={{ color: 'var(--color-text-2)' }}>
                可用 ¥{summary.availableCash.toLocaleString(undefined, { maximumFractionDigits: 0 })}
              </div>
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-label">持仓市值</div>
            <div className="stat-value">
              ¥{summary.totalMarketValue.toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </div>
            <div className="stat-sub">{summary.positionCount} 只股票</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">总盈亏</div>
            <div className={`stat-value ${summary.totalPnl >= 0 ? 'up' : 'down'}`}>
              {summary.totalPnl >= 0 ? '+' : ''}¥{Math.abs(summary.totalPnl).toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </div>
            <div className={`stat-sub ${summary.totalPnl >= 0 ? 'up' : 'down'}`}>
              {summary.totalPnl >= 0 ? '+' : ''}{summary.totalPnlPct.toFixed(2)}% · {summary.upCount}盈{summary.downCount}亏
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-label">当日盈亏</div>
            <div className={`stat-value ${(summary.totalDailyPnl || 0) >= 0 ? 'up' : 'down'}`}>
              {(summary.totalDailyPnl || 0) >= 0 ? '+' : ''}¥{Math.abs(summary.totalDailyPnl || 0).toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </div>
            <div className="stat-sub" style={{ color: 'var(--color-text-3)' }}>
              今日持仓变动
            </div>
          </div>
        </div>
      )}

      {/* Holdings Table */}
      <div className="card">
        <Table
          columns={[
            { title: '股票', dataIndex: 'stockName', width: 140, render: (v: string, r: Holding) => (
              <div style={{ cursor: 'pointer' }} onClick={() => navigate(`/stock/${r.stockCode}`)}>
                <div style={{ fontWeight: 600, fontSize: 13, color: 'var(--color-primary)' }}>{v || r.stockCode}</div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.stockCode}</div>
              </div>
            )},
            { title: '成本', dataIndex: 'costPrice', width: 85, sorter: (a: any, b: any) => a.costPrice - b.costPrice, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>¥{v?.toFixed(2)}</span> },
            { title: '现价', dataIndex: 'curPrice', width: 95, sorter: (a: any, b: any) => (a.curPrice||0) - (b.curPrice||0), render: (v: number, r: Holding) => (
              <div>
                <span style={{ fontFamily: 'monospace', fontSize: 12, color: 'var(--color-text-1)' }}>¥{(v || 0).toFixed(2)}</span>
                {r.priceDate && <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{r.priceDate.slice(5)}</div>}
              </div>
            )},
            { title: '持仓', dataIndex: 'quantity', width: 64, sorter: (a: any, b: any) => a.quantity - b.quantity, render: (v: number) => `${v}股` },
            { title: '市值', dataIndex: 'marketVal', width: 100, sorter: (a: any, b: any) => a.marketVal - b.marketVal, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>¥{v?.toLocaleString(undefined, { maximumFractionDigits: 0 })}</span> },
            { title: '持有', dataIndex: 'holdDays', width: 64, sorter: (a: any, b: any) => a.holdDays - b.holdDays, render: (v: number) => v > 0 ? <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{v}天</span> : <span style={{ color: 'var(--color-text-3)' }}>—</span> },
            { title: '当日盈亏', dataIndex: 'dailyPnl', width: 100, sorter: (a: any, b: any) => (a.dailyPnl||0) - (b.dailyPnl||0), render: (v: number) => <span style={{ color: v >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontFamily: 'monospace', fontSize: 12 }}>{v > 0 ? '+' : ''}¥{Math.abs(v || 0).toLocaleString(undefined, { maximumFractionDigits: 0 })}</span> },
            { title: '当日%', dataIndex: 'dailyPnlPct', width: 72, sorter: (a: any, b: any) => (a.dailyPnlPct||0) - (b.dailyPnlPct||0), render: (v: number) => <span style={{ color: (v || 0) >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontFamily: 'monospace', fontWeight: 600, fontSize: 12 }}>{(v || 0) >= 0 ? '+' : ''}{(v || 0).toFixed(2)}%</span> },
            { title: '盈亏', dataIndex: 'pnl', width: 100, sorter: (a: any, b: any) => a.pnl - b.pnl, render: (v: number) => <span style={{ color: v >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}¥{Math.abs(v).toLocaleString(undefined, { maximumFractionDigits: 0 })}</span> },
            { title: '盈亏%', dataIndex: 'pnlPct', width: 80, sorter: (a: any, b: any) => a.pnlPct - b.pnlPct, render: (v: number) => <span style={{ color: v >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontFamily: 'monospace', fontWeight: 600 }}>{v >= 0 ? '+' : ''}{v?.toFixed(2)}%</span> },
            {
              title: '操作', width: 100, render: (_: any, r: Holding) => (
                <span style={{ display: 'flex', gap: 8 }}>
                  <Button size="mini" type="text" icon={<Edit size={12} />} onClick={() => openEdit(r)} />
                  <Popconfirm title="确定卖出此持仓？将以最新价结算" onOk={() => handleDelete(r.id)}>
                    <Button size="mini" type="text" status="danger" icon={<Trash2 size={12} />} />
                  </Popconfirm>
                </span>
              ),
            },
          ]}
          data={data}
          rowKey="id"
          loading={loading}
          pagination={false}
          scroll={{ x: 1000 }}
          locale={{ emptyText: '暂无持仓，点击右上角「新增持仓」添加' }}
        />
      </div>

      {/* Add/Edit Modal */}
      <Modal
        visible={modalOpen}
        title={editingId ? '修改持仓' : '买入股票'}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        okText={editingId ? '保存修改' : '确认买入'}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {!editingId && (
            <div style={{ position: 'relative' }}>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>股票代码/名称</div>
              <Input
                prefix={<Search size={14} style={{ color: 'var(--color-text-3)' }} />}
                placeholder="输入代码或名称搜索..."
                value={formCode}
                onChange={handleSearch}
                style={{ ...inp }}
              />
              {searchResults.length > 0 && (
                <div style={{
                  position: 'absolute', top: 62, left: 0, right: 0, zIndex: 100,
                  background: 'var(--color-bg-1)', border: '1px solid var(--color-border-1)', borderRadius: 6,
                  boxShadow: '0 4px 12px rgba(0,0,0,0.08)', maxHeight: 200, overflow: 'auto',
                }}>
                  {searchResults.map((s: any) => (
                    <div
                      key={s.code}
                      onClick={() => selectStock(s)}
                      style={{
                        padding: '8px 12px', cursor: 'pointer', fontSize: 13,
                        borderBottom: '1px solid var(--color-border-1)',
                        display: 'flex', justifyContent: 'space-between',
                      }}
                      onMouseEnter={e => (e.currentTarget.style.background = 'var(--color-fill-2)')}
                      onMouseLeave={e => (e.currentTarget.style.background = '')}
                    >
                      <span style={{ fontWeight: 600, fontFamily: 'monospace' }}>{s.code}</span>
                      <span>{s.name}</span>
                    </div>
                  ))}
                </div>
              )}
              {formName && <div style={{ marginTop: 4, fontSize: 12, color: 'var(--color-primary)' }}>已选: {formName}</div>}
            </div>
          )}
          {editingId && (
            <div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>股票</div>
              <div style={{ fontSize: 14, fontWeight: 600 }}>{formCode} {formName}</div>
            </div>
          )}
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>买入价格 (元)</div>
            <InputNumber value={formCost} onChange={v => setFormCost(v || 0)} min={0.01} step={0.01} precision={3} style={{ width: '100%' }} placeholder="买入均价" />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>买入数量 (股)</div>
            <InputNumber value={formQty} onChange={v => setFormQty(v || 0)} min={1} step={100} precision={0} style={{ width: '100%' }} placeholder="持股数量" />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>买入日期</div>
            <Input value={formBuyDate} onChange={setFormBuyDate} style={{ ...inp }} placeholder="YYYY-MM-DD" />
          </div>
          {!editingId && (
            <div style={{ padding: '8px 12px', background: 'var(--color-info-bg)', borderRadius: 6, fontSize: 12, color: 'var(--color-primary)' }}>
              预计买入金额：<b>¥{((formCost * formQty) || 0).toLocaleString(undefined, { maximumFractionDigits: 0 })}</b>
              {summary && formCost > 0 && formQty > 0 && formCost * formQty > summary.availableCash && (
                <span style={{ color: 'var(--color-danger-text)', marginLeft: 8 }}>⚠ 余额不足</span>
              )}
            </div>
          )}
        </div>
      </Modal>

      {/* Fund Management Modal */}
      <Modal
        visible={fundModalOpen}
        title="资金管理"
        onCancel={() => setFundModalOpen(false)}
        onOk={handleFundUpdate}
        okText={fundAction === 'deposit' ? '确认入金' : '确认出金'}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {summary && (
            <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
              <div style={{ flex: 1, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 8 }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>总资产</div>
                <div style={{ fontSize: 18, fontWeight: 700 }}>¥{summary.totalEquity.toLocaleString(undefined, { maximumFractionDigits: 0 })}</div>
              </div>
              <div style={{ flex: 1, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 8 }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>可用余额</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-primary)' }}>¥{summary.availableCash.toLocaleString(undefined, { maximumFractionDigits: 0 })}</div>
              </div>
            </div>
          )}
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>操作类型</div>
            <div style={{ display: 'flex', gap: 8 }}>
              <Button type={fundAction === 'deposit' ? 'primary' : 'default'} onClick={() => setFundAction('deposit')}>入金</Button>
              <Button type={fundAction === 'withdraw' ? 'primary' : 'default'} onClick={() => setFundAction('withdraw')}>出金</Button>
            </div>
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>金额 (元)</div>
            <InputNumber value={fundAmount} onChange={v => setFundAmount(v || 0)} min={1} step={1000} precision={2} style={{ width: '100%' }} placeholder="输入金额" />
          </div>
          {fundAction === 'withdraw' && summary && fundAmount > summary.availableCash && (
            <div style={{ padding: '8px 12px', background: 'var(--color-danger-bg)', borderRadius: 6, fontSize: 12, color: 'var(--color-danger-text)' }}>
              ⚠ 可用余额不足，最多可出金 ¥{summary.availableCash.toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
}
