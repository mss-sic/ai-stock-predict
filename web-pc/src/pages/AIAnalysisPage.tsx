import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Input, Button } from '@arco-design/web-react';
import { Sparkles, Send } from 'lucide-react';
import { postAIAnalyze } from '../services/api';

interface Message { role: 'user' | 'ai'; text: string }

export default function AIAnalysisPage() {
  const { code } = useParams<{ code: string }>();
  const [msgs, setMsgs] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);

  const send = async () => {
    if (!input.trim() || !code) return;
    setMsgs(p => [...p, { role: 'user', text: input }]);
    setInput(''); setLoading(true);
    try {
      const res: any = await postAIAnalyze(code, input);
      setMsgs(p => [...p, { role: 'ai', text: res.data.reply }]);
    } catch { setMsgs(p => [...p, { role: 'ai', text: '分析服务暂不可用，请稍后重试。' }]); }
    finally { setLoading(false); }
  };

  return (
    <div style={{ height: 'calc(100vh - 160px)', display: 'flex', flexDirection: 'column' }}>
      <div className="page-header"><h2><Sparkles size={20} style={{marginRight:4}} />AI 智能分析</h2><span className="muted">{code} · 深度研报解读 + 信号归因</span></div>
      <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div className="card-body" style={{ flex: 1, overflow: 'auto' }}>
          {msgs.map((m, i) => (
            <div key={i} className={`chat-msg ${m.role}`}>
              <div className="avatar">{m.role === 'ai' ? 'AI' : 'U'}</div>
              <div className="body">{m.text}</div>
            </div>
          ))}
          {msgs.length === 0 && <div className="muted" style={{textAlign:'center',padding:40}}>输入问题开始 AI 分析，例如：<br/>"分析这只股票的近期走势和风险"</div>}
        </div>
        <div className="card-header" style={{ borderTop: '1px solid var(--color-border-1)', borderBottom: 'none' }}>
          <Input value={input} onChange={setInput} onPressEnter={send} placeholder="输入分析问题..." style={{ flex: 1 }} />
          <Button type="primary" icon={<Send size={14} />} onClick={send} loading={loading}>发送</Button>
        </div>
      </div>
    </div>
  );
}
