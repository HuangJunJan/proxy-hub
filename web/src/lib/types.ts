export type Language = "zh" | "en";
export type ThemeMode = "light" | "dark" | "system";

export interface SetupStatus {
  needed: boolean;
}

export interface OpenAIChannel {
  name: string;
  "base-url": string;
  priority?: number;
  "api-key-entries": APIKeyEntry[];
  models: ModelEntry[];
  disabled?: boolean;
  "timeout-sec"?: number;
  notes?: string;
}

export interface OAuthChannel {
  name: string;
  oauth?: {
    "access-token"?: string;
    "refresh-token"?: string;
    "expires-at"?: string;
  };
  models: ModelEntry[];
  disabled?: boolean;
  "timeout-sec"?: number;
  notes?: string;
}

export interface APIKeyEntry {
  "api-key": string;
  "proxy-url"?: string;
}

export interface ModelEntry {
  name: string;
  alias?: string;
}

export interface ChannelsResponse {
  "openai-api": OpenAIChannel[];
  "chatgpt-oauth": OAuthChannel[];
}

export interface ChannelHealthResult {
  ok: boolean;
  latencyMs?: number;
  error?: string;
}

export interface DownstreamKey {
  name?: string;
  notes?: string;
  token?: string;
  tokenMask: string;
  disabled?: boolean;
}

export interface RequestLog {
  id: number;
  ts: number;
  apiKeyTokenMask: string;
  apiKeyName?: string;
  endpoint?: string;
  requestType?: string;
  channelName?: string;
  channelType?: string;
  downstreamModel: string;
  upstreamModel?: string;
  upstreamKeyIndex?: number;
  statusCode: number;
  isStream: boolean;
  durationMs: number;
  firstTokenMs?: number;
  reasoningEffort?: string;
  billingMode?: string;
  promptTokens?: number;
  completionTokens?: number;
  reasoningTokens?: number;
  totalTokens?: number;
  errorKind?: string;
  errorMessage?: string;
  attempts: number;
  userAgent?: string;
}

export interface LogsResponse {
  items: RequestLog[];
  page: number;
  limit: number;
}

export interface ChannelSummary {
  channelName: string;
  requests: number;
  successes: number;
  failures: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  avgDurationMs: number;
}

export interface SeriesPoint {
  ts: number;
  value: number;
}

export type ChatRole = "assistant" | "system" | "user";

export interface ChatMessage {
  role: ChatRole;
  content: string;
}

export interface AdminChatRequest {
  channelType: "openai-api";
  channelName: string;
  model: string;
  messages: ChatMessage[];
}

export interface AdminChatResponse {
  content: string;
  promptTokens?: number;
  completionTokens?: number;
  totalTokens?: number;
  raw?: unknown;
}
