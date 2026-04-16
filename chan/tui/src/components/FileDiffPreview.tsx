import React, { type FC } from "react";
import { Text } from "silvery";

interface FileDiffPreviewProps {
  filePath?: string;
  preview?: string;
  insertions?: number;
  deletions?: number;
}

const FileDiffPreview: FC<FileDiffPreviewProps> = ({
  filePath,
  insertions,
  deletions,
}) => {
  const statLine = formatMutationStats(insertions, deletions);
  const summary = buildSummary(filePath, statLine);

  return <Text color="$success">{summary}</Text>;
};

export default FileDiffPreview;

function buildSummary(filePath: string | undefined, statLine: string): string {
  const target = filePath?.trim() || "file";
  if (!statLine) {
    return `Updated ${target}.`;
  }
  return `Updated ${target}. ${statLine}.`;
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
