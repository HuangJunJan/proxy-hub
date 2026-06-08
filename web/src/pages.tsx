import { useState } from 'react'
import type { ReactElement, ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { api, type BreakdownRow, type LogQuery, type MCPServerInput, type MCPTargetInput, type PricingInput } from './api'

// ---- 共享小组件 ----

function Loading() {
  return <div className="py-10 text-center text-sm text-gray-400">加载中…</div>
}

function ErrorBox({ error }: { error: unknown }) {
  const msg = error instanceof Error ? error.message : String(error)
  return <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">出错：{msg}</div>
}

function Card({ title, value, sub }: { title: string; value: string; sub?: string }) {
  return (
    <div className="rounded-lg border bg-white p-4 shadow-sm">
      <div className="text-xs uppercase tracking-wide text-gray-400">{title}</div>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
      {sub && <div className="mt-1 text-xs text-gray-400">{sub}</div>}
    </div>
  )
}

const fmtInt = (n: number) => n.toLocaleString('en-US')
const fmtMs = (n: number) => `${n.toFixed(0)} ms`
const fmtPct = (n: number) => `${(n * 100).toFixed(2)}%`

// ---- 概览 ----

export function OverviewPage({ range }: { range: string }) {
  const { data, isLoading, error } = useQuery({ queryKey: ['overview', range], queryFn: () => api.overview(range) })
  if (isLoading) return <Loading />
  if (error) return <ErrorBox error={error} />
  if (!data) return null
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Card title="请求数" value={fmtInt(data.request_count)} sub={`成功 ${fmtInt(data.success_count)}`} />
        <Card title="错误率" value={fmtPct(data.error_rate)} sub={`错误 ${fmtInt(data.error_count)}`} />
        <Card title="总成本 (USD)" value={`$${data.total_cost}`} sub="读取时按定价计算" />
        <Card title="平均延迟" value={fmtMs(data.avg_latency_ms)} sub={`TTFT ${fmtMs(data.avg_first_token_ms)}`} />
        <Card title="输入 token" value={fmtInt(data.input_tokens)} sub={`缓存读 ${fmtInt(data.cache_read_tokens)}`} />
        <Card title="输出 token" value={fmtInt(data.output_tokens)} sub={`推理 ${fmtInt(data.reasoning_tokens)}`} />
        <Card
          title="缓存创建 token"
          value={fmtInt(data.cache_creation_tokens)}
          sub={`缓存读 ${fmtInt(data.cache_read_tokens)}`}
        />
        <Card title="丢弃事件" value={fmtInt(data.dropped_events)} sub={data.dropped_events > 0 ? '缓冲溢出（非静默）' : '无'} />
      </div>
      {data.pricing_missing.length > 0 && (
        <div className="rounded border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          缺定价模型（成本按 0 计）：{data.pricing_missing.join('、')}。可在「定价」页补充。
        </div>
      )}
    </div>
  )
}

// ---- 趋势 ----

export function TimeseriesPage({ range }: { range: string }) {
  const [interval, setInterval] = useState('hour')
  const { data, isLoading, error } = useQuery({
    queryKey: ['timeseries', range, interval],
    queryFn: () => api.timeseries(range, interval),
  })
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 text-sm">
        <span className="text-gray-500">粒度</span>
        <select value={interval} onChange={(e) => setInterval(e.target.value)} className="rounded border px-2 py-1">
          <option value="hour">小时</option>
          <option value="day">天</option>
        </select>
      </div>
      {isLoading && <Loading />}
      {error && <ErrorBox error={error} />}
      {data && (
        <div className="space-y-6">
          <ChartCard title="Token（输入 / 输出）">
            <LineChart data={data.points}>
              <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
              <XAxis dataKey="bucket" tick={{ fontSize: 11 }} minTickGap={24} />
              <YAxis tick={{ fontSize: 11 }} width={70} />
              <Tooltip />
              <Line type="monotone" dataKey="input_tokens" name="输入" stroke="#2563eb" dot={false} />
              <Line type="monotone" dataKey="output_tokens" name="输出" stroke="#16a34a" dot={false} />
            </LineChart>
          </ChartCard>
          <ChartCard title="请求数 / 错误数">
            <LineChart data={data.points}>
              <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
              <XAxis dataKey="bucket" tick={{ fontSize: 11 }} minTickGap={24} />
              <YAxis tick={{ fontSize: 11 }} width={50} />
              <Tooltip />
              <Line type="monotone" dataKey="request_count" name="请求" stroke="#2563eb" dot={false} />
              <Line type="monotone" dataKey="error_count" name="错误" stroke="#dc2626" dot={false} />
            </LineChart>
          </ChartCard>
        </div>
      )}
    </div>
  )
}

function ChartCard({ title, children }: { title: string; children: ReactElement }) {
  return (
    <div className="rounded-lg border bg-white p-4 shadow-sm">
      <div className="mb-2 text-sm font-medium text-gray-600">{title}</div>
      <div style={{ width: '100%', height: 260 }}>
        <ResponsiveContainer>{children}</ResponsiveContainer>
      </div>
    </div>
  )
}

// ---- 分组 ----

const BREAKDOWN_BY = [
  { id: 'model', label: '模型' },
  { id: 'channel', label: '渠道' },
  { id: 'api_key', label: 'API Key' },
  { id: 'error_type', label: '错误类型' },
]

export function BreakdownPage({ range }: { range: string }) {
  const [by, setBy] = useState('model')
  const { data, isLoading, error } = useQuery({
    queryKey: ['breakdown', by, range],
    queryFn: () => api.breakdown(by, range),
  })
  return (
    <div className="space-y-4">
      <div className="flex gap-1">
        {BREAKDOWN_BY.map((b) => (
          <button
            key={b.id}
            onClick={() => setBy(b.id)}
            className={
              'rounded px-3 py-1 text-sm ' + (by === b.id ? 'bg-blue-600 text-white' : 'bg-white border text-gray-600')
            }
          >
            {b.label}
          </button>
        ))}
      </div>
      {isLoading && <Loading />}
      {error && <ErrorBox error={error} />}
      {data && <BreakdownTable rows={data.data} by={by} />}
    </div>
  )
}

function BreakdownTable({ rows, by }: { rows: BreakdownRow[]; by: string }) {
  if (rows.length === 0) return <div className="py-8 text-center text-sm text-gray-400">区间内无数据</div>
  const showTokens = by !== 'error_type'
  const showCost = by === 'model'
  return (
    <div className="overflow-x-auto rounded-lg border bg-white">
      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
          <tr>
            <th className="px-4 py-2">{by}</th>
            <th className="px-4 py-2 text-right">请求</th>
            {showTokens && <th className="px-4 py-2 text-right">输入</th>}
            {showTokens && <th className="px-4 py-2 text-right">输出</th>}
            {showCost && <th className="px-4 py-2 text-right">成本</th>}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="border-t">
              <td className="px-4 py-2 font-mono text-xs">
                {r.dim || '(空)'}
                {r.pricing_missing && <span className="ml-2 text-amber-600">缺价</span>}
              </td>
              <td className="px-4 py-2 text-right">{fmtInt(r.request_count)}</td>
              {showTokens && <td className="px-4 py-2 text-right">{fmtInt(r.input_tokens ?? 0)}</td>}
              {showTokens && <td className="px-4 py-2 text-right">{fmtInt(r.output_tokens ?? 0)}</td>}
              {showCost && <td className="px-4 py-2 text-right">${r.cost ?? '0'}</td>}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ---- 请求日志钻取 ----

export function LogsPage() {
  const [form, setForm] = useState<LogQuery>({ page: 1, size: 50 })
  const [active, setActive] = useState<LogQuery>({ page: 1, size: 50 })
  const { data, isLoading, error } = useQuery({ queryKey: ['logs', active], queryFn: () => api.logs(active) })

  const search = () => setActive({ ...form, page: 1 })
  const setPage = (p: number) => setActive((a) => ({ ...a, page: p }))

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-2">
        <Field label="request_id">
          <input
            className="w-56 rounded border px-2 py-1 text-sm"
            value={form.request_id ?? ''}
            onChange={(e) => setForm({ ...form, request_id: e.target.value })}
          />
        </Field>
        <Field label="模型">
          <input
            className="w-40 rounded border px-2 py-1 text-sm"
            value={form.model ?? ''}
            onChange={(e) => setForm({ ...form, model: e.target.value })}
          />
        </Field>
        <Field label="渠道 ID">
          <input
            className="w-24 rounded border px-2 py-1 text-sm"
            value={form.channel_id ?? ''}
            onChange={(e) => setForm({ ...form, channel_id: Number(e.target.value) || undefined })}
          />
        </Field>
        <Field label="Key ID">
          <input
            className="w-24 rounded border px-2 py-1 text-sm"
            value={form.api_key_id ?? ''}
            onChange={(e) => setForm({ ...form, api_key_id: Number(e.target.value) || undefined })}
          />
        </Field>
        <button onClick={search} className="rounded bg-blue-600 px-4 py-1.5 text-sm text-white hover:bg-blue-700">
          查询
        </button>
      </div>

      {isLoading && <Loading />}
      {error && <ErrorBox error={error} />}
      {data && (
        <>
          <div className="overflow-x-auto rounded-lg border bg-white">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
                <tr>
                  <th className="px-3 py-2">时间</th>
                  <th className="px-3 py-2">模型</th>
                  <th className="px-3 py-2 text-right">渠道</th>
                  <th className="px-3 py-2 text-right">状态</th>
                  <th className="px-3 py-2 text-right">输入</th>
                  <th className="px-3 py-2 text-right">输出</th>
                  <th className="px-3 py-2 text-right">延迟</th>
                  <th className="px-3 py-2 text-right">成本</th>
                  <th className="px-3 py-2">request_id</th>
                </tr>
              </thead>
              <tbody>
                {data.data.map((r) => (
                  <tr key={r.id} className={'border-t ' + (r.is_error ? 'bg-red-50' : '')}>
                    <td className="px-3 py-2 whitespace-nowrap text-xs text-gray-500">{r.created_at}</td>
                    <td className="px-3 py-2 font-mono text-xs">{r.requested_model}</td>
                    <td className="px-3 py-2 text-right">{r.channel_id}</td>
                    <td className="px-3 py-2 text-right">
                      {r.status_code}
                      {r.error_type && <span className="ml-1 text-xs text-red-600">{r.error_type}</span>}
                    </td>
                    <td className="px-3 py-2 text-right">{fmtInt(r.input_tokens)}</td>
                    <td className="px-3 py-2 text-right">{fmtInt(r.output_tokens)}</td>
                    <td className="px-3 py-2 text-right">{r.latency_ms}ms</td>
                    <td className="px-3 py-2 text-right">${r.cost}</td>
                    <td className="px-3 py-2 font-mono text-xs text-gray-400">{r.request_id.slice(0, 12)}</td>
                  </tr>
                ))}
                {data.data.length === 0 && (
                  <tr>
                    <td colSpan={9} className="py-8 text-center text-gray-400">
                      无匹配日志
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <Pager page={data.page} size={data.size} total={data.total} onPage={setPage} />
        </>
      )}
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-gray-500">{label}</span>
      {children}
    </label>
  )
}

function Pager({ page, size, total, onPage }: { page: number; size: number; total: number; onPage: (p: number) => void }) {
  const pages = Math.max(1, Math.ceil(total / size))
  return (
    <div className="flex items-center justify-between text-sm text-gray-500">
      <span>
        共 {fmtInt(total)} 条 · 第 {page}/{pages} 页
      </span>
      <div className="flex gap-2">
        <button disabled={page <= 1} onClick={() => onPage(page - 1)} className="rounded border px-3 py-1 disabled:opacity-40">
          上一页
        </button>
        <button disabled={page >= pages} onClick={() => onPage(page + 1)} className="rounded border px-3 py-1 disabled:opacity-40">
          下一页
        </button>
      </div>
    </div>
  )
}

// ---- 渠道健康 ----

export function HealthPage() {
  const { data, isLoading, error } = useQuery({ queryKey: ['health'], queryFn: () => api.health() })
  if (isLoading) return <Loading />
  if (error) return <ErrorBox error={error} />
  if (!data) return null
  return (
    <div className="overflow-x-auto rounded-lg border bg-white">
      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
          <tr>
            <th className="px-4 py-2 text-right">渠道</th>
            <th className="px-4 py-2">模型</th>
            <th className="px-4 py-2">状态</th>
            <th className="px-4 py-2 text-right">连续失败</th>
            <th className="px-4 py-2">冷却至</th>
            <th className="px-4 py-2">最近错误</th>
          </tr>
        </thead>
        <tbody>
          {data.data.map((h, i) => (
            <tr key={i} className="border-t">
              <td className="px-4 py-2 text-right">{h.channel_id}</td>
              <td className="px-4 py-2 font-mono text-xs">{h.model}</td>
              <td className="px-4 py-2">
                <span className={h.is_healthy ? 'text-green-600' : 'text-red-600'}>
                  {h.is_healthy ? '健康' : '异常'}
                </span>
              </td>
              <td className="px-4 py-2 text-right">{h.consecutive_failures}</td>
              <td className="px-4 py-2 text-xs text-gray-500">{h.cooldown_until || '-'}</td>
              <td className="px-4 py-2 text-xs text-gray-500">{h.last_error || '-'}</td>
            </tr>
          ))}
          {data.data.length === 0 && (
            <tr>
              <td colSpan={6} className="py-8 text-center text-gray-400">
                暂无健康记录
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

// ---- 定价 ----

const emptyPricing: PricingInput = {
  input_per_million: '',
  output_per_million: '',
  cache_read_per_million: '',
  cache_creation_per_million: '',
}

export function PricingPage() {
  const qc = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey: ['pricing'], queryFn: () => api.pricing() })
  const [model, setModel] = useState('')
  const [form, setForm] = useState<PricingInput>(emptyPricing)

  const upsert = useMutation({
    mutationFn: () => api.upsertPricing(model, form),
    onSuccess: () => {
      setModel('')
      setForm(emptyPricing)
      qc.invalidateQueries({ queryKey: ['pricing'] })
    },
  })
  const del = useMutation({
    mutationFn: (m: string) => api.deletePricing(m),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pricing'] }),
  })

  return (
    <div className="space-y-4">
      <div className="rounded-lg border bg-white p-4 shadow-sm">
        <div className="mb-2 text-sm font-medium text-gray-600">新增 / 覆盖定价（每百万 token，USD）</div>
        <div className="flex flex-wrap items-end gap-2">
          <Field label="model_id">
            <input className="w-48 rounded border px-2 py-1 text-sm" value={model} onChange={(e) => setModel(e.target.value)} />
          </Field>
          <Field label="输入">
            <input
              className="w-24 rounded border px-2 py-1 text-sm"
              value={form.input_per_million}
              onChange={(e) => setForm({ ...form, input_per_million: e.target.value })}
            />
          </Field>
          <Field label="输出">
            <input
              className="w-24 rounded border px-2 py-1 text-sm"
              value={form.output_per_million}
              onChange={(e) => setForm({ ...form, output_per_million: e.target.value })}
            />
          </Field>
          <Field label="缓存读">
            <input
              className="w-24 rounded border px-2 py-1 text-sm"
              value={form.cache_read_per_million}
              onChange={(e) => setForm({ ...form, cache_read_per_million: e.target.value })}
            />
          </Field>
          <Field label="缓存创建">
            <input
              className="w-24 rounded border px-2 py-1 text-sm"
              value={form.cache_creation_per_million}
              onChange={(e) => setForm({ ...form, cache_creation_per_million: e.target.value })}
            />
          </Field>
          <button
            disabled={!model || upsert.isPending}
            onClick={() => upsert.mutate()}
            className="rounded bg-blue-600 px-4 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-40"
          >
            保存
          </button>
        </div>
        {upsert.error && <div className="mt-2 text-sm text-red-600">{(upsert.error as Error).message}</div>}
      </div>

      {isLoading && <Loading />}
      {error && <ErrorBox error={error} />}
      {data && (
        <div className="overflow-x-auto rounded-lg border bg-white">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-2">模型</th>
                <th className="px-4 py-2 text-right">输入</th>
                <th className="px-4 py-2 text-right">输出</th>
                <th className="px-4 py-2 text-right">缓存读</th>
                <th className="px-4 py-2 text-right">缓存创建</th>
                <th className="px-4 py-2">来源</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {data.data.map((p) => (
                <tr key={p.model_id} className="border-t">
                  <td className="px-4 py-2 font-mono text-xs">{p.model_id}</td>
                  <td className="px-4 py-2 text-right">{p.input_per_million}</td>
                  <td className="px-4 py-2 text-right">{p.output_per_million}</td>
                  <td className="px-4 py-2 text-right">{p.cache_read_per_million}</td>
                  <td className="px-4 py-2 text-right">{p.cache_creation_per_million}</td>
                  <td className="px-4 py-2 text-xs text-gray-500">{p.source}</td>
                  <td className="px-4 py-2 text-right">
                    <button
                      onClick={() => {
                        setModel(p.model_id)
                        setForm({
                          input_per_million: p.input_per_million,
                          output_per_million: p.output_per_million,
                          cache_read_per_million: p.cache_read_per_million,
                          cache_creation_per_million: p.cache_creation_per_million,
                        })
                      }}
                      className="mr-2 text-blue-600 hover:underline"
                    >
                      编辑
                    </button>
                    <button onClick={() => del.mutate(p.model_id)} className="text-red-600 hover:underline">
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ---- MCP 共享管理 ----

export function MCPPage() {
  const qc = useQueryClient()
  const servers = useQuery({ queryKey: ['mcp-servers'], queryFn: () => api.mcpServers() })
  const targets = useQuery({ queryKey: ['mcp-targets'], queryFn: () => api.mcpTargets() })
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['mcp-servers'] })
    qc.invalidateQueries({ queryKey: ['mcp-targets'] })
  }
  const toggle = useMutation({
    mutationFn: (v: { id: string; client: string; enabled: boolean }) => api.mcpToggle(v.id, v.client, v.enabled),
    onSuccess: invalidate,
  })
  const delServer = useMutation({ mutationFn: (id: string) => api.mcpDeleteServer(id), onSuccess: invalidate })
  const sync = useMutation({ mutationFn: () => api.mcpSync(), onSuccess: invalidate })
  const delTarget = useMutation({ mutationFn: (id: number) => api.mcpDeleteTarget(id), onSuccess: invalidate })
  const importT = useMutation({ mutationFn: (id: number) => api.mcpImport(id), onSuccess: invalidate })

  const [sid, setSid] = useState('')
  const [specText, setSpecText] = useState('{\n  "type": "stdio",\n  "command": "npx",\n  "args": ["-y", "<pkg>"]\n}')
  const [addErr, setAddErr] = useState('')
  const createServer = useMutation({
    mutationFn: (body: MCPServerInput) => api.mcpCreateServer(body),
    onSuccess: () => {
      setSid('')
      setAddErr('')
      invalidate()
    },
    onError: (e) => setAddErr((e as Error).message),
  })
  const submitServer = () => {
    setAddErr('')
    if (!sid) {
      setAddErr('需填 id')
      return
    }
    let spec: Record<string, unknown>
    try {
      spec = JSON.parse(specText)
    } catch {
      setAddErr('spec 不是合法 JSON')
      return
    }
    createServer.mutate({ id: sid, spec })
  }

  const [tClient, setTClient] = useState('claude')
  const [tPath, setTPath] = useState('')
  const [tLabel, setTLabel] = useState('')
  const createTarget = useMutation({
    mutationFn: (body: MCPTargetInput) => api.mcpCreateTarget(body),
    onSuccess: () => {
      setTPath('')
      setTLabel('')
      invalidate()
    },
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium text-gray-600">MCP 服务器（SSOT → 投影到 Codex / Claude）</h2>
        <button
          onClick={() => sync.mutate()}
          disabled={sync.isPending}
          className="rounded bg-green-600 px-4 py-1.5 text-sm text-white hover:bg-green-700 disabled:opacity-40"
        >
          {sync.isPending ? '同步中…' : '立即同步全部'}
        </button>
      </div>

      {/* 新增 server */}
      <div className="rounded-lg border bg-white p-4 shadow-sm">
        <div className="mb-2 text-sm font-medium text-gray-600">新增 / 覆盖 server</div>
        <div className="flex flex-wrap gap-3">
          <Field label="id（配置键）">
            <input className="w-48 rounded border px-2 py-1 text-sm" value={sid} onChange={(e) => setSid(e.target.value)} />
          </Field>
          <label className="flex flex-1 flex-col gap-1">
            <span className="text-xs text-gray-500">spec（JSON：stdio command/args/env 或 http url/headers）</span>
            <textarea
              className="h-28 w-full rounded border px-2 py-1 font-mono text-xs"
              value={specText}
              onChange={(e) => setSpecText(e.target.value)}
            />
          </label>
        </div>
        <div className="mt-2 flex items-center gap-3">
          <button
            onClick={submitServer}
            disabled={createServer.isPending}
            className="rounded bg-blue-600 px-4 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-40"
          >
            保存
          </button>
          {addErr && <span className="text-sm text-red-600">{addErr}</span>}
        </div>
      </div>

      {/* server 列表 */}
      {servers.isLoading && <Loading />}
      {servers.error && <ErrorBox error={servers.error} />}
      {servers.data && (
        <div className="overflow-x-auto rounded-lg border bg-white">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-2">id</th>
                <th className="px-4 py-2">type</th>
                <th className="px-4 py-2 text-center">Codex</th>
                <th className="px-4 py-2 text-center">Claude</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {servers.data.data.map((s) => (
                <tr key={s.id} className="border-t">
                  <td className="px-4 py-2 font-mono text-xs">{s.id}</td>
                  <td className="px-4 py-2 text-xs text-gray-500">{String(s.spec['type'] ?? 'stdio')}</td>
                  <td className="px-4 py-2 text-center">
                    <input
                      type="checkbox"
                      checked={s.enabled_codex}
                      onChange={(e) => toggle.mutate({ id: s.id, client: 'codex', enabled: e.target.checked })}
                    />
                  </td>
                  <td className="px-4 py-2 text-center">
                    <input
                      type="checkbox"
                      checked={s.enabled_claude}
                      onChange={(e) => toggle.mutate({ id: s.id, client: 'claude', enabled: e.target.checked })}
                    />
                  </td>
                  <td className="px-4 py-2 text-right">
                    <button onClick={() => delServer.mutate(s.id)} className="text-red-600 hover:underline">
                      删除
                    </button>
                  </td>
                </tr>
              ))}
              {servers.data.data.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-gray-400">
                    暂无 MCP server
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* 同步目标 */}
      <div className="rounded-lg border bg-white p-4 shadow-sm">
        <div className="mb-2 text-sm font-medium text-gray-600">同步目标（显式登记的客户端配置文件，绝对路径）</div>
        <div className="flex flex-wrap items-end gap-2">
          <Field label="client">
            <select value={tClient} onChange={(e) => setTClient(e.target.value)} className="rounded border px-2 py-1 text-sm">
              <option value="claude">claude</option>
              <option value="codex">codex</option>
            </select>
          </Field>
          <Field label="config_path">
            <input className="w-80 rounded border px-2 py-1 text-sm" value={tPath} onChange={(e) => setTPath(e.target.value)} />
          </Field>
          <Field label="label">
            <input className="w-32 rounded border px-2 py-1 text-sm" value={tLabel} onChange={(e) => setTLabel(e.target.value)} />
          </Field>
          <button
            disabled={!tPath || createTarget.isPending}
            onClick={() => createTarget.mutate({ client: tClient, config_path: tPath, label: tLabel })}
            className="rounded bg-blue-600 px-4 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-40"
          >
            登记
          </button>
        </div>
      </div>

      {targets.data && (
        <div className="overflow-x-auto rounded-lg border bg-white">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-2">client</th>
                <th className="px-4 py-2">config_path</th>
                <th className="px-4 py-2">最近状态</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {targets.data.data.map((t) => (
                <tr key={t.id} className="border-t">
                  <td className="px-4 py-2">{t.client}</td>
                  <td className="px-4 py-2 font-mono text-xs">{t.config_path}</td>
                  <td className="px-4 py-2 text-xs text-gray-500">{t.last_sync_status || '-'}</td>
                  <td className="px-4 py-2 text-right">
                    <button onClick={() => importT.mutate(t.id)} className="mr-2 text-blue-600 hover:underline">
                      导入
                    </button>
                    <button onClick={() => delTarget.mutate(t.id)} className="text-red-600 hover:underline">
                      删除
                    </button>
                  </td>
                </tr>
              ))}
              {targets.data.data.length === 0 && (
                <tr>
                  <td colSpan={4} className="py-8 text-center text-gray-400">
                    暂无同步目标
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
