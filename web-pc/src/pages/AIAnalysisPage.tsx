import { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { Input, Button, Tooltip } from '@arco-design/web-react';
import { Sparkles, Send, TrendingUp, AlertTriangle, Lightbulb, Trash2, Loader2 } from 'lucide-react';
import { authFetch } from '../services/api';

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
  const [clearing, setClearing] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  // Load history on mount
  useEffect(() => {
    if (!code) return;
    (async () => {
      try {
        const res = await authFetch(`/api/v1/ai/history/${code}`);
        const json = await res.json();
        const history: Message[] = (json.data || []).map((m: any) => ({
          role: m.role,
          text: m.content,
        }));
        if (history.length > 0) {
          setMsgs(history);
        }
      } catch (_) {}
    })();
  }, [code]);

  // Scroll to bottom on new messages
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [msgs, loading]);

  const send = async (text?: string) => {
    const msg = text || input;
    if (!msg.trim() || !code) return;
    // Append user message immediately
    setMsgs(p => [...p, { role: 'user', text: msg }]);
    if (!text) setInput('');
    setLoading(true);

    // Add placeholder for AI response
    const aiIdx = msgs.length + 1;
    setMsgs(p => [...p, { role: 'ai', text: '' }]);

    try {
      const res = await authFetch('/api/v1/ai/analyze/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code, question: msg }),
      });

      const reader = res.body?.getReader();
      if (!reader) throw new Error('no reader');

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          const data = line.slice(6);
          if (data === '[DONE]') continue;
          try {
            const parsed = JSON.parse(data);
            if (parsed.chunk) {
              setMsgs(prev => {
                const copy = [...prev];
                copy[copy.length - 1] = { ...copy[copy.length - 1], text: copy[copy.length - 1].text + parsed.chunk };
                return copy;
              });
            }
            if (parsed.error) {
              setMsgs(prev => {
                const copy = [...prev];
                copy[copy.length - 1] = { ...copy[copy.length - 1], text: '错误: ' + parsed.error };
                return copy;
              });
            }
          } catch (_) {}
        }
      }

      // If no content received, replace with fallback
      setMsgs(prev => {
        const copy = [...prev];
        if (copy[copy.length - 1].text === '') {
          copy[copy.length - 1] = { ...copy[copy.length - 1], text: '(回复为空)' };
        }
        return copy;
      });
    } catch {
      setMsgs(prev => {
        const copy = [...prev];
        copy[copy.length - 1] = { ...copy[copy.length - 1], text: '分析服务暂不可用，请检查网络或AI配置。' };
        return copy;
      });
    }
    setLoading(false);
  };

  const handleClear = async () => {
    if (!code) return;
    setClearing(true);
    try {
      await authFetch(`/api/v1/ai/history/${code}`, { method: 'DELETE' });
      setMsgs([]);
    } catch (_) {}
    setClearing(false);
  };

  return (
    <div style={{ height: 'calc(100vh - 200px)', display: 'flex', flexDirection: 'column' }}>
      <div className="page-header">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
          <div>
            <h2 style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Sparkles size={20} />
              <span style={{ fontSize: 16, fontWeight: 700 }}>AI 智能分析</span>
            </h2>
            <span className="muted">{code} · 深度研报解读 + 信号归因</span>
          </div>
          {msgs.length > 0 && (
            <Tooltip content="清除对话历史">
              <Button
                size="small"
                type="text"
                icon={<Trash2 size={14} />}
                onClick={handleClear}
                loading={clearing}
                style={{ color: '#86909c' }}
              />
            </Tooltip>
          )}
        </div>
      </div>

      <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div className="card-body ai-gradient" style={{ flex: 1, overflow: 'auto', borderRadius: '4px 4px 0 0' }}>
          {msgs.length === 0 ? (
            <div style={{ padding: 32 }}>
              <div className="mb16" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div className="avatar" style={{
                  width: 40, height: 40, background: 'linear-gradient(135deg, var(--arcoblue-6), var(--purple-6))',
                  display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 8,
                  color: '#fff', fontWeight: 600,
                }}>AI</div>
                <div>
                  <div style={{ fontWeight: 600 }}>智策 AI 助手</div>
                  <div className="muted">基于多维数据提供深度分析</div>
                </div>
              </div>
              <div className="muted mb16">试试以下问题：</div>
              <div className="row gap8" style={{ flexWrap: 'wrap' }}>
                {SUGGESTIONS.map((s, i) => (
                  <button key={i} className="chip" onClick={() => send(s)}
                    style={{ fontSize: 13, padding: '6px 16px', cursor: 'pointer', border: '1px solid #e5e6eb', borderRadius: 6, background: '#fff' }}>
                    {i === 0 ? <TrendingUp size={12} style={{ marginRight: 3 }} /> :
                     i === 1 ? <Lightbulb size={12} style={{ marginRight: 3 }} /> :
                     i === 2 ? <AlertTriangle size={12} style={{ marginRight: 3 }} /> :
                     <Sparkles size={12} style={{ marginRight: 3 }} />}
                    {s}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <>
              {msgs.map((m, i) => (
                <div key={i} className={`chat-msg ${m.role}`}>
                  <div className="avatar">{m.role === 'ai' ? 'AI' : 'U'}</div>
                  <div className="body" style={{ whiteSpace: 'pre-wrap' }}>
                    {m.text || <span className="muted"><Loader2 size={12} className="spin" style={{ marginRight: 4 }} />思考中...</span>}
                  </div>
                </div>
              ))}
            </>
          )}
          <div ref={bottomRef} />
        </div>

        <div className="card-header" style={{ borderTop: '1px solid var(--color-border-1)', borderBottom: 'none' }}>
          <Input
            value={input}
            onChange={setInput}
            onPressEnter={() => send()}
            placeholder="输入分析问题..."
            style={{ flex: 1 }}
            disabled={loading}
          />
          <Button type="primary" icon={<Send size={14} />} onClick={() => send()} loading={loading}>
            发送
          </Button>
        </div>
      </div>
    </div>
  );
}
