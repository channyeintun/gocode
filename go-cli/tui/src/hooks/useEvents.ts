import { useState, useCallback } from "react";
import type { StreamEvent, TokenDeltaPayload, ModeChangedPayload, ModelChangedPayload, CostUpdatePayload, ToolStartPayload, ToolResultPayload, PermissionRequestPayload, ErrorPayload, SessionRestoredPayload } from "../protocol/types.js";

export interface EngineUIState {
  streamedText: string;
  mode: string;
  model: string;
  cost: { totalUsd: number; inputTokens: number; outputTokens: number };
  activeTool: { id: string; name: string } | null;
  pendingPermission: PermissionRequestPayload | null;
  error: string | null;
  isStreaming: boolean;
}

const initialState = (model: string): EngineUIState => ({
  streamedText: "",
  mode: "plan",
  model,
  cost: { totalUsd: 0, inputTokens: 0, outputTokens: 0 },
  activeTool: null,
  pendingPermission: null,
  error: null,
  isStreaming: false,
});

export function useEvents(initialModel: string) {
  const [uiState, setUIState] = useState<EngineUIState>(() => initialState(initialModel));

  const handleEvent = useCallback((event: StreamEvent) => {
    switch (event.type) {
      case "token_delta": {
        const p = event.payload as TokenDeltaPayload;
        setUIState((s) => ({ ...s, streamedText: s.streamedText + p.text, isStreaming: true }));
        break;
      }
      case "turn_complete":
        setUIState((s) => ({ ...s, isStreaming: false, activeTool: null }));
        break;
      case "tool_start": {
        const p = event.payload as ToolStartPayload;
        setUIState((s) => ({ ...s, activeTool: { id: p.tool_id, name: p.name } }));
        break;
      }
      case "tool_result":
      case "tool_error":
        setUIState((s) => ({ ...s, activeTool: null }));
        break;
      case "permission_request": {
        const p = event.payload as PermissionRequestPayload;
        setUIState((s) => ({ ...s, pendingPermission: p }));
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
          cost: { totalUsd: p.total_usd, inputTokens: p.input_tokens, outputTokens: p.output_tokens },
        }));
        break;
      }
      case "session_restored": {
        const p = event.payload as SessionRestoredPayload;
        setUIState((s) => ({ ...s, mode: p.mode, isStreaming: false, error: null }));
        break;
      }
      case "error": {
        const p = event.payload as ErrorPayload;
        setUIState((s) => ({ ...s, error: p.message }));
        break;
      }
    }
  }, []);

  const clearStream = useCallback(() => {
    setUIState((s) => ({ ...s, streamedText: "", error: null }));
  }, []);

  const clearPermission = useCallback(() => {
    setUIState((s) => ({ ...s, pendingPermission: null }));
  }, []);

  return { uiState, handleEvent, clearStream, clearPermission };
}
