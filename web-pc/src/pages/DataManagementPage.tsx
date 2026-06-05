import { useState, useEffect } from 'react';
import { Upload, Button, Message, Table, Tag } from '@arco-design/web-react';
import { Database, Upload as UploadIcon, RefreshCw, FileSpreadsheet, CheckCircle, XCircle, Clock } from 'lucide-react';
import { uploadExcel, triggerCollection, fetchImportHistory } from '../services/api';

export default function DataManagementPage() {
  const [status, setStatus] = useState<{ type: string; msg: string } | null>(null);
  const [tab, setTab] = useState<'import' | 'collect' | 'history'>('import');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);

  useEffect(() => {
    if (tab === 'history') loadHistory();
  }, [tab]);

  const loadHistory = async () => {
    try {
      const res: any = await fetchImportHistory();
      setHistory(res.data || []);
    } catch { setHistory([]); }
  };

  const handleUpload = async (file: File) => {
    setLoading(true);
    setResult(null);
    setStatus(null);
    try {
      const res: any = await uploadExcel(file);
      setResult(res.data);
      setStatus({ type: 'success', msg: '导入成功' });
      Message.success('Excel 导入完成');
    } catch (err: any) {
      const msg = err?.response?.data?.error || '导入失败，请检查文件格式';
      setStatus({ type: 'error', msg });
      Message.error(msg);
    }
    setLoading(false);
    return false;
  };

  const handleTrigger = async () => {
    setLoading(true);
    try {
      await triggerCollection();
      setStatus({ type: 'success', msg: '采集任务已触发' });
      Message.success('采集任务已触发，请稍后查看');
    } catch {
      setStatus({ type: 'error', msg: '触发失败' });
      Message.error('触发失败');
    }
    setLoading(false);
  };

  const statusTag = (s: string) => {
    if (s === 'success') return <Tag color="green" icon={<CheckCircle size={12} />}>成功</Tag>;
    if (s === 'partial') return <Tag color="orange" icon={<Clock size={12} />}>部分成功</Tag>;
    return <Tag color="red" icon={<XCircle size={12} />}>失败</Tag>;
  };

  return (
    <div>
      <div className="page-header">
        <h2><Database size={20} style={{ marginRight: 8 }} />数据管理</h2>
        <span className="muted">Excel 导入 · 采集计划 · 导入历史</span>
      </div>

      {/* Tab bar */}
      <div style={{
        display: 'flex', gap: 0, marginBottom: 16,
        background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
        overflow: 'hidden',
      }}>
        {[
          { key: 'import', label: 'Excel 导入', icon: <UploadIcon size={14} /> },
          { key: 'collect', label: '采集管理', icon: <RefreshCw size={14} /> },
          { key: 'history', label: '导入历史', icon: <Clock size={14} /> },
        ].map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key as any)}
            style={{
              padding: '10px 20px', border: 'none', cursor: 'pointer', fontSize: 13,
              background: tab === t.key ? '#e8f3ff' : 'transparent',
              color: tab === t.key ? '#165dff' : '#4e5969',
              fontWeight: tab === t.key ? 500 : 400,
              display: 'flex', alignItems: 'center', gap: 6,
              borderRight: '1px solid #e5e6eb',
              transition: 'all 100ms',
            }}
          >
            {t.icon}{t.label}
          </button>
        ))}
      </div>

      {/* Content */}
      {tab === 'import' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Upload card */}
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <FileSpreadsheet size={16} color="#165dff" />
                <span style={{ fontSize: 15, fontWeight: 600 }}>上传 Excel 文件</span>
              </span>
              <span className="muted" style={{ fontSize: 12 }}>支持 .xlsx / .xlsm</span>
            </div>
            <div className="card-body">
              <Upload
                drag
                accept=".xlsx,.xlsm"
                autoUpload={false}
                disabled={loading}
                onChange={(_, file) => handleUpload(file.originFile as File)}
                tip="拖拽或点击上传，参考文件: MSS20260603.xlsm"
              />
              {loading && (
                <div style={{
                  marginTop: 16, padding: '12px 16px',
                  background: '#e8f3ff', borderRadius: 4,
                  display: 'flex', alignItems: 'center', gap: 10,
                  fontSize: 13, color: '#165dff',
                }}>
                  <RefreshCw size={14} className="spin" />
                  正在解析并导入数据...
                </div>
              )}
              {status && !loading && (
                <div style={{
                  marginTop: 16, padding: '12px 16px', borderRadius: 4,
                  background: status.type === 'error' ? '#ffece8' : '#e8ffea',
                  color: status.type === 'error' ? '#cb272d' : '#009a29',
                  fontSize: 13,
                }}>
                  {status.type === 'error' ? <XCircle size={14} style={{ marginRight: 6 }} /> : <CheckCircle size={14} style={{ marginRight: 6 }} />}
                  {status.msg}
                </div>
              )}
            </div>
          </div>

          {/* Result card */}
          {result && (
            <div className="card">
              <div className="card-header">
                <span style={{ fontSize: 15, fontWeight: 600 }}>导入结果</span>
                <span className="muted" style={{ fontSize: 12 }}>{result.fileName}</span>
              </div>
              <div className="card-body">
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 16 }}>
                  {[
                    { label: '交易日数', value: result.datesImported, color: '#165dff' },
                    { label: '上榜记录', value: result.picksImported, color: '#f53f3f' },
                    { label: '信号数据', value: result.signalsImported, color: '#00b42a' },
                    { label: '新增个股', value: result.stocksCreated, color: '#ff7d00' },
                  ].map(item => (
                    <div key={item.label} style={{
                      textAlign: 'center', padding: '12px',
                      background: '#f7f8fa', borderRadius: 6,
                    }}>
                      <div style={{ fontSize: 24, fontWeight: 700, color: item.color, fontFamily: 'var(--font-family-mono, monospace)' }}>
                        {item.value}
                      </div>
                      <div style={{ fontSize: 12, color: '#86909c', marginTop: 4 }}>{item.label}</div>
                    </div>
                  ))}
                </div>
                {result.previews?.length > 0 && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    {result.previews.map((p: string, i: number) => (
                      <div key={i} style={{
                        padding: '8px 12px', background: '#f7f8fa', borderRadius: 4,
                        fontSize: 13, color: '#4e5969',
                        display: 'flex', alignItems: 'center', gap: 6,
                      }}>
                        <CheckCircle size={12} color="#00b42a" />
                        {p}
                      </div>
                    ))}
                  </div>
                )}
                {result.errors?.length > 0 && (
                  <div style={{ marginTop: 12 }}>
                    {result.errors.map((e: string, i: number) => (
                      <div key={i} style={{
                        padding: '8px 12px', background: '#ffece8', borderRadius: 4,
                        fontSize: 12, color: '#cb272d', marginBottom: 4,
                      }}>
                        <XCircle size={12} style={{ marginRight: 6 }} />{e}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {tab === 'collect' && (
        <div className="card">
          <div className="card-header">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <RefreshCw size={16} color="#165dff" />
              <span style={{ fontSize: 15, fontWeight: 600 }}>采集管理</span>
            </span>
          </div>
          <div className="card-body">
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
              <Button type="primary" loading={loading} icon={<RefreshCw size={14} />} onClick={handleTrigger}>
                手动触发采集
              </Button>
              <span className="muted" style={{ fontSize: 13 }}>
                定时计划：每个交易日 15:30 自动执行
              </span>
            </div>
            {status && (
              <div style={{
                padding: '10px 16px', borderRadius: 4,
                background: status.type === 'error' ? '#ffece8' : '#e8ffea',
                color: status.type === 'error' ? '#cb272d' : '#009a29',
                fontSize: 13,
              }}>
                {status.msg}
              </div>
            )}
          </div>
        </div>
      )}

      {tab === 'history' && (
        <div className="card">
          <div className="card-header">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Clock size={16} color="#165dff" />
              <span style={{ fontSize: 15, fontWeight: 600 }}>导入历史</span>
            </span>
            <Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadHistory}>刷新</Button>
          </div>
          <div className="card-body" style={{ padding: 0 }}>
            {history.length === 0 ? (
              <div style={{ padding: 40, textAlign: 'center', color: '#86909c', fontSize: 13 }}>
                暂无导入记录
              </div>
            ) : (
              <Table
                data={history}
                rowKey="id"
                size="small"
                columns={[
                  { title: '文件名', dataIndex: 'fileName', width: 200 },
                  { title: '导入条数', dataIndex: 'rowsImported', width: 100, render: (v: number) => <span style={{ fontWeight: 600 }}>{v}</span> },
                  {
                    title: '状态', dataIndex: 'status', width: 100,
                    render: (v: string) => statusTag(v),
                  },
                  {
                    title: '时间', dataIndex: 'importedAt', width: 180,
                    render: (v: string) => <span style={{ color: '#86909c', fontSize: 12 }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>,
                  },
                ]}
                pagination={false}
                border={false}
                stripe
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
