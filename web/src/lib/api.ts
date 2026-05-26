import axios from "axios";
import type {
  ChannelSummary,
  ChannelsResponse,
  DownstreamKey,
  LogsResponse,
  OpenAIChannel,
  SeriesPoint,
  SetupStatus,
} from "./types";

const client = axios.create({
  headers: { "content-type": "application/json" },
  withCredentials: true,
});

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await client.request<T>({
    data: init.body,
    headers: init.headers as Record<string, string> | undefined,
    method: init.method ?? "GET",
    url: path,
  });
  if (response.status === 204) {
    return undefined as T;
  }
  return response.data;
}

async function requestArray<T>(path: string, init: RequestInit = {}): Promise<T[]> {
  const data = await request<unknown>(path, init);
  return Array.isArray(data) ? (data as T[]) : [];
}

export const api = {
  setupStatus: () => request<SetupStatus>("/api/admin/setup/status"),
  setup: (username: string, password: string) =>
    request<{ token: string }>("/api/admin/setup", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  login: (username: string, password: string) =>
    request<{ username: string }>("/api/admin/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<void>("/api/admin/logout", { method: "POST" }),
  me: () => request<{ username: string }>("/api/admin/me"),
  channels: () => request<ChannelsResponse>("/api/admin/channels"),
  createOpenAIChannel: (channel: OpenAIChannel) =>
    request<void>("/api/admin/channels/openai-api", {
      method: "POST",
      body: JSON.stringify(channel),
    }),
  deleteChannel: (type: string, name: string) =>
    request<void>(`/api/admin/channels/${type}/${encodeURIComponent(name)}`, {
      method: "DELETE",
    }),
  probeModels: (baseUrl: string, apiKey: string) =>
    request<{ models: string[] }>("/api/admin/channels/probe-models", {
      method: "POST",
      body: JSON.stringify({ "base-url": baseUrl, "api-key": apiKey }),
    }),
  keys: () => requestArray<DownstreamKey>("/api/admin/keys"),
  createKey: (name: string, notes: string) =>
    request<{ token: string }>("/api/admin/keys", {
      method: "POST",
      body: JSON.stringify({ name, notes }),
    }),
  updateKey: (id: string, patch: Partial<DownstreamKey>) =>
    request<void>(`/api/admin/keys/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteKey: (id: string) =>
    request<void>(`/api/admin/keys/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
  logs: (params: URLSearchParams) => request<LogsResponse>(`/api/admin/logs?${params.toString()}`),
  channelStats: (window: string) => requestArray<ChannelSummary>(`/api/admin/stats/channels?window=${window}`),
  series: (channel: string, metric: string, window: string) =>
    requestArray<SeriesPoint>(
      `/api/admin/stats/series?channel=${encodeURIComponent(channel)}&metric=${metric}&window=${window}`,
    ),
};

export function getErrorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data;
    if (typeof data === "string") {
      return data;
    }
    if (data && typeof data === "object" && "error" in data) {
      return typeof data.error === "string" ? data.error : JSON.stringify(data.error);
    }
  }
  return error instanceof Error ? error.message : String(error);
}
