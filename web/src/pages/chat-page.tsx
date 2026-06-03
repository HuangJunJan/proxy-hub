import { Bot, Eraser, Send, User } from "lucide-react";
import {
  FormEvent,
  KeyboardEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Page,
  PageBody,
  PageDescription,
  PageHeader,
  PageTitle,
} from "../components/layout/page";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/field";
import { Input } from "../components/ui/input";
import { PageActions, RefreshButton } from "../components/ui/page-actions";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import { SectionCard } from "../components/ui/section-card";
import { Textarea } from "../components/ui/textarea";
import { useConfirm } from "../components/ui/use-confirm";
import { api } from "../lib/api";
import { useAppContext } from "../lib/app-context";
import { useAsyncAction } from "../lib/use-async-action";
import type {
  ChatMessage,
  ChannelsResponse,
  ModelEntry,
  OpenAIChannel,
} from "../lib/types";

type ChatLine = ChatMessage & { id: string };
type ModelOption = { label: string; value: string };

const emptyChannels: ChannelsResponse = {
  "chatgpt-oauth": [],
  "openai-api": [],
};

export function ChatPage() {
  const { t } = useAppContext();
  const { confirm, confirmDialog } = useConfirm();
  const [channels, setChannels] = useState<ChannelsResponse>(emptyChannels);
  const [channelName, setChannelName] = useState("");
  const [model, setModel] = useState("");
  const [messages, setMessages] = useState<ChatLine[]>([]);
  const [draft, setDraft] = useState("");
  const bottomRef = useRef<HTMLDivElement | null>(null);

  const openAIChannels = useMemo(
    () => channels["openai-api"].filter((channel) => !channel.disabled),
    [channels],
  );
  const selectedChannel = useMemo(
    () =>
      openAIChannels.find((channel) => channel.name === channelName) ?? null,
    [channelName, openAIChannels],
  );
  const modelOptions = useMemo(
    () => modelOptionsForChannel(selectedChannel),
    [selectedChannel],
  );

  async function refresh() {
    try {
      const next = await api.channels();
      setChannels(next);
    } catch {
      // Global axios interceptor displays the error toast.
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (openAIChannels.length === 0) {
      setChannelName("");
      return;
    }
    if (!openAIChannels.some((channel) => channel.name === channelName)) {
      setChannelName(openAIChannels[0].name);
    }
  }, [channelName, openAIChannels]);

  useEffect(() => {
    if (!selectedChannel) {
      setModel("");
      return;
    }
    if (!model.trim() && modelOptions.length > 0) {
      setModel(modelOptions[0].value);
    }
  }, [model, modelOptions, selectedChannel]);

  const { loading, run: sendMessage } = useAsyncAction(async () => {
    const content = draft.trim();
    const requestedModel = model.trim();
    if (!selectedChannel || !requestedModel || !content) {
      return;
    }
    const previousMessages = messages;
    const userMessage: ChatLine = {
      content,
      id: crypto.randomUUID(),
      role: "user",
    };
    const nextMessages = [...messages, userMessage];
    setMessages(nextMessages);
    setDraft("");
    try {
      const response = await api.chatCompletion({
        channelName: selectedChannel.name,
        channelType: "openai-api",
        messages: nextMessages.map(({ content: messageContent, role }) => ({
          content: messageContent,
          role,
        })),
        model: requestedModel,
      });
      setMessages((current) => [
        ...current,
        {
          content:
            response.content || JSON.stringify(response.raw ?? {}, null, 2),
          id: crypto.randomUUID(),
          role: "assistant",
        },
      ]);
    } catch {
      setMessages(previousMessages);
    }
  });

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end" });
  }, [messages, loading]);

  function send(event?: FormEvent) {
    event?.preventDefault();
    void sendMessage();
  }

  function handleDraftKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void send();
    }
  }

  return (
    <Page>
      <PageHeader
        actions={
          <PageActions>
            <RefreshButton label={t("refresh")} onClick={refresh} />
            <Button
              onClick={async () => {
                const confirmed = await confirm({
                  cancelLabel: t("cancel"),
                  confirmLabel: t("clear"),
                  description: t("confirmClearChat"),
                  title: t("confirmClearChatTitle"),
                  tone: "destructive",
                });
                if (confirmed) {
                  setMessages([]);
                }
              }}
              type="button"
              variant="outline"
            >
              <Eraser size={16} />
              {t("clear")}
            </Button>
          </PageActions>
        }
      >
        <PageTitle visuallyHidden>{t("chat")}</PageTitle>
        <PageDescription>
          {selectedChannel ? selectedChannel["base-url"] : t("emptyChat")}
        </PageDescription>
      </PageHeader>
      <PageBody className="chat-layout">
        <aside className="chat-controls">
          <SectionCard
            description={selectedChannel?.name || t("empty")}
            title={t("connection")}
          >
            <div className="form-stack">
              <Field label={t("channelName")}>
                <Select
                  value={channelName || "none"}
                  onValueChange={(value) =>
                    setChannelName(value === "none" ? "" : value)
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {openAIChannels.length === 0 && (
                      <SelectItem value="none">{t("empty")}</SelectItem>
                    )}
                    {openAIChannels.map((channel) => (
                      <SelectItem key={channel.name} value={channel.name}>
                        {channel.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label={t("model")}>
                <Input
                  disabled={!selectedChannel}
                  list="admin-chat-model-options"
                  placeholder={t("modelPlaceholder")}
                  value={model}
                  onChange={(event) => setModel(event.target.value)}
                />
                <datalist id="admin-chat-model-options">
                  {modelOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </datalist>
              </Field>
              <div className="chat-meta">
                <Badge variant={selectedChannel ? "success" : "muted"}>
                  {selectedChannel ? t("configured") : t("notConfigured")}
                </Badge>
                {selectedChannel && <span>{selectedChannel["base-url"]}</span>}
              </div>
            </div>
          </SectionCard>
        </aside>
        {confirmDialog}
        <main className="chat-panel">
          <SectionCard className="chat-card" title={t("chat")}>
            <div className="chat-thread" aria-live="polite">
              {messages.length === 0 && (
                <div className="chat-empty">
                  <Bot size={26} />
                  <span>{t("emptyChat")}</span>
                </div>
              )}
              {messages.map((message) => (
                <div
                  className={`chat-message ${message.role}`}
                  key={message.id}
                >
                  <span className="chat-avatar" aria-hidden="true">
                    {message.role === "assistant" ? (
                      <Bot size={16} />
                    ) : (
                      <User size={16} />
                    )}
                  </span>
                  <p>{message.content}</p>
                </div>
              ))}
              {loading && (
                <div className="chat-message assistant">
                  <span className="chat-avatar" aria-hidden="true">
                    <Bot size={16} />
                  </span>
                  <span className="spinner" />
                </div>
              )}
              <div ref={bottomRef} />
            </div>
            <form className="chat-compose" onSubmit={send}>
              <Textarea
                disabled={!selectedChannel || !model.trim() || loading}
                onChange={(event) => setDraft(event.target.value)}
                onKeyDown={handleDraftKeyDown}
                placeholder={t("chatPlaceholder")}
                value={draft}
              />
              <Button
                disabled={
                  !selectedChannel || !model.trim() || !draft.trim() || loading
                }
                type="submit"
              >
                <Send size={16} />
                {t("send")}
              </Button>
            </form>
          </SectionCard>
        </main>
      </PageBody>
    </Page>
  );
}

function modelOptionsForChannel(channel: OpenAIChannel | null): ModelOption[] {
  if (!channel) {
    return [];
  }
  return dedupeModelEntries(channel.models ?? []).map((entry) => {
    const value = effectiveAlias(entry);
    const upstream = entry.name.trim();
    return {
      label: upstream && upstream !== value ? `${value} -> ${upstream}` : value,
      value,
    };
  });
}

function dedupeModelEntries(models: ModelEntry[]) {
  const seen = new Set<string>();
  return models.filter((entry) => {
    const alias = effectiveAlias(entry);
    if (!alias || seen.has(alias)) {
      return false;
    }
    seen.add(alias);
    return true;
  });
}

function effectiveAlias(model: ModelEntry) {
  return (model.alias || model.name).trim();
}
