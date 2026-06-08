import { useEffect, useState, useCallback } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Button, Input, Modal, Message, Popconfirm } from '@arco-design/web-react';
import { Star, Plus, Trash2, TrendingUp, TrendingDown } from 'lucide-react';
import {
  fetchWatchlist, fetchWatchlistGroups, createWatchlistGroup,
  renameWatchlistGroup, deleteWatchlistGroup, removeFromWatchlist,
  clearWatchlist,
} from '../services/api';

interface Group { id: number; name: string; sortOrder: number; }
interface Stock { stockCode: string; stockName: string; close: number; addedPrice: number; addedAt: string; yield: number; groupId: number; }

export default function WatchlistPage() {
  const navigate = useNavigate();
  const [groups, setGroups] = useState<Group[]>([]);
  const [stocks, setStocks] = useState<Stock[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>('all');
  const [loading, setLoading] = useState(false);

  const [showAddGroup, setShowAddGroup] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [renaming, setRenaming] = useState<{ id: number; name: string } | null>(null);

  const loadGroups = useCallback(async () => {
    try {
      const { data } = await fetchWatchlistGroups();
      setGroups(data.data || []);
    } catch {}
  }, []);

  const loadStocks = useCallback(async () => {
    setLoading(true);
    try {
      const gid = activeGroup === 'all' ? undefined : parseInt(activeGroup);
      const { data } = await fetchWatchlist(gid);
      setStocks(data.data || []);
    } catch {}
    setLoading(false);
  }, [activeGroup]);

  useEffect(() => { loadGroups(); }, [loadGroups]);
  useEffect(() => { loadStocks(); }, [loadStocks]);

  const handleAddGroup = async () => {
    if (!newGroupName.trim()) { Message.warning('名称不能为空'); return; }
    try {
      await createWatchlistGroup(newGroupName.trim());
      setNewGroupName('');
      setShowAddGroup(false);
      loadGroups();
      Message.success('分组已创建');
    } catch (err: any) {
      Message.error(err.response?.data?.message || err.message || '创建失败');
    }
  };

  const handleRename = async () => {
    if (!renaming || !renaming.name.trim()) return;
    try {
      await renameWatchlistGroup(renaming.id, renaming.name.trim());
      setRenaming(null);
      loadGroups();
    } catch (err: any) {
      Message.error(err.message || '重命名失败');
    }
  };

  const handleDeleteGroup = async (id: number) => {
    try {
      await deleteWatchlistGroup(id);
      if (activeGroup === String(id)) setActiveGroup('all');
      loadGroups();
      loadStocks();
      Message.success('分组已删除');
    } catch {}
  };

  const handleRemoveStock = async (code: string) => {
    try { await removeFromWatchlist(code); loadStocks(); Message.success('已移除'); } catch {}
  };

  const handleClear = async () => {
    const gid = activeGroup === 'all' ? undefined : parseInt(activeGroup);
    try { await clearWatchlist(gid); loadStocks(); Message.success('已清空'); } catch {}
  };

  const fmtPct = (v: number) => {
    if (!v || v === 0) return <span style={{ color: '#86909c' }}>0.00%</span>;
    const positive = v > 0;
    return (
      <span style={{ color: positive ? '#f53f3f' : '#00b42a', display: 'flex', alignItems: 'center', gap: 2 }}>
        {positive ? <TrendingUp size={12} /> : <TrendingDown size={12} />}
        {positive ? '+' : ''}{v.toFixed(2)}%
      </span>
    );
  };

  const th: React.CSSProperties = { padding: '10px 14px', textAlign: 'left', color: '#86909c', fontWeight: 600, fontSize: 11 };
  const td: React.CSSProperties = { padding: '10px 14px', color: '#4e5969' };

  return (
    <div style={{ padding: '0 0 40px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ color: '#1d2129', fontSize: 18, fontWeight: 700, margin: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Star size={18} color="#f7ba1e" /> 自选股
        </h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <Button size="small" icon={<Plus size={14} />} onClick={() => setShowAddGroup(true)}>新建分组</Button>
          <Popconfirm title="确认清空当前分组的所有自选股？" onOk={handleClear} okText="确认" cancelText="取消">
            <Button size="small" type="outline" status="danger" icon={<Trash2 size={14} />}>清空</Button>
          </Popconfirm>
        </div>
      </div>

      {/* Group Tabs — custom to avoid arco Tabs React 19 ref warning */}
      <div style={{ marginBottom: 16, display: 'flex', gap: 4, flexWrap: 'wrap', alignItems: 'center' }}>
        <button
          onClick={() => setActiveGroup('all')}
          style={{
            padding: '6px 14px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 13,
            background: activeGroup === 'all' ? '#165dff' : '#f2f3f5',
            color: activeGroup === 'all' ? '#fff' : '#4e5969',
            fontWeight: activeGroup === 'all' ? 600 : 400,
            transition: 'all 0.15s',
          }}
        >全部</button>
        {groups.map(g => (
          <div key={g.id} style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
            <button
              onClick={() => setActiveGroup(String(g.id))}
              style={{
                padding: '6px 14px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 13,
                background: activeGroup === String(g.id) ? '#165dff' : '#f2f3f5',
                color: activeGroup === String(g.id) ? '#fff' : '#4e5969',
                fontWeight: activeGroup === String(g.id) ? 600 : 400,
                transition: 'all 0.15s',
              }}
            >
              {renaming?.id === g.id ? (
                <input
                  value={renaming.name}
                  onChange={e => setRenaming({ ...renaming, name: e.target.value })}
                  onBlur={handleRename}
                  onKeyDown={e => { if (e.key === 'Enter') handleRename(); if (e.key === 'Escape') setRenaming(null); }}
                  autoFocus
                  style={{ width: 70, padding: '0 4px', border: '1px solid rgba(255,255,255,0.5)', borderRadius: 3, fontSize: 12, outline: 'none', background: 'rgba(255,255,255,0.15)', color: '#fff' }}
                  onClick={e => e.stopPropagation()}
                />
              ) : (
                <span onDoubleClick={() => setRenaming({ id: g.id, name: g.name })}>{g.name}</span>
              )}
            </button>
            <Popconfirm
              title={`确认删除分组「${g.name}」？股票将移回默认分组`}
              onOk={() => handleDeleteGroup(g.id)}
              okText="确认删除"
              cancelText="取消"
            >
              <span
                style={{ cursor: 'pointer', color: '#f53f3f', fontSize: 10, marginLeft: -4, zIndex: 1 }}
                onClick={e => e.stopPropagation()}
                title="删除分组"
              ><Trash2 size={11} /></span>
            </Popconfirm>
          </div>
        ))}
      </div>

      {/* Stock table */}
      <div style={{ borderRadius: 10, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ background: '#f7f8fa' }}>
              <th style={th}>股票</th>
              <th style={th}>现价</th>
              <th style={th}>自选价</th>
              <th style={th}>自选收益率</th>
              <th style={th}>自选时间</th>
              <th style={{ ...th, textAlign: 'center', width: 60 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {stocks.length === 0 && !loading && (
              <tr><td colSpan={6} style={{ ...td, textAlign: 'center', color: '#c9cdd4', padding: 40 }}>
                暂无自选股，前往 <Link to="/stocks" style={{ color: 'var(--arcoblue-6)' }}>股票列表</Link> 添加
              </td></tr>
            )}
            {stocks.map(s => (
              <tr key={s.stockCode} style={{ borderBottom: '1px solid #f2f3f5', cursor: 'pointer' }}
                onClick={() => navigate(`/stock/${s.stockCode}`)}>
                <td style={{ ...td, fontWeight: 600, color: '#1d2129' }}>
                  {s.stockName}
                  <span style={{ fontSize: 11, color: '#86909c', marginLeft: 6, fontFamily: 'monospace' }}>{s.stockCode}</span>
                </td>
                <td style={{ ...td, fontFamily: 'monospace' }}>{s.close > 0 ? s.close.toFixed(2) : '-'}</td>
                <td style={{ ...td, fontFamily: 'monospace' }}>{s.addedPrice > 0 ? s.addedPrice.toFixed(2) : '-'}</td>
                <td style={td}>{fmtPct(s.yield)}</td>
                <td style={{ ...td, color: '#86909c' }}>{s.addedAt}</td>
                <td style={{ ...td, textAlign: 'center' }}>
                  <Popconfirm
                    title={`确认将「${s.stockName}」移出自选？`}
                    onOk={() => handleRemoveStock(s.stockCode)}
                    okText="确认"
                    cancelText="取消"
                  >
                    <button onClick={e => e.stopPropagation()}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#f53f3f', padding: 4 }}>
                      <Trash2 size={14} />
                    </button>
                  </Popconfirm>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Add group modal */}
      <Modal
        visible={showAddGroup}
        title="新建分组"
        onCancel={() => setShowAddGroup(false)}
        onOk={handleAddGroup}
        okText="创建"
      >
        <Input
          placeholder="分组名称"
          value={newGroupName}
          onChange={setNewGroupName}
          maxLength={20}
          onKeyDown={e => { if (e.key === 'Enter') handleAddGroup(); }}
        />
        <div style={{ color: '#86909c', fontSize: 11, marginTop: 6 }}>最多 20 个分组（当前 {groups.length} 个）</div>
      </Modal>
    </div>
  );
}
