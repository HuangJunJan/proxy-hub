import { useState } from 'react'
import { getAdminKey, setAdminKey } from './api'
import { BreakdownPage, HealthPage, LogsPage, OverviewPage, PricingPage, TimeseriesPage } from './pages'

const TABS = [
  { id: 'overview', label: '概览' },
  { id: 'timeseries', label: '趋势' },
  { id: 'breakdown', label: '分组' },
  { id: 'logs', label: '请求日志' },
  { id: 'health', label: '渠道健康' },
  { id: 'pricing', label: '定价' },
] as const
type TabID = (typeof TABS)[number]['id']

const RANGES = ['1h', '24h', '7d', '30d']

export default function App() {
  const [key, setKey] = useState(getAdminKey())
  const [tab, setTab] = useState<TabID>('overview')
  const [range, setRange] = useState('24h')

  if (!key) {
    return (
      <KeyGate
        onSet={(k) => {
          setAdminKey(k)
          setKey(k)
        }}
      />
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 text-gray-900">
      <header className="flex items-center justify-between border-b bg-white px-6 py-3">
        <h1 className="text-lg font-semibold">proxy-hub 控制台</h1>
        <div className="flex items-center gap-3">
          {tab !== 'logs' && tab !== 'health' && tab !== 'pricing' && (
            <select
              value={range}
              onChange={(e) => setRange(e.target.value)}
              className="rounded border px-2 py-1 text-sm"
            >
              {RANGES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          )}
          <button
            onClick={() => {
              setAdminKey('')
              setKey('')
            }}
            className="rounded border px-3 py-1 text-sm text-gray-600 hover:bg-gray-100"
          >
            退出
          </button>
        </div>
      </header>

      <nav className="flex gap-1 border-b bg-white px-4">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={
              'px-4 py-2 text-sm font-medium border-b-2 ' +
              (tab === t.id ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-800')
            }
          >
            {t.label}
          </button>
        ))}
      </nav>

      <main className="mx-auto max-w-7xl p-6">
        {tab === 'overview' && <OverviewPage range={range} />}
        {tab === 'timeseries' && <TimeseriesPage range={range} />}
        {tab === 'breakdown' && <BreakdownPage range={range} />}
        {tab === 'logs' && <LogsPage />}
        {tab === 'health' && <HealthPage />}
        {tab === 'pricing' && <PricingPage />}
      </main>
    </div>
  )
}

function KeyGate({ onSet }: { onSet: (k: string) => void }) {
  const [v, setV] = useState('')
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50">
      <div className="w-96 rounded-lg bg-white p-8 shadow">
        <h1 className="mb-4 text-lg font-semibold">proxy-hub 控制台</h1>
        <p className="mb-3 text-sm text-gray-500">输入 admin key 以访问管理面板。</p>
        <input
          type="password"
          value={v}
          onChange={(e) => setV(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && v) onSet(v)
          }}
          placeholder="admin key"
          className="mb-3 w-full rounded border px-3 py-2"
        />
        <button onClick={() => v && onSet(v)} className="w-full rounded bg-blue-600 py-2 text-white hover:bg-blue-700">
          进入
        </button>
      </div>
    </div>
  )
}
