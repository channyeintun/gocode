import React, { type FC, useEffect, useMemo, useState } from "react";
import { Box, Text } from "ink";
import {
  calculateTokenWarningState,
  formatTokenCount,
} from "../utils/modelContext.js";

interface PromptFooterProps {
  mode: string;
  model: string;
  maxContextWindow?: number | null;
  maxOutputTokens?: number | null;
  currentContextUsage?: number | null;
  isLoading: boolean;
  disabled?: boolean;
  promptValue: string;
  inputTokens: number;
  outputTokens: number;
}

const INPUT_HINT =
  "Enter send | Shift+Enter newline | Arrows move | Tab mode | Esc cancel";
const DISABLED_HINT = "Engine busy | Esc cancel";

const PromptFooter: FC<PromptFooterProps> = ({
  mode,
  model,
  maxContextWindow,
  maxOutputTokens,
  currentContextUsage,
  isLoading,
  disabled,
  promptValue,
  inputTokens,
  outputTokens,
}) => {
  const [terminalColumns, setTerminalColumns] = useState(
    process.stdout.columns ?? 80,
  );

  useEffect(() => {
    const handleResize = () => {
      setTerminalColumns(process.stdout.columns ?? 80);
    };

    handleResize();
    process.stdout.on("resize", handleResize);

    return () => {
      process.stdout.off("resize", handleResize);
    };
  }, []);

  const footerLayout = terminalColumns < 96 ? "column" : "row";
  const promptTextColumns = useMemo(
    () => getPromptTextColumns(terminalColumns),
    [terminalColumns],
  );
  const wrappedLineCount = useMemo(
    () => getWrappedLineSegments(promptValue, promptTextColumns).length,
    [promptTextColumns, promptValue],
  );
  const tokenUsage = currentContextUsage ?? inputTokens + outputTokens;
  const tokenWarning = useMemo(
    () =>
      calculateTokenWarningState(
        tokenUsage,
        model,
        maxContextWindow,
        maxOutputTokens,
      ),
    [maxContextWindow, maxOutputTokens, model, tokenUsage],
  );
  const showWrappedIndicator = promptValue.length > 0 && wrappedLineCount > 1;
  const activityLabel = isLoading ? "running" : disabled ? "blocked" : "ready";
  const hint = disabled ? DISABLED_HINT : INPUT_HINT;
  const warningText = tokenWarning.isWarning
    ? `Compact soon (~${tokenWarning.percentLeft}% until threshold) · ${formatTokenCount(tokenUsage)}/${formatTokenCount(tokenWarning.effectiveContextWindow)} used · Run /compact before the next long turn`
    : null;

  return (
    <Box flexDirection="column">
      {warningText ? (
        <Box paddingX={2} paddingTop={1}>
          <Text color={tokenWarning.isError ? "red" : "yellow"}>
            {warningText}
          </Text>
        </Box>
      ) : null}
      <Box
        paddingX={2}
        paddingTop={warningText ? 0 : 1}
        flexDirection={footerLayout}
        justifyContent="space-between"
      >
        <Text dimColor>
          <Text color={getModeColor(mode)} bold>
            {formatModeLabel(mode)}
          </Text>
          {"  "}
          <Text>{activityLabel}</Text>
          {showWrappedIndicator ? `  wrapped:${wrappedLineCount}` : ""}
        </Text>
        <Text dimColor>{hint}</Text>
      </Box>
    </Box>
  );
};

export default PromptFooter;

function formatModeLabel(mode: string): string {
  return mode === "plan" ? "PLAN" : mode.toUpperCase();
}

function getModeColor(mode: string): "blue" | "green" | "yellow" {
  if (mode === "plan") {
    return "blue";
  }

  if (mode === "fast") {
    return "green";
  }

  return "yellow";
}

function getPromptTextColumns(terminalColumns: number): number {
  return Math.max(8, terminalColumns - 7);
}

function getWrappedLineSegments(value: string, columns: number): string[] {
  const wrapWidth = Math.max(1, columns - 1);
  const logicalLines = value.split("\n");
  const segments: string[] = [];

  for (const line of logicalLines) {
    if (line.length === 0) {
      segments.push("");
      continue;
    }

    for (let offset = 0; offset < line.length; offset += wrapWidth) {
      segments.push(line.slice(offset, offset + wrapWidth));
    }
  }

  return segments;
}
