import { useState } from 'react';
import { Upload, Button } from '@arco-design/web-react';
import { Database, Upload as UploadIcon, RefreshCw } from 'lucide-react';
import { uploadExcel, triggerCollection, fetchCollectorStatus } from '../services/api';

export default function DataManagementPage() {
  const [status, setStatus] = useState<any>(null);
  const [tab, setTab] = useState<'import' | 'collect'>('import');

  const handleUpload = async (file: File) => {
    try {
      const res: any = await uploadExcel(file);
      setStatus({ type: 'success', msg: `✅ 导入成功！${res.data?.stocksImported || 0} 条, ${res.data?.datesImported || 0} 天` });
    } catch { setStatus({ type: 'error', msg: '❌ 导入失败，请检查文件格式' }); }
    return false;
  };

  const handleTrigger = async () => {
    try { await triggerCollection(); setStatus({ type: 'success', msg: '⏳ 采集任务已触发，请稍后刷新查看' }); }
    catch { setStatus({ type: 'error', msg: '❌ 触发失败' }); }
  };

  return (
    <div>
      <div className="page-header"><h2><Database size={20} style={{marginRight:4}} />数据管理</h2><span className="muted">Excel 导入 · 采集计划 · 数据状态</span></div>

      <div className="card mb16">
        <div className="card-header">
          <div className="seg">
            <button className={tab==='import'?'active':''} onClick={()=>setTab('import')}><UploadIcon size={13} style={{marginRight:4}} />Excel 导入</button>
            <button className={tab==='collect'?'active':''} onClick={()=>setTab('collect')}><RefreshCw size={13} style={{marginRight:4}} />采集管理</button>
          </div>
        </div>
        <div className="card-body">
          {tab === 'import' ? (
            <div>
              <Upload drag accept=".xlsx,.xlsm" autoUpload={false} onChange={(_, file) => handleUpload(file.originFile as File)}
                tip="支持 .xlsx / .xlsm 格式，参考 MSS20260603.xlsm" />
              {status && <div className={`signal-row mt16 ${status.type==='error'?'tag-red':''}`} style={{background:status.type==='error'?'var(--red-1)':undefined}}>{status.msg}</div>}
            </div>
          ) : (
            <div>
              <div className="row gap16 mb16">
                <Button type="primary" icon={<RefreshCw size={14} />} onClick={handleTrigger}>手动触发采集</Button>
                <span className="muted">定时计划：每个交易日 15:30 自动执行</span>
              </div>
              {status && <div className="signal-row">{status.msg}</div>}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
