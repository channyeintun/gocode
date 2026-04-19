import React, { type FC } from "react";
import { Box, Text } from "silvery";
import type { UISlashCommand } from "../hooks/useEvents.js";

interface SlashCommandPreviewProps {
  commands: UISlashCommand[];
  selectedIndex: number;
}

const SlashCommandPreview: FC<SlashCommandPreviewProps> = ({
  commands,
  selectedIndex,
}) => {
  const startIndex = Math.max(0, Math.min(selectedIndex, commands.length - 6));
  const visibleCommands = commands.slice(startIndex, startIndex + 6);

  return (
    <Box flexDirection="column" marginTop={1} minWidth={0}>
      {visibleCommands.map((command, index) => {
        const actualIndex = startIndex + index;
        const selected = actualIndex === selectedIndex;

        return (
          <Box
            key={command.name}
            flexDirection="row"
            paddingLeft={1}
            width="100%"
            minWidth={0}
          >
            <Text color={selected ? "$primary" : "$muted"}>
              {selected ? "›" : " "}
            </Text>
            <Text color={selected ? "$primary" : undefined} bold>
              {` /${command.name}`}
            </Text>
            <Box flexGrow={1} minWidth={0}>
              <Text color="$muted" wrap="truncate-end">
                {`  ${command.description}`}
              </Text>
            </Box>
          </Box>
        );
      })}
    </Box>
  );
};

export default SlashCommandPreview;
