// StreamEvent types (Go → Ink)
export type EventType =
  | "token_delta"
  | "thinking_delta"
  | "turn_complete"
  | "tool_start"
  | "tool_progress"
  | "tool_result"
  | "tool_error"
  | "permission_request"
  | "mode_changed"
  | "model_changed"
  | "cost_update"
  | "compact_start"
  | "compact_end"
  | "artifact_created"
  | "artifact_updated"
  | "ready"
  | "error"
  | "session_restored";

export interface StreamEvent {
  type: EventType;
  payload?: unknown;
}

// ClientMessage types (Ink → Go)
export type ClientMessageType =
  | "user_input"
  | "slash_command"
  | "permission_response"
  | "cancel"
  | "mode_toggle"
  | "shutdown";

export interface ClientMessage {
  type: ClientMessageType;
  payload?: unknown;
}

// Typed payloads
export interface TokenDeltaPayload {
  text: string;
}

export interface TurnCompletePayload {
  stop_reason: string;
}

export interface ToolStartPayload {
  tool_id: string;
  name: string;
  input: string;
}

export interface ToolProgressPayload {
  tool_id: string;
  bytes_read: number;
}

export interface ToolResultPayload {
  tool_id: string;
  output: string;
  truncated: boolean;
  name?: string;
  input?: string;
  file_path?: string;
  preview?: string;
  insertions?: number;
  deletions?: number;
}

export interface ToolErrorPayload {
  tool_id: string;
  error: string;
  name?: string;
  input?: string;
}

export interface PermissionRequestPayload {
  request_id: string;
  tool_id: string;
  tool: string;
  command: string;
  risk: string;
}

export interface ModeChangedPayload {
  mode: string;
}

export interface ModelChangedPayload {
  model: string;
}

export interface CostUpdatePayload {
  total_usd: number;
  input_tokens: number;
  output_tokens: number;
}

export interface CompactStartPayload {
  strategy: string;
  tokens_before: number;
}

export interface CompactEndPayload {
  tokens_after: number;
}

export interface ArtifactCreatedPayload {
  id: string;
  kind: string;
  title: string;
}

export interface ArtifactUpdatedPayload {
  id: string;
  content: string;
}

export interface SessionRestoredPayload {
  session_id: string;
  mode: string;
}

export interface ReadyPayload {
  protocol_version: number;
}

export interface ErrorPayload {
  message: string;
  recoverable: boolean;
}
