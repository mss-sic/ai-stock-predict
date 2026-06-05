import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Input, Button } from '@arco-design/web-react';
import { postAIAnalyze } from '../services/api';

interface Message { role: 'user' | 'ai'; text: string; }

export default function AIAnalysisPage() {
  const { code } = useParams<{ code: string }>();
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);

  const send = async () => {
    if (!input.trim() || !code) return;
    const userMsg: Message = { role: 'user', text: input };
    setMessages((prev) => [...prev, userMsg]);
    setInput('');
    setLoading(true);
    try {
      const res: any = await postAIAnalyze(code, userMsg.text);
      setMessages((prev) => [...prev, { role: 'ai', text: res.data.reply }]);
    } catch {
      setMessages((prev) => [...prev, { role: 'ai', text: '分析服务暂时不可用，请稍后再试。' }]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card title={`🤖 AI 智能分析 - ${code}`} style={{ height: 'calc(100vh - 120px)', display: 'flex', flexDirection: 'column' }}>
      <div style={{ flex: 1, overflow: 'auto', marginBottom: 16 }}>
        {messages.map((m, i) => (
          <div key={i} style={{ marginBottom: 12, textAlign: m.role === 'user' ? 'right' : 'left' }}>
            <div style={{ display: 'inline-block', padding: '8px 16px', borderRadius: 8,
              background: m.role === 'user' ? 'var(--color-primary-light-1)' : 'var(--color-fill-2)',
              maxWidth: '80%' }}>
              {m.text}
            </div>
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 8 }}>
        <Input value={input} onChange={setInput} onPressEnter={send} placeholder="输入分析问题..." />
        <Button type="primary" onClick={send} loading={loading}>发送</Button>
      </div>
    </Card>
  );
}
