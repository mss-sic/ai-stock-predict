import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Input, Button } from '@arco-design/web-react';
import { Sparkles, Send, TrendingUp, AlertTriangle, Lightbulb } from 'lucide-react';
import { postAIAnalyze } from '../services/api';

interface Message { role: 'user' | 'ai'; text: string }

const SUGGESTIONS = [
  '分析这只股票的近期走势和风险',
  '当前估值水平如何？是否合理？',
  '机构持仓变化和北向资金动向',
  '帮我写一份建仓计划',
];

export default function AIAnalysisPage() {
  const { code } = useParams<{ code: string }>();
  const [msgs, setMsgs] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);

  const send = async (text?: string) => {
    const msg = text || input;
    if (!msg.trim() || !code) return;
    setMsgs(p => [...p, { role: 'user', text: msg }]);
    if (!text) setInput('');
    setLoading(true);
    try {
      const res: any = await postAIAnalyze(code, msg);
      setMsgs(p => [...p, { role: 'ai', text: res.data.reply }]);
    } catch { setMsgs(p => [...p, { role: 'ai', text: '分析服务暂不可用，请稍后重试。' }]); }
    finally { setLoading(false); }
  };

  return (
    <div style={{ height: 'calc(100vh - 200px)', display: 'flex', flexDirection: 'column' }}>
      <div className="page-header">
        <h2><Sparkles size={20} style={{ marginRight: 4 }} />AI 智能分析</h2>
        <span className="muted">{code} · 深度研报解读 + 信号归因</span>
      </div>

      <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div className="card-body ai-gradient" style={{ flex: 1, overflow: 'auto', borderRadius: '4px 4px 0 0' }}>
          {msgs.length === 0 ? (
            <div style={{ padding: 32 }}>
              <div className="mb16" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div className="avatar" style={{ width: 40, height: 40, background: 'linear-gradient(135deg, var(--arcoblue-6), var(--purple-6))', display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 8, color: '#fff', fontWeight: 600 }}>AI</div>
                <div>
                  <div style={{ fontWeight: 600 }}>智策 AI 助手</div>
                  <div className="muted">基于多维数据提供深度分析</div>
                </div>
              </div>
              <div className="muted mb16">试试以下问题：</div>
              <div className="row gap8" style={{ flexWrap: 'wrap' }}>
                {SUGGESTIONS.map((s, i) => (
                  <button key={i} className="chip" onClick={() => send(s)} style={{ fontSize: 13, padding: '6px 16px' }}>
                    {i === 0 ? <TrendingUp size={12} style={{ marginRight: 3 }} /> : i === 1 ? <Lightbulb size={12} style={{ marginRight: 3 }} /> : i === 2 ? <AlertTriangle size={12} style={{ marginRight: 3 }} /> : <Sparkles size={12} style={{ marginRight: 3 }} />}
                    {s}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            msgs.map((m, i) => (
              <div key={i} className={`chat-msg ${m.role}`}>
                <div className="avatar">{m.role === 'ai' ? 'AI' : 'U'}</div>
                <div className="body" style={{ whiteSpace: 'pre-wrap' }}>{m.text}</div>
              </div>
            ))
          )}
          {loading && <div className="chat-msg ai"><div className="avatar">AI</div><div className="body muted">分析中...</div></div>}
        </div>
        <div className="card-header" style={{ borderTop: '1px solid var(--color-border-1)', borderBottom: 'none' }}>
          <Input value={input} onChange={setInput} onPressEnter={() => send()} placeholder="输入分析问题..." style={{ flex: 1 }} />
          <Button type="primary" icon={<Send size={14} />} onClick={() => send()} loading={loading}>发送</Button>
        </div>
      </div>
    </div>
  );
}
