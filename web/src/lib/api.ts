import axios from "axios";
import type { AxiosRequestConfig } from "axios";
import { emitErrorMessage } from "./error-events";

window.addEventListener("error", (event) => {
  emitErrorMessage(getErrorMessage(event.error ?? event.message));
});
import type {
  AdminChatRequest,
  AdminChatResponse,
  ChannelSummary,
  ChannelHealthResult,
  ChannelsResponse,
  DownstreamKey,
  LogsResponse,
  OAuthChannel,
  OpenAIChannel,
  SeriesPoint,
  SetupStatus,
} from "./types";

const client = axios.create({
  headers: { "content-type": "application/json" },
  withCredentials: true,
});

client.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (!isGlobalErrorSuppressed(error)) {
      emitErrorMessage(getErrorMessage(error));
    }
    return Promise.reject(error);
  },
);

type ApiRequestInit = RequestInit & { suppressGlobalError?: boolean };

type ApiAxiosRequestConfig = AxiosRequestConfig & { suppressGlobalError?: boolean };

async function request<T>(path: string, init: ApiRequestInit = {}): Promise<T> {
  const response = await client.request<T>({
    data: init.body,
    headers: init.headers as Record<string, string> | undefined,
    method: init.method ?? "GET",
    suppressGlobalError: init.suppressGlobalError,
    url: path,
  } as ApiAxiosRequestConfig);
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
  me: (options?: Pick<ApiRequestInit, "suppressGlobalError">) =>
    request<{ username: string }>("/api/admin/me", options),
  channels: () => request<ChannelsResponse>("/api/admin/channels"),
  createOpenAIChannel: (channel: OpenAIChannel) =>
    request<void>("/api/admin/channels/openai-api", {
      method: "POST",
      body: JSON.stringify(channel),
    }),
  updateChannel: (type: "chatgpt-oauth" | "openai-api", name: string, channel: OAuthChannel | OpenAIChannel) =>
    request<void>(`/api/admin/channels/${type}/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(channel),
    }),
  deleteChannel: (type: string, name: string) =>
    request<void>(`/api/admin/channels/${type}/${encodeURIComponent(name)}`, {
      method: "DELETE",
    }),
  healthCheckChannel: (type: string, name: string) =>
    request<ChannelHealthResult>(`/api/admin/channels/${type}/${encodeURIComponent(name)}/health`, {
      method: "POST",
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
  chatCompletion: (payload: AdminChatRequest) =>
    request<AdminChatResponse>("/api/admin/chat/completions", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};

export function getErrorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data;
    const responseMessage = getResponseErrorMessage(data);
    if (responseMessage) {
      return responseMessage;
    }
  }
  return error instanceof Error ? error.message : String(error);
}

function isGlobalErrorSuppressed(error: unknown) {
  return axios.isAxiosError(error) && (error.config as ApiAxiosRequestConfig | undefined)?.suppressGlobalError === true;
}

function getResponseErrorMessage(data: unknown) {
  if (typeof data === "string") {
    return data;
  }
  if (!data || typeof data !== "object") {
    return "";
  }
  if ("message" in data && typeof data.message === "string") {
    return data.message;
  }
  if (!("error" in data)) {
    return "";
  }
  const { error } = data;
  if (typeof error === "string") {
    return error;
  }
  if (error && typeof error === "object" && "message" in error && typeof error.message === "string") {
    return error.message;
  }
  return JSON.stringify(error);
}
