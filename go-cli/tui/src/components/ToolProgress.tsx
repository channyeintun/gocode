import React, { type FC } from "react";
import { Box, Text } from "ink";
import type { UIToolCall } from "../hooks/useEvents.js";
import MarkdownText from "./MarkdownText.js";

interface ToolProgressProps {
  toolCall: UIToolCall;
}

const STATUS_DOT = "●";
const RESPONSE_PREFIX = "  ⎿  ";

function summarizeInput(name: string, raw: string): string {
  try {
    const obj = JSON.parse(raw);
    if (name === "bash" && obj.command) return obj.command;
    if (
      (name === "file_read" || name === "file_write" || name === "file_edit") &&
      obj.file_path
    )
      return obj.file_path;
    if (name === "glob" && obj.pattern) return obj.pattern;
    if (name === "grep" && obj.pattern) return obj.pattern;
    if (name === "git" && obj.subcommand) return obj.subcommand;
    if (name === "web_search" && obj.query) return obj.query;
    if (name === "web_fetch" && obj.url) return obj.url;
  } catch {
    // ignore
  }
  return raw.length > 60 ? raw.slice(0, 57) + "..." : raw;
}

const ToolProgress: FC<ToolProgressProps> = ({ toolCall }) => {
  const summary = toolCall.input
    ? summarizeInput(toolCall.name, toolCall.input)
    : "";
  const headerColor =
    toolCall.status === "error"
      ? "red"
      : toolCall.status === "completed"
        ? "green"
        : undefined;
  const isDim =
    toolCall.status === "running" || toolCall.status === "waiting_permission";
  const response = renderResponse(toolCall);

  return (
    <Box flexDirection="column" marginBottom={1}>
      <Box flexDirection="row">
        <Box minWidth={2}>
          <Text color={headerColor} dimColor={isDim}>
            {STATUS_DOT}
          </Text>
        </Box>
        <Text color={headerColor} dimColor={isDim}>
          <Text bold>{toolCall.name}</Text>
          {summary ? ` (${summary})` : ""}
        </Text>
      </Box>
      {response ? (
        <Box flexDirection="row">
          <Text dimColor>{RESPONSE_PREFIX}</Text>
          <Box flexGrow={1}>{response}</Box>
        </Box>
      ) : null}
    </Box>
  );
};

export default ToolProgress;

function renderResponse(toolCall: UIToolCall) {
  if (toolCall.status === "waiting_permission") {
    return <Text dimColor>Waiting for permission…</Text>;
  }

  if (toolCall.status === "running") {
    return (
      <Text dimColor>
        Working…
        {toolCall.progressBytes !== undefined
          ? ` ${toolCall.progressBytes} bytes processed`
          : ""}
      </Text>
    );
  }

  if (toolCall.status === "error") {
    return (
      <Text color="red">
        {summarizeOutput(toolCall.error ?? "Tool failed")}
      </Text>
    );
  }

  if (!toolCall.output) {
    return <Text color="green">Completed.</Text>;
  }

  return (
    <MarkdownText text={summarizeOutput(toolCall.output, toolCall.truncated)} />
  );
}

function summarizeOutput(raw: string, truncated?: boolean): string {
  const trimmed = raw.trim();
  if (!trimmed) {
    return truncated ? "Completed. Output truncated." : "Completed.";
  }

  const lines = trimmed.split("\n");
  const clippedLines = lines.slice(0, 6);
  const clipped = clippedLines.join("\n");
  const shortened =
    clipped.length > 320 ? `${clipped.slice(0, 317)}...` : clipped;

  if (
    lines.length > clippedLines.length ||
    clipped.length < trimmed.length ||
    truncated
  ) {
    return `${shortened}\n\n_Output truncated._`;
  }

  return shortened;
}
