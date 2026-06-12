import { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Input, InputNumber, Popconfirm } from '@arco-design/web-react';
import { Briefcase, Plus, Edit, Trash2, Search } from 'lucide-react';
import { fetchHoldings, createHolding, updateHolding, deleteHolding } from '../services/api';
import { searchStock } from '../services/api';

interface Holding {
  id: number; stockCode: string; stockName: string;
  costPrice: number; quantity: number; curPrice: number;
  marketVal: number; pnl: number; pnlPct: number;
}

export default function HoldingsPage() {
  const [data, setData] = useState<Holding[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formCode, setFormCode] = useState('');
  const [formCost, setFormCost] = useState<number>(0);
  const [formQty, setFormQty] = useState<number>(0);
  const [formName, setFormName] = useState('');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [searching, setSearching] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { data: r } = await fetchHoldings();
      setData(r.data || []);
    } catch {}
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  // Search stock by code or name
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
    setSearchResults([]);
    setModalOpen(true);
  };

  const openEdit = (h: Holding) => {
    setEditingId(h.id);
    setFormCode(h.stockCode);
    setFormName(h.stockName);
    setFormCost(h.costPrice);
    setFormQty(h.quantity);
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
        await updateHolding(editingId, formCost, formQty);
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '修改成功' } }));
      } else {
        await createHolding(formCode, formCost, formQty);
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '添加成功' } }));
      }
      setModalOpen(false);
      load();
    } catch (err: any) {
      // interceptor handles toast
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteHolding(id);
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '已删除' } }));
      load();
    } catch {}
  };

  const totalValue = data.reduce((s, h) => s + h.marketVal, 0);
  const totalCost = data.reduce((s, h) => s + h.costPrice * h.quantity, 0);
  const totalPnl = data.reduce((s, h) => s + h.pnl, 0);
  const totalPnlPct = totalCost > 0 ? (totalPnl / totalCost) * 100 : 0;
  const upCount = data.filter(h => h.pnl >= 0).length;

  const inp: React.CSSProperties = {
    width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid #e5e6eb',
    background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 14, outline: 'none', boxSizing: 'border-box',
  };

  return (
    <div>
      <div className="page-header">
        <h2><Briefcase size={20} style={{ marginRight: 4 }} />持仓管理</h2>
        <Button type="primary" icon={<Plus size={14} />} onClick={openCreate}>新增持仓</Button>
      </div>

      <div className="stat-grid mb16">
        <div className="stat-card">
          <div className="stat-label">总市值</div>
          <div className="stat-value">{totalValue.toLocaleString(undefined, { maximumFractionDigits: 0 })}<span style={{ fontSize: 14, color: 'var(--color-text-3)' }}> 元</span></div>
        </div>
        <div className="stat-card">
          <div className="stat-label">总盈亏</div>
          <div className={`stat-value ${totalPnl >= 0 ? 'up' : 'down'}`}>
            {totalPnl >= 0 ? '+' : ''}{totalPnl.toLocaleString(undefined, { maximumFractionDigits: 0 })}<span style={{ fontSize: 14 }}> 元</span>
          </div>
          <div className={`stat-sub ${totalPnl >= 0 ? 'up' : 'down'}`}>
            {totalPnl >= 0 ? '+' : ''}{totalPnlPct.toFixed(2)}%
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">持仓数</div>
          <div className="stat-value">{data.length}<span style={{ fontSize: 14, color: 'var(--color-text-3)' }}> 只</span></div>
          <div className="stat-sub">
            <span style={{ color: '#f53f3f' }}>{upCount}盈</span> / <span style={{ color: '#00b42a' }}>{data.length - upCount}亏</span>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">持仓成本</div>
          <div className="stat-value">{totalCost.toLocaleString(undefined, { maximumFractionDigits: 0 })}<span style={{ fontSize: 14, color: 'var(--color-text-3)' }}> 元</span></div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">持仓明细</span>
        </div>
        <Table
          columns={[
            { title: '代码', dataIndex: 'stockCode', width: 96, render: (v: string) => <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{v}</span> },
            { title: '名称', dataIndex: 'stockName', width: 96 },
            { title: '成本', dataIndex: 'costPrice', width: 80, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>{v?.toFixed(3)}</span> },
            { title: '现价', dataIndex: 'curPrice', width: 88, render: (v: number) => <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{v?.toFixed(2)}</span> },
            { title: '持仓', dataIndex: 'quantity', width: 64, render: (v: number) => `${v}股` },
            { title: '市值', dataIndex: 'marketVal', width: 100, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>¥{v?.toLocaleString(undefined, { maximumFractionDigits: 0 })}</span> },
            { title: '盈亏', dataIndex: 'pnl', width: 100, render: (v: number) => <span style={{ color: v >= 0 ? '#f53f3f' : '#00b42a', fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}¥{Math.abs(v).toLocaleString(undefined, { maximumFractionDigits: 0 })}</span> },
            { title: '盈亏%', dataIndex: 'pnlPct', width: 80, render: (v: number) => <span style={{ color: v >= 0 ? '#f53f3f' : '#00b42a', fontFamily: 'monospace', fontWeight: 600 }}>{v >= 0 ? '+' : ''}{v?.toFixed(2)}%</span> },
            {
              title: '操作', width: 100, render: (_: any, r: Holding) => (
                <span style={{ display: 'flex', gap: 8 }}>
                  <Button size="mini" type="text" icon={<Edit size={12} />} onClick={() => openEdit(r)} />
                  <Popconfirm title="确定删除此持仓？" onOk={() => handleDelete(r.id)}>
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
          scroll={{ x: 900 }}
          locale={{ emptyText: '暂无持仓，点击右上角「新增持仓」添加' }}
        />
      </div>

      {/* Add/Edit Modal */}
      <Modal
        visible={modalOpen}
        title={editingId ? '修改持仓' : '新增持仓'}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        okText={editingId ? '保存修改' : '确认添加'}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {!editingId && (
            <div style={{ position: 'relative' }}>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>股票代码/名称</div>
              <Input
                prefix={<Search size={14} color="#86909c" />}
                placeholder="输入代码或名称搜索..."
                value={formCode}
                onChange={handleSearch}
                style={{ ...inp }}
              />
              {searchResults.length > 0 && (
                <div style={{
                  position: 'absolute', top: 62, left: 0, right: 0, zIndex: 100,
                  background: '#fff', border: '1px solid #e5e6eb', borderRadius: 6,
                  boxShadow: '0 4px 12px rgba(0,0,0,0.08)', maxHeight: 200, overflow: 'auto',
                }}>
                  {searchResults.map((s: any) => (
                    <div
                      key={s.code}
                      onClick={() => selectStock(s)}
                      style={{
                        padding: '8px 12px', cursor: 'pointer', fontSize: 13,
                        borderBottom: '1px solid #f2f3f5',
                        display: 'flex', justifyContent: 'space-between',
                      }}
                      onMouseEnter={e => (e.currentTarget.style.background = '#f2f3f5')}
                      onMouseLeave={e => (e.currentTarget.style.background = '')}
                    >
                      <span style={{ fontWeight: 600, fontFamily: 'monospace' }}>{s.code}</span>
                      <span>{s.name}</span>
                    </div>
                  ))}
                </div>
              )}
              {formName && <div style={{ marginTop: 4, fontSize: 12, color: '#165dff' }}>已选: {formName}</div>}
            </div>
          )}
          {editingId && (
            <div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>股票</div>
              <div style={{ fontSize: 14, fontWeight: 600 }}>{formCode} {formName}</div>
            </div>
          )}
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>成本价格 (元)</div>
            <InputNumber
              value={formCost}
              onChange={v => setFormCost(v || 0)}
              min={0.01}
              step={0.01}
              precision={3}
              style={{ width: '100%' }}
              placeholder="买入均价"
            />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>持股数量 (股)</div>
            <InputNumber
              value={formQty}
              onChange={v => setFormQty(v || 0)}
              min={1}
              step={100}
              precision={0}
              style={{ width: '100%' }}
              placeholder="持股数量"
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
