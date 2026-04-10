import React, { type FC } from "react";
import { Box, Text } from "ink";
import type { UIToolCall } from "../hooks/useEvents.js";
import MarkdownText from "./MarkdownText.js";

interface ToolProgressProps {
  toolCall: UIToolCall;
}

const STATUS_DOT = "●";
const RESPONSE_PREFIX = "  ⎿  ";

interface ToolDescriptor {
  title: string;
  summary: string;
}

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
  const descriptor = describeTool(toolCall);
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
          <Text bold>{descriptor.title}</Text>
          {descriptor.summary ? ` (${descriptor.summary})` : ""}
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
    return <Text dimColor>{permissionLabel(toolCall)}</Text>;
  }

  if (toolCall.status === "running") {
    return <Text dimColor>{runningLabel(toolCall)}</Text>;
  }

  if (toolCall.status === "error") {
    return renderError(toolCall);
  }

  return renderSuccess(toolCall);
}

function describeTool(toolCall: UIToolCall): ToolDescriptor {
  switch (toolCall.name) {
    case "bash":
      return {
        title: "Bash",
        summary: summarizeInput(toolCall.name, toolCall.input),
      };
    case "file_read":
      return {
        title: "Read File",
        summary: basenameOrFallback(
          summarizeInput(toolCall.name, toolCall.input),
        ),
      };
    case "file_write":
      return {
        title: "Write File",
        summary: basenameOrFallback(
          summarizeInput(toolCall.name, toolCall.input),
        ),
      };
    case "file_edit":
      return {
        title: "Edit File",
        summary: basenameOrFallback(
          summarizeInput(toolCall.name, toolCall.input),
        ),
      };
    case "grep":
      return {
        title: "Search Files",
        summary: summarizeInput(toolCall.name, toolCall.input),
      };
    case "glob":
      return {
        title: "Find Files",
        summary: summarizeInput(toolCall.name, toolCall.input),
      };
    case "git":
      return { title: "Git", summary: summarizeGitInput(toolCall.input) };
    case "web_search":
      return {
        title: "Web Search",
        summary: summarizeInput(toolCall.name, toolCall.input),
      };
    case "web_fetch":
      return {
        title: "Fetch URL",
        summary: summarizeInput(toolCall.name, toolCall.input),
      };
    default:
      return {
        title: toolCall.name,
        summary: summarizeInput(toolCall.name, toolCall.input),
      };
  }
}

function permissionLabel(toolCall: UIToolCall): string {
  switch (toolCall.name) {
    case "bash":
      return "Waiting for permission to run command…";
    case "file_write":
    case "file_edit":
      return "Waiting for permission to modify file…";
    case "web_fetch":
      return "Waiting for permission to fetch URL…";
    default:
      return "Waiting for permission…";
  }
}

function runningLabel(toolCall: UIToolCall): string {
  const progressSuffix =
    toolCall.progressBytes !== undefined
      ? ` ${toolCall.progressBytes} bytes processed`
      : "";

  switch (toolCall.name) {
    case "bash":
      return `Running command…${progressSuffix}`;
    case "file_read":
      return `Reading file…${progressSuffix}`;
    case "file_write":
      return `Writing file…${progressSuffix}`;
    case "file_edit":
      return `Editing file…${progressSuffix}`;
    case "grep":
      return `Searching files…${progressSuffix}`;
    case "glob":
      return `Finding files…${progressSuffix}`;
    case "git":
      return `Running git command…${progressSuffix}`;
    case "web_search":
      return `Searching the web…${progressSuffix}`;
    case "web_fetch":
      return `Fetching page content…${progressSuffix}`;
    default:
      return `Working…${progressSuffix}`;
  }
}

function renderError(toolCall: UIToolCall) {
  if (toolCall.name === "file_write" || toolCall.name === "file_edit") {
    return (
      <Text color="red">
        File update failed: {summarizeOutput(toolCall.error ?? "Tool failed")}
      </Text>
    );
  }

  if (toolCall.name === "bash") {
    return (
      <MarkdownText
        text={summarizeOutput(toolCall.error ?? "Command failed")}
      />
    );
  }

  return (
    <Text color="red">{summarizeOutput(toolCall.error ?? "Tool failed")}</Text>
  );
}

function renderSuccess(toolCall: UIToolCall) {
  switch (toolCall.name) {
    case "file_write":
    case "file_edit":
      return <MarkdownText text={summarizeFileMutation(toolCall)} />;
    case "file_read":
      return (
        <MarkdownText
          text={summarizeFileRead(toolCall.output, toolCall.truncated)}
        />
      );
    case "grep":
      return (
        <MarkdownText
          text={summarizeSearchMatches(
            toolCall.output,
            toolCall.truncated,
            "match",
          )}
        />
      );
    case "glob":
      return (
        <MarkdownText
          text={summarizeSearchMatches(
            toolCall.output,
            toolCall.truncated,
            "file",
          )}
        />
      );
    case "web_search":
      return <MarkdownText text={summarizeWebSearch(toolCall.output)} />;
    case "web_fetch":
      return (
        <MarkdownText
          text={summarizeWebFetch(toolCall.output, toolCall.truncated)}
        />
      );
    case "git":
      return (
        <MarkdownText
          text={summarizeGitOutput(toolCall.output, toolCall.truncated)}
        />
      );
    case "bash":
      return (
        <MarkdownText
          text={summarizeShellOutput(toolCall.output, toolCall.truncated)}
        />
      );
    default:
      if (!toolCall.output) {
        return <Text color="green">Completed.</Text>;
      }
      return (
        <MarkdownText
          text={summarizeOutput(toolCall.output, toolCall.truncated)}
        />
      );
  }
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

function summarizeFileMutation(toolCall: UIToolCall): string {
  const parts: string[] = [];

  if (toolCall.output) {
    parts.push(summarizeOutput(toolCall.output, toolCall.truncated));
  } else {
    parts.push(
      toolCall.truncated ? "Completed. Output truncated." : "Completed.",
    );
  }

  const statLine = formatMutationStats(toolCall.insertions, toolCall.deletions);
  if (statLine) {
    parts.push(statLine);
  }

  if (toolCall.preview) {
    parts.push(["```diff", toolCall.preview, "```"].join("\n"));
  }

  return parts.join("\n\n");
}

function summarizeFileRead(raw?: string, truncated?: boolean): string {
  if (!raw) {
    return truncated ? "Read completed. Output truncated." : "Read completed.";
  }
  return summarizeOutput(raw, truncated);
}

function summarizeSearchMatches(
  raw?: string,
  truncated?: boolean,
  noun?: string,
): string {
  if (!raw) {
    return truncated
      ? `Results truncated for ${noun ?? "result"} search.`
      : "No output.";
  }
  const trimmed = raw.trim();
  if (trimmed === "No matches found" || trimmed === "No files found") {
    return trimmed;
  }

  const lines = trimmed.split("\n").filter(Boolean);
  const preview = lines.slice(0, 8).join("\n");
  const count = lines.filter((line) => !line.startsWith("(")).length;
  const suffix =
    count > 0 ? `Found ${count} ${count === 1 ? noun : `${noun}s`}.\n\n` : "";
  return `${suffix}${summarizeOutput(preview, truncated || lines.length > 8)}`;
}

function summarizeWebSearch(raw?: string): string {
  if (!raw) {
    return "Search completed.";
  }
  const lines = raw.trim().split("\n");
  const preview = lines.slice(0, 10).join("\n");
  return summarizeOutput(preview, lines.length > 10);
}

function summarizeWebFetch(raw?: string, truncated?: boolean): string {
  if (!raw) {
    return truncated
      ? "Fetch completed. Output truncated."
      : "Fetch completed.";
  }
  const lines = raw.trim().split("\n");
  const preview = lines.slice(0, 14).join("\n");
  return summarizeOutput(preview, truncated || lines.length > 14);
}

function summarizeGitOutput(raw?: string, truncated?: boolean): string {
  if (!raw) {
    return truncated
      ? "Git command completed. Output truncated."
      : "Git command completed.";
  }
  return summarizeOutput(raw, truncated);
}

function summarizeShellOutput(raw?: string, truncated?: boolean): string {
  if (!raw) {
    return truncated
      ? "Command completed. Output truncated."
      : "Command completed with no output.";
  }
  return summarizeOutput(raw, truncated);
}

function summarizeGitInput(raw: string): string {
  try {
    const obj = JSON.parse(raw) as {
      operation?: string;
      revision?: string;
      file_path?: string;
    };
    const parts = [obj.operation, obj.revision, obj.file_path].filter(
      (value): value is string => typeof value === "string" && value.length > 0,
    );
    return parts.join(" ");
  } catch {
    return summarizeInput("git", raw);
  }
}

function basenameOrFallback(value: string): string {
  if (!value) {
    return value;
  }
  const parts = value.split("/");
  return parts[parts.length - 1] || value;
}

function formatMutationStats(insertions?: number, deletions?: number): string {
  const additions = insertions ?? 0;
  const removals = deletions ?? 0;
  const parts: string[] = [];
  if (additions > 0) {
    parts.push(`Added ${additions} ${additions === 1 ? "line" : "lines"}`);
  }
  if (removals > 0) {
    parts.push(`Removed ${removals} ${removals === 1 ? "line" : "lines"}`);
  }
  return parts.join(", ");
}
