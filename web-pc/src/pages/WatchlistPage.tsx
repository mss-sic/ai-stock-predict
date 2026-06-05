import { useState } from 'react';
import { Table, Input, Button } from '@arco-design/web-react';
import { Star, Trash2, Search } from 'lucide-react';

interface Item { id: number; stockCode: string }
export default function WatchlistPage() {
  const [list, setList] = useState<Item[]>([]);
  const [keyword, setKeyword] = useState('');
  const remove = (id: number) => setList(p => p.filter(i => i.id !== id));

  return (
    <div>
      <div className="page-header"><h2><Star size={20} style={{marginRight:4}} />自选股</h2><span className="muted">跟踪你关注的股票</span></div>
      <div className="card">
        <div className="card-header">
          <div className="row gap8">
            <Input prefix={<Search size={14} />} value={keyword} onChange={setKeyword} placeholder="输入代码搜索..." style={{width:240}} />
            <Button type="primary" size="small">添加</Button>
          </div>
          <span className="muted">共 {list.length} 只</span>
        </div>
        <Table columns={[
          { title: '代码', dataIndex: 'stockCode', render: (v: string) => <span className="num" style={{fontWeight:600}}>{v}</span> },
          { title: '操作', width: 80, render: (_: any, r: Item) => <Button type="text" size="mini" icon={<Trash2 size={13} />} onClick={() => remove(r.id)} style={{color:'var(--color-text-3)'}} /> },
        ]} data={list} rowKey="id" pagination={false} />
      </div>
    </div>
  );
}
