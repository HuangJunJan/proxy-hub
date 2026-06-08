// API 客户端：所有 /admin/* 请求注入 Authorization: Bearer <admin_key>（存 localStorage）。
// 开发期经 Vite 代理到 7777；生产同源（go:embed 单端口）。

const KEY_STORAGE = 'proxy-hub-admin-key'

export function getAdminKey(): string {
  return localStorage.getItem(KEY_STORAGE) ?? ''
}

export function setAdminKey(k: string): void {
  localStorage.setItem(KEY_STORAGE, k)
}

async function errText(res: Response): Promise<string> {
  try {
    const j = (await res.json()) as { error?: string }
    return j.error ?? `HTTP ${res.status}`
  } catch {
    return `HTTP ${res.status}`
  }
}

async function adminGet<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Authorization: `Bearer ${getAdminKey()}` } })
  if (!res.ok) throw new Error(await errText(res))
  return (await res.json()) as T
}

async function adminSend<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { Authorization: `Bearer ${getAdminKey()}`, 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) throw new Error(await errText(res))
  return (await res.json()) as T
}

function qs(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '' && v !== 0) sp.set(k, String(v))
  }
  return sp.toString()
}

export interface Overview {
  range: string
  request_count: number
  success_count: number
  error_count: number
  error_rate: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  reasoning_tokens: number
  total_cost: string
  avg_latency_ms: number
  avg_first_token_ms: number
  pricing_missing: string[]
  dropped_events: number
}

export interface TsPoint {
  bucket: string
  request_count: number
  error_count: number
  input_tokens: number
  output_tokens: number
  avg_latency_ms: number
}
export interface TimeseriesResp {
  interval: string
  points: TsPoint[]
}

export interface BreakdownRow {
  dim: string
  request_count: number
  error_count?: number
  input_tokens?: number
  output_tokens?: number
  cost?: string
  pricing_missing?: boolean
}
export interface BreakdownResp {
  by: string
  data: BreakdownRow[]
}

export interface LogRow {
  id: number
  request_id: string
  created_at: string
  api_key_id: number
  channel_id: number
  group: string
  requested_model: string
  upstream_model: string
  endpoint_format: string
  is_stream: boolean
  input_tokens: number
  output_tokens: number
  total_tokens: number
  latency_ms: number
  first_token_ms: number | null
  status_code: number
  is_error: boolean
  error_type: string
  session_id: string
  usage_source: string
  cost: string
}
export interface LogsResp {
  data: LogRow[]
  page: number
  size: number
  total: number
}

export interface HealthRow {
  channel_id: number
  model: string
  is_healthy: boolean
  consecutive_failures: number
  last_error: string
  cooldown_until: string
  updated_at: string
}
export interface HealthResp {
  data: HealthRow[]
}

export interface PricingRow {
  model_id: string
  input_per_million: string
  output_per_million: string
  cache_read_per_million: string
  cache_creation_per_million: string
  source: string
  updated_at: string
}
export interface PricingResp {
  data: PricingRow[]
}

export interface PricingInput {
  input_per_million: string
  output_per_million: string
  cache_read_per_million: string
  cache_creation_per_million: string
}

export interface LogQuery {
  request_id?: string
  api_key_id?: number
  channel_id?: number
  model?: string
  page?: number
  size?: number
}

export const api = {
  overview: (range: string) => adminGet<Overview>(`/admin/stats/overview?range=${range}`),
  timeseries: (range: string, interval: string) =>
    adminGet<TimeseriesResp>(`/admin/stats/timeseries?range=${range}&interval=${interval}`),
  breakdown: (by: string, range: string) =>
    adminGet<BreakdownResp>(`/admin/stats/breakdown?by=${by}&range=${range}`),
  logs: (q: LogQuery) =>
    adminGet<LogsResp>(
      `/admin/stats/logs?${qs({
        request_id: q.request_id,
        api_key_id: q.api_key_id,
        channel_id: q.channel_id,
        model: q.model,
        page: q.page,
        size: q.size,
      })}`,
    ),
  health: () => adminGet<HealthResp>(`/admin/stats/health`),
  pricing: () => adminGet<PricingResp>(`/admin/pricing`),
  upsertPricing: (model: string, body: PricingInput) =>
    adminSend<unknown>('PUT', `/admin/pricing/${encodeURIComponent(model)}`, body),
  deletePricing: (model: string) =>
    adminSend<unknown>('DELETE', `/admin/pricing/${encodeURIComponent(model)}`),
}
