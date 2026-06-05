import { useState } from 'react';
import { Card, Tabs, Upload, Button, Message } from '@arco-design/web-react';
import { uploadExcel, triggerCollection, fetchCollectorStatus } from '../services/api';

export default function DataManagementPage() {
  const [status, setStatus] = useState<any>(null);

  const handleUpload = async (file: File) => {
    try {
      const res: any = await uploadExcel(file);
      Message.success(`导入成功！${res.data?.stocksImported || 0} 条数据`);
    } catch {
      Message.error('导入失败');
    }
    return false;
  };

  const handleTrigger = async () => {
    try {
      await triggerCollection();
      Message.success('采集任务已触发');
      const res: any = await fetchCollectorStatus();
      setStatus(res.data);
    } catch {
      Message.error('触发失败');
    }
  };

  return (
    <Tabs defaultActiveTab="1">
      <Tabs.TabPane key="1" title="Excel 导入">
        <Card>
          <Upload drag accept=".xlsx,.xlsm" autoUpload={false} onChange={(_, file) => handleUpload(file.originFile as File)} />
        </Card>
      </Tabs.TabPane>
      <Tabs.TabPane key="2" title="采集管理">
        <Card>
          <Button type="primary" onClick={handleTrigger}>手动触发采集</Button>
          {status && <pre style={{ marginTop: 16, background: 'var(--color-fill-2)', padding: 12, borderRadius: 4 }}>{JSON.stringify(status, null, 2)}</pre>}
        </Card>
      </Tabs.TabPane>
    </Tabs>
  );
}
