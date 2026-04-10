import path from "node:path";
import React, { type FC } from "react";
import { Box, Text } from "ink";
import {
  formatTokenCount,
  getApproxEffectiveContextWindow,
} from "../utils/modelContext.js";

interface StatusBarProps {
  ready: boolean;
  mode: string;
  model: string;
  sessionId?: string | null;
  totalCostUsd: number;
  inputTokens: number;
  outputTokens: number;
}

const StatusBar: FC<StatusBarProps> = ({
  ready,
  mode,
  model,
  sessionId,
  totalCostUsd,
  inputTokens,
  outputTokens,
}) => {
  const modeColor = mode === "plan" ? "blue" : "green";
  const readinessLabel = ready ? "READY" : "BOOTING";
  const readinessColor = ready ? "green" : "yellow";
  const workspaceLabel = path.basename(process.cwd());
  const sessionLabel = sessionId ? `session ${sessionId.slice(0, 8)}` : null;
  const modelLabel = formatModelLabel(model);
  const contextWindow = getApproxEffectiveContextWindow(model);
  const contextTokens = inputTokens + outputTokens;
  const contextPercent = Math.min(
    999,
    contextWindow > 0 ? Math.round((contextTokens / contextWindow) * 100) : 0,
  );
  const contextColor =
    contextPercent >= 90 ? "red" : contextPercent >= 70 ? "yellow" : "gray";

  return (
    <Box paddingX={1} paddingY={0}>
      <Text wrap="truncate-end">
        <Text color={readinessColor}>{readinessLabel.toLowerCase()}</Text>
        <Text color="gray"> · </Text>
        <Text bold>{workspaceLabel}</Text>
        {sessionLabel ? (
          <>
            <Text color="gray"> · </Text>
            <Text color="gray">{sessionLabel}</Text>
          </>
        ) : null}
        <Text color="gray"> · </Text>
        <Text color={modeColor}>{mode.toUpperCase()}</Text>
        <Text color="gray"> · </Text>
        <Text color="yellow">{modelLabel}</Text>
        <Text color="gray"> · </Text>
        <Text color={contextColor}>{`ctx ~${contextPercent}%`}</Text>
        <Text color="gray"> {formatTokenCount(contextTokens)}/</Text>
        <Text color="gray">{formatTokenCount(contextWindow)}</Text>
        <Text color="gray"> · </Text>
        <Text color="gray">{`${formatTokenCount(inputTokens)}↑ ${formatTokenCount(outputTokens)}↓`}</Text>
        <Text color="gray"> · </Text>
        <Text color="gray">{`$${totalCostUsd.toFixed(4)}`}</Text>
      </Text>
    </Box>
  );
};

export default StatusBar;

function formatModelLabel(model: string): string {
  const compact = model.trim();
  if (!compact) {
    return "Unknown model";
  }

  if (compact.startsWith("claude-")) {
    return compact
      .replace(/^claude-/, "Claude ")
      .replace(/-(\d{8}|latest)$/i, "")
      .replace(/-/g, " ");
  }

  if (compact.startsWith("gemini-")) {
    return compact.replace(/^gemini-/, "Gemini ").replace(/-/g, " ");
  }

  if (compact.startsWith("gpt-")) {
    return compact.toUpperCase();
  }

  return compact.replace(/-/g, " ");
}
