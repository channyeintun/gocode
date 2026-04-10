import path from "node:path";
import React, { type FC } from "react";
import { Box, Text } from "ink";
import {
  formatTokenCount,
  getEffectiveContextWindow,
} from "../utils/modelContext.js";
import type { UIRateLimits } from "../hooks/useEvents.js";

interface StatusBarProps {
  ready: boolean;
  mode: string;
  model: string;
  sessionId?: string | null;
  sessionTitle?: string | null;
  maxContextWindow?: number | null;
  maxOutputTokens?: number | null;
  currentContextUsage?: number | null;
  totalCostUsd: number;
  inputTokens: number;
  outputTokens: number;
  rateLimits: UIRateLimits;
}

const StatusBar: FC<StatusBarProps> = ({
  ready,
  mode,
  model,
  sessionId,
  sessionTitle,
  maxContextWindow,
  maxOutputTokens,
  currentContextUsage,
  totalCostUsd,
  inputTokens,
  outputTokens,
  rateLimits,
}) => {
  const modeColor = mode === "plan" ? "blue" : "green";
  const readinessLabel = ready ? "READY" : "BOOTING";
  const readinessColor = ready ? "green" : "yellow";
  const workspaceLabel = path.basename(process.cwd());
  const sessionLabel = sessionTitle?.trim()
    ? sessionTitle.trim()
    : sessionId
      ? `session ${sessionId.slice(0, 8)}`
      : null;
  const modelLabel = formatModelLabel(model);
  const contextWindow = getEffectiveContextWindow(
    model,
    maxContextWindow,
    maxOutputTokens,
  );
  const contextTokens = currentContextUsage ?? inputTokens + outputTokens;
  const contextPercent = Math.min(
    999,
    contextWindow > 0 ? Math.round((contextTokens / contextWindow) * 100) : 0,
  );
  const contextColor =
    contextPercent >= 90 ? "red" : contextPercent >= 70 ? "yellow" : "gray";
  const hasRateLimits = !!rateLimits.fiveHour || !!rateLimits.sevenDay;

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
        {hasRateLimits ? (
          <>
            <Text color="gray"> · </Text>
            {rateLimits.fiveHour ? (
              <>
                <Text
                  color={rateLimitColor(rateLimits.fiveHour.usedPercentage)}
                >
                  {formatRateLimitWindow(
                    "5h",
                    rateLimits.fiveHour.usedPercentage,
                  )}
                </Text>
                {rateLimits.sevenDay ? <Text color="gray"> </Text> : null}
              </>
            ) : null}
            {rateLimits.sevenDay ? (
              <Text color={rateLimitColor(rateLimits.sevenDay.usedPercentage)}>
                {formatRateLimitWindow(
                  "7d",
                  rateLimits.sevenDay.usedPercentage,
                )}
              </Text>
            ) : null}
          </>
        ) : null}
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

function formatRateLimitWindow(label: string, usedPercentage: number): string {
  const rounded = Math.max(0, Math.min(999, Math.round(usedPercentage)));
  return `${label} ${rounded}%`;
}

function rateLimitColor(usedPercentage: number): "gray" | "yellow" | "red" {
  if (usedPercentage >= 90) {
    return "red";
  }
  if (usedPercentage >= 70) {
    return "yellow";
  }
  return "gray";
}
