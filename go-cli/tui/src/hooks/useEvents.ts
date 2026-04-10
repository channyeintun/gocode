import { useState, useCallback } from "react";
import type {
  ArtifactCreatedPayload,
  ArtifactUpdatedPayload,
  CompactEndPayload,
  CompactStartPayload,
  CostUpdatePayload,
  ErrorPayload,
  ModeChangedPayload,
  ModelChangedPayload,
  PermissionRequestPayload,
  ReadyPayload,
  SessionRestoredPayload,
  StreamEvent,
  TurnCompletePayload,
  TokenDeltaPayload,
  ToolErrorPayload,
  ToolProgressPayload,
  ToolResultPayload,
  ToolStartPayload,
} from "../protocol/types.js";

export interface UIArtifact {
  id: string;
  kind: string;
  title: string;
  content: string;
}

export interface UIMessage {
  id: string;
  role: "user" | "assistant";
  text: string;
}

export interface UITranscriptEntry {
  id: string;
  kind: "message" | "tool_call";
}

export type UIToolStatus =
  | "running"
  | "waiting_permission"
  | "completed"
  | "error";

export interface UIToolCall {
  id: string;
  name: string;
  input: string;
  status: UIToolStatus;
  output?: string;
  error?: string;
  truncated?: boolean;
  progressBytes?: number;
  permissionRequestId?: string;
  filePath?: string;
  preview?: string;
  insertions?: number;
  deletions?: number;
}

export interface EngineUIState {
  ready: boolean;
  messages: UIMessage[];
  transcript: UITranscriptEntry[];
  streamedText: string;
  thinkingText: string;
  mode: string;
  model: string;
  cost: { totalUsd: number; inputTokens: number; outputTokens: number };
  artifacts: UIArtifact[];
  toolCalls: UIToolCall[];
  compact: {
    active: boolean;
    strategy: string;
    tokensBefore: number;
    tokensAfter?: number;
  } | null;
  statusLine: string | null;
  pendingPermission: PermissionRequestPayload | null;
  error: string | null;
  isStreaming: boolean;
}

const initialState = (model: string, mode: string): EngineUIState => ({
  ready: false,
  messages: [],
  transcript: [],
  streamedText: "",
  thinkingText: "",
  mode,
  model,
  cost: { totalUsd: 0, inputTokens: 0, outputTokens: 0 },
  artifacts: [],
  toolCalls: [],
  compact: null,
  statusLine: null,
  pendingPermission: null,
  error: null,
  isStreaming: false,
});

let nextMessageId = 0;

function createMessage(role: UIMessage["role"], text: string): UIMessage {
  nextMessageId += 1;
  return {
    id: `msg-${nextMessageId}`,
    role,
    text,
  };
}

export function useEvents(initialModel: string, initialMode: string) {
  const [uiState, setUIState] = useState<EngineUIState>(() =>
    initialState(initialModel, initialMode),
  );

  const handleEvent = useCallback((event: StreamEvent) => {
    switch (event.type) {
      case "ready": {
        const p = event.payload as ReadyPayload;
        setUIState((s) => ({
          ...s,
          ready: p.protocol_version > 0,
          statusLine: `Engine ready (protocol v${p.protocol_version})`,
        }));
        break;
      }
      case "token_delta": {
        const p = event.payload as TokenDeltaPayload;
        setUIState((s) => ({
          ...s,
          streamedText: s.streamedText + p.text,
          isStreaming: true,
          statusLine: null,
        }));
        break;
      }
      case "thinking_delta": {
        const p = event.payload as TokenDeltaPayload;
        setUIState((s) => ({
          ...s,
          thinkingText: s.thinkingText + p.text,
          isStreaming: true,
        }));
        break;
      }
      case "turn_complete": {
        const p = event.payload as TurnCompletePayload;
        setUIState((s) => {
          const text = s.streamedText.trim();
          const message =
            text.length > 0
              ? createMessage("assistant", text)
              : createMessage(
                  "assistant",
                  "(Model returned an empty response)",
                );
          return {
            ...s,
            messages: [...s.messages, message],
            transcript: appendTranscriptEntry(s.transcript, {
              id: message.id,
              kind: "message",
            }),
            streamedText: "",
            thinkingText: "",
            isStreaming: false,
            compact: null,
            statusLine: `Turn complete (${p.stop_reason})`,
          };
        });
        break;
      }
      case "tool_start": {
        const p = event.payload as ToolStartPayload;
        setUIState((s) => ({
          ...s,
          transcript: appendTranscriptEntry(s.transcript, {
            id: p.tool_id,
            kind: "tool_call",
          }),
          toolCalls: upsertToolCall(s.toolCalls, {
            id: p.tool_id,
            name: p.name,
            input: p.input,
            status: "running",
            output: undefined,
            error: undefined,
            truncated: false,
            progressBytes: undefined,
            permissionRequestId: undefined,
          }),
        }));
        break;
      }
      case "tool_progress": {
        const p = event.payload as ToolProgressPayload;
        setUIState((s) => ({
          ...s,
          transcript: appendTranscriptEntry(s.transcript, {
            id: p.tool_id,
            kind: "tool_call",
          }),
          toolCalls: upsertToolCall(s.toolCalls, {
            id: p.tool_id,
            name: toolCallName(s.toolCalls, p.tool_id),
            input: toolCallInput(s.toolCalls, p.tool_id),
            status: "running",
            progressBytes: p.bytes_read,
          }),
        }));
        break;
      }
      case "tool_result": {
        const p = event.payload as ToolResultPayload;
        setUIState((s) => ({
          ...s,
          transcript: appendTranscriptEntry(s.transcript, {
            id: p.tool_id,
            kind: "tool_call",
          }),
          toolCalls: upsertToolCall(s.toolCalls, {
            id: p.tool_id,
            name: p.name ?? toolCallName(s.toolCalls, p.tool_id),
            input: p.input ?? toolCallInput(s.toolCalls, p.tool_id),
            status: "completed",
            output: p.output,
            truncated: p.truncated,
            error: undefined,
            permissionRequestId: undefined,
            filePath: p.file_path,
            preview: p.preview,
            insertions: p.insertions,
            deletions: p.deletions,
          }),
        }));
        break;
      }
      case "tool_error": {
        const p = event.payload as ToolErrorPayload;
        setUIState((s) => ({
          ...s,
          transcript: appendTranscriptEntry(s.transcript, {
            id: p.tool_id,
            kind: "tool_call",
          }),
          toolCalls: upsertToolCall(s.toolCalls, {
            id: p.tool_id,
            name: p.name ?? toolCallName(s.toolCalls, p.tool_id),
            input: p.input ?? toolCallInput(s.toolCalls, p.tool_id),
            status: "error",
            error: p.error,
            permissionRequestId: undefined,
          }),
        }));
        break;
      }
      case "compact_start": {
        const p = event.payload as CompactStartPayload;
        setUIState((s) => ({
          ...s,
          compact: {
            active: true,
            strategy: p.strategy,
            tokensBefore: p.tokens_before,
          },
          statusLine: null,
        }));
        break;
      }
      case "compact_end": {
        const p = event.payload as CompactEndPayload;
        setUIState((s) => ({
          ...s,
          compact: s.compact
            ? {
                ...s.compact,
                active: false,
                tokensAfter: p.tokens_after,
              }
            : {
                active: false,
                strategy: "compact",
                tokensBefore: 0,
                tokensAfter: p.tokens_after,
              },
          statusLine: `Compaction complete (${p.tokens_after} tokens)`,
        }));
        break;
      }
      case "permission_request": {
        const p = event.payload as PermissionRequestPayload;
        setUIState((s) => ({
          ...s,
          pendingPermission: p,
          transcript: appendTranscriptEntry(s.transcript, {
            id: p.tool_id,
            kind: "tool_call",
          }),
          toolCalls: upsertToolCall(s.toolCalls, {
            id: p.tool_id,
            name: p.tool,
            input: p.command,
            status: "waiting_permission",
            permissionRequestId: p.request_id,
          }),
        }));
        break;
      }
      case "mode_changed": {
        const p = event.payload as ModeChangedPayload;
        setUIState((s) => ({ ...s, mode: p.mode }));
        break;
      }
      case "model_changed": {
        const p = event.payload as ModelChangedPayload;
        setUIState((s) => ({ ...s, model: p.model }));
        break;
      }
      case "cost_update": {
        const p = event.payload as CostUpdatePayload;
        setUIState((s) => ({
          ...s,
          cost: {
            totalUsd: p.total_usd,
            inputTokens: p.input_tokens,
            outputTokens: p.output_tokens,
          },
        }));
        break;
      }
      case "artifact_created": {
        const p = event.payload as ArtifactCreatedPayload;
        setUIState((s) => ({
          ...s,
          artifacts: upsertArtifact(s.artifacts, {
            id: p.id,
            kind: p.kind,
            title: p.title,
            content: "",
          }),
        }));
        break;
      }
      case "artifact_updated": {
        const p = event.payload as ArtifactUpdatedPayload;
        setUIState((s) => {
          const existing = s.artifacts.find((a) => a.id === p.id);
          if (!existing) {
            // Ignore updates for artifacts that haven't been created yet
            return s;
          }
          return {
            ...s,
            artifacts: upsertArtifact(s.artifacts, {
              id: p.id,
              kind: existing.kind,
              title: existing.title,
              content: p.content,
            }),
          };
        });
        break;
      }
      case "session_restored": {
        const p = event.payload as SessionRestoredPayload;
        setUIState((s) => ({
          ...s,
          ready: true,
          mode: p.mode,
          isStreaming: false,
          error: null,
          statusLine: `Resumed session ${p.session_id}`,
        }));
        break;
      }
      case "error": {
        const p = event.payload as ErrorPayload;
        setUIState((s) => ({
          ...s,
          error: p.message,
          isStreaming: false,
          compact: null,
        }));
        break;
      }
    }
  }, []);

  const clearStream = useCallback(() => {
    setUIState((s) => ({
      ...s,
      streamedText: "",
      thinkingText: "",
      compact: null,
      statusLine: null,
      error: null,
    }));
  }, []);

  const clearPermission = useCallback((decision?: string) => {
    setUIState((s) => ({
      ...s,
      pendingPermission: null,
      toolCalls: s.pendingPermission
        ? upsertToolCall(s.toolCalls, {
            id: s.pendingPermission.tool_id,
            name: s.pendingPermission.tool,
            input: s.pendingPermission.command,
            status:
              decision === "allow" ||
              decision === "always_allow" ||
              decision === "allow_all_session"
                ? "running"
                : "waiting_permission",
            permissionRequestId: undefined,
          })
        : s.toolCalls,
    }));
  }, []);

  const appendUserMessage = useCallback((text: string) => {
    setUIState((s) => {
      const message = createMessage("user", text);
      return {
        ...s,
        messages: [...s.messages, message],
        transcript: appendTranscriptEntry(s.transcript, {
          id: message.id,
          kind: "message",
        }),
      };
    });
  }, []);

  const beginAssistantTurn = useCallback(() => {
    setUIState((s) => ({
      ...s,
      streamedText: "",
      thinkingText: "",
      error: null,
      statusLine: null,
      isStreaming: true,
    }));
  }, []);

  return {
    uiState,
    handleEvent,
    clearStream,
    clearPermission,
    appendUserMessage,
    beginAssistantTurn,
  };
}

function upsertArtifact(
  artifacts: UIArtifact[],
  nextArtifact: UIArtifact,
): UIArtifact[] {
  const remaining = artifacts.filter(
    (artifact) => artifact.id !== nextArtifact.id,
  );
  return [nextArtifact, ...remaining];
}

function appendTranscriptEntry(
  transcript: UITranscriptEntry[],
  entry: UITranscriptEntry,
): UITranscriptEntry[] {
  if (
    transcript.some(
      (current) => current.id === entry.id && current.kind === entry.kind,
    )
  ) {
    return transcript;
  }
  return [...transcript, entry];
}

function upsertToolCall(
  toolCalls: UIToolCall[],
  nextToolCall: UIToolCall,
): UIToolCall[] {
  const existing = toolCalls.find(
    (toolCall) => toolCall.id === nextToolCall.id,
  );
  if (!existing) {
    return [...toolCalls, nextToolCall];
  }

  return toolCalls.map((toolCall) =>
    toolCall.id === nextToolCall.id
      ? {
          ...toolCall,
          ...nextToolCall,
          name: nextToolCall.name || toolCall.name,
          input: nextToolCall.input || toolCall.input,
          output:
            nextToolCall.output !== undefined
              ? nextToolCall.output
              : toolCall.output,
          error:
            nextToolCall.error !== undefined
              ? nextToolCall.error
              : toolCall.error,
          truncated:
            nextToolCall.truncated !== undefined
              ? nextToolCall.truncated
              : toolCall.truncated,
          progressBytes:
            nextToolCall.progressBytes !== undefined
              ? nextToolCall.progressBytes
              : toolCall.progressBytes,
          permissionRequestId:
            nextToolCall.permissionRequestId !== undefined
              ? nextToolCall.permissionRequestId
              : toolCall.permissionRequestId,
          filePath:
            nextToolCall.filePath !== undefined
              ? nextToolCall.filePath
              : toolCall.filePath,
          preview:
            nextToolCall.preview !== undefined
              ? nextToolCall.preview
              : toolCall.preview,
          insertions:
            nextToolCall.insertions !== undefined
              ? nextToolCall.insertions
              : toolCall.insertions,
          deletions:
            nextToolCall.deletions !== undefined
              ? nextToolCall.deletions
              : toolCall.deletions,
        }
      : toolCall,
  );
}

function toolCallName(toolCalls: UIToolCall[], id: string): string {
  return toolCalls.find((toolCall) => toolCall.id === id)?.name ?? "tool";
}

function toolCallInput(toolCalls: UIToolCall[], id: string): string {
  return toolCalls.find((toolCall) => toolCall.id === id)?.input ?? "";
}

function findArtifactField(
  artifacts: UIArtifact[],
  id: string,
  field: "kind" | "title",
  fallback: string,
): string {
  const artifact = artifacts.find((entry) => entry.id === id);
  return artifact?.[field] ?? fallback;
}
