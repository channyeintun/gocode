import React, { type FC } from "react";
import { Box, Text } from "silvery";
import MarkdownText from "./MarkdownText.js";

interface PlanPanelProps {
  title: string;
  content: string;
  version?: number;
  source?: string;
  status?: string;
}

const PlanPanel: FC<PlanPanelProps> = ({
  title,
  content,
  version,
  source,
  status,
}) => {
  const body = content.trim();
  if (!body) return null;

  const metaParts: string[] = [];
  if (version && version > 0) metaParts.push(`v${version}`);
  if (source) metaParts.push(`src:${source}`);
  if (status) metaParts.push(`[${status}]`);
  const meta = metaParts.join("  ·  ");

  const statusColor =
    status === "final"
      ? "$success"
      : status === "draft"
        ? "$warning"
        : "$muted";

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="$border"
      paddingX={1}
      marginBottom={1}
      minWidth={0}
    >
      <Box flexDirection="column" minWidth={0}>
        <Text bold color="$primary">
          {title}
        </Text>
        {meta ? (
          <Text color={statusColor}>{meta}</Text>
        ) : (
          <Text color="$muted">{"Implementation Plan"}</Text>
        )}
      </Box>
      <Box marginTop={1} minWidth={0}>
        <MarkdownText text={body} />
      </Box>
    </Box>
  );
};

export default PlanPanel;
