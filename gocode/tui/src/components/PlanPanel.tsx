import React, { type FC } from "react";
import { Box, Text } from "ink";

interface PlanPanelProps {
  title: string;
  content: string;
}

const PlanPanel: FC<PlanPanelProps> = ({ title, content }) => {
  const body = content.trim();
  if (!body) return null;

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="blue"
      paddingX={1}
      marginBottom={1}
    >
      <Text bold color="blue">
        {title}
      </Text>
      <Text color="gray">{"Markdown artifact"}</Text>
      <Box marginTop={1}>
        <Text>{body}</Text>
      </Box>
    </Box>
  );
};

export default PlanPanel;
