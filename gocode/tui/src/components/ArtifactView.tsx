import React, { type FC } from "react";
import { Box, Text } from "ink";
import type { UIArtifact } from "../hooks/useEvents.js";

interface ArtifactViewProps {
  artifacts: UIArtifact[];
}

const ArtifactView: FC<ArtifactViewProps> = ({ artifacts }) => {
  if (artifacts.length === 0) return null;

  return (
    <Box flexDirection="column" marginTop={1}>
      {artifacts.map((artifact, index) => (
        <Box
          key={artifact.id}
          flexDirection="column"
          borderStyle="round"
          borderColor="gray"
          paddingX={1}
          marginTop={index === 0 ? 0 : 1}
        >
          <Text bold>{artifact.title}</Text>
          <Text color="gray">{artifact.kind}</Text>
          <Box marginTop={1}>
            <Text>{previewArtifactContent(artifact.content)}</Text>
          </Box>
        </Box>
      ))}
    </Box>
  );
};

function previewArtifactContent(content: string): string {
  const normalized = content.trim();
  if (!normalized) {
    return "Artifact content is not available yet.";
  }

  const lines = normalized.split("\n");
  const previewLines = lines.slice(0, 18);
  const preview = previewLines.join("\n");
  if (previewLines.length === lines.length && preview.length <= 2500) {
    return preview;
  }

  return `${preview.slice(0, 2500)}\n\n[Artifact preview truncated]`;
}

export default ArtifactView;
