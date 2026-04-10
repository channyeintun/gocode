import React, { type FC } from "react";
import { Box, Text } from "ink";
import Spinner from "ink-spinner";

interface ToolProgressProps {
  toolName: string;
  toolInput?: string;
}

function summarizeInput(name: string, raw: string): string {
  try {
    const obj = JSON.parse(raw);
    if (name === "bash" && obj.command) return obj.command;
    if ((name === "file_read" || name === "file_write" || name === "file_edit") && obj.file_path) return obj.file_path;
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

const ToolProgress: FC<ToolProgressProps> = ({ toolName, toolInput }) => {
  const summary = toolInput ? summarizeInput(toolName, toolInput) : "";
  return (
    <Box paddingLeft={1}>
      <Text color="gray">
        <Spinner type="dots" /> {"Tool: "}
        <Text bold>{toolName}</Text>
        {summary ? <Text color="cyan">{" "}{summary}</Text> : null}
      </Text>
    </Box>
  );
};

export default ToolProgress;
