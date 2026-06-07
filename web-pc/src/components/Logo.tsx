// 智策投研 Logo — SVG 矢量图标
export default function Logo({ size = 28 }: { size?: number }) {
  const s = size / 28;
  return (
    <svg width={28 * s} height={28 * s} viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
      {/* 背景圆角矩形 */}
      <rect x="0.5" y="0.5" width="27" height="27" rx="6" fill="url(#bg-grad)" stroke="url(#border-grad)" strokeWidth="0.8" />
      {/* K线柱体 — 红色实体柱 */}
      <rect x="5" y="14" width="2.3" height="7" rx="0.8" fill="url(#candle-red)" />
      {/* 上影线 */}
      <line x1="6.15" y1="10" x2="6.15" y2="14" stroke="url(#candle-red)" strokeWidth="0.9" />
      {/* K线柱体 — 绿色空心柱 */}
      <rect x="9" y="7" width="2.3" height="8" rx="0.8" fill="none" stroke="url(#candle-green)" strokeWidth="0.9" />
      {/* 上影线 */}
      <line x1="10.15" y1="4" x2="10.15" y2="7" stroke="url(#candle-green)" strokeWidth="0.9" />
      {/* 下影线 */}
      <line x1="10.15" y1="15" x2="10.15" y2="18" stroke="url(#candle-green)" strokeWidth="0.9" />
      {/* 上升趋势线（算法选股） */}
      <path d="M13 21L17 15L21 17L24 10" stroke="url(#trend)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" fill="none" />
      {/* 小箭头尖 */}
      <path d="M22.5 9.5L24 10L23.2 12.2" stroke="url(#trend)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
      {/* AI 小点 */}
      <circle cx="24" cy="10" r="1.6" fill="#3b82f6" opacity="0.9" />

      <defs>
        <linearGradient id="bg-grad" x1="0" y1="0" x2="28" y2="28">
          <stop stopColor="#1e293b" />
          <stop offset="1" stopColor="#0f172a" />
        </linearGradient>
        <linearGradient id="border-grad" x1="0" y1="0" x2="28" y2="28">
          <stop stopColor="#334155" />
          <stop offset="1" stopColor="#1e293b" />
        </linearGradient>
        <linearGradient id="candle-red" x1="0" y1="0" x2="0" y2="1">
          <stop stopColor="#ef4444" />
          <stop offset="1" stopColor="#dc2626" />
        </linearGradient>
        <linearGradient id="candle-green" x1="0" y1="0" x2="0" y2="1">
          <stop stopColor="#22c55e" />
          <stop offset="1" stopColor="#16a34a" />
        </linearGradient>
        <linearGradient id="trend" x1="13" y1="21" x2="25" y2="10">
          <stop stopColor="#3b82f6" />
          <stop offset="1" stopColor="#818cf8" />
        </linearGradient>
      </defs>
    </svg>
  );
}

// 大号版（用于登录页）
export function LogoLarge() {
  return (
    <svg width="64" height="64" viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect x="1" y="1" width="62" height="62" rx="14" fill="url(#bg-grad-lg)" stroke="url(#border-lg)" strokeWidth="1.5" />
      {/* K线柱体红色 */}
      <rect x="12" y="32" width="5" height="16" rx="1.5" fill="url(#red-lg)" />
      <line x1="14.5" y1="22" x2="14.5" y2="32" stroke="url(#red-lg)" strokeWidth="2" />
      {/* K线绿色 */}
      <rect x="21" y="14" width="5" height="19" rx="1.5" fill="none" stroke="url(#green-lg)" strokeWidth="2" />
      <line x1="23.5" y1="8" x2="23.5" y2="14" stroke="url(#green-lg)" strokeWidth="2" />
      <line x1="23.5" y1="33" x2="23.5" y2="41" stroke="url(#green-lg)" strokeWidth="2" />
      {/* 趋势线 */}
      <path d="M30 48L39 33L48 38L55 22" stroke="url(#trend-lg)" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" fill="none" />
      <path d="M51.5 20.5L55 22L53.2 27" stroke="url(#trend-lg)" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" fill="none" />
      {/* AI dot */}
      <circle cx="55" cy="22" r="3.5" fill="#3b82f6" opacity="0.9" />

      <defs>
        <linearGradient id="bg-grad-lg" x1="0" y1="0" x2="64" y2="64">
          <stop stopColor="#1e293b" />
          <stop offset="1" stopColor="#0f172a" />
        </linearGradient>
        <linearGradient id="border-lg" x1="0" y1="0" x2="64" y2="64">
          <stop stopColor="#334155" />
          <stop offset="1" stopColor="#1e293b" />
        </linearGradient>
        <linearGradient id="red-lg" x1="0" y1="0" x2="0" y2="1">
          <stop stopColor="#ef4444" />
          <stop offset="1" stopColor="#dc2626" />
        </linearGradient>
        <linearGradient id="green-lg" x1="0" y1="0" x2="0" y2="1">
          <stop stopColor="#22c55e" />
          <stop offset="1" stopColor="#16a34a" />
        </linearGradient>
        <linearGradient id="trend-lg" x1="30" y1="48" x2="56" y2="21">
          <stop stopColor="#3b82f6" />
          <stop offset="1" stopColor="#818cf8" />
        </linearGradient>
      </defs>
    </svg>
  );
}
