import React, { type FC, useState } from "react";
import { Box, ListView, Text, useBoxRect, useInput } from "silvery";
import type { UIRewindSelection } from "../hooks/useEvents.js";

interface RewindSelectionPromptProps {
  selection: UIRewindSelection;
  onSelect: (messageIndex: number) => void;
  onCancel: () => void;
}

const RewindSelectionPrompt: FC<RewindSelectionPromptProps> = ({
  selection,
  onSelect,
  onCancel,
}) => {
  const [selectedIndex, setSelectedIndex] = useState(
    Math.max(selection.turns.length - 1, 0),
  );

  useInput((input, key) => {
    if (key.escape) {
      onCancel();
      return;
    }

    const shortcut = input?.toLowerCase();
    if (!shortcut) {
      return;
    }

    if (shortcut === "q") {
      onCancel();
    }
  });

  const handleListSelect = (index: number) => {
    const selected = selection.turns[index];
    if (selected) {
      onSelect(selected.messageIndex);
    }
  };

  return (
    <Box
      flexDirection="column"
      flexGrow={1}
      flexShrink={1}
      minWidth={0}
      minHeight={0}
      backgroundColor="$popover-bg"
      borderStyle="double"
      borderColor="$inputborder"
      overflow="hidden"
      paddingX={2}
      paddingY={1}
    >
      <Box flexDirection="column" flexShrink={0} minWidth={0}>
        <Text bold color="$warning">
          Rewind Conversation
        </Text>
        <Box marginTop={1} flexDirection="column" minWidth={0}>
          <Text>
            Choose the user turn to keep. Later messages will be dropped.
          </Text>
          <Text color="$muted">
            {selection.turns.length} available turn
            {selection.turns.length === 1 ? "" : "s"}
          </Text>
        </Box>
      </Box>

      <RewindTurnList
        turns={selection.turns}
        selectedIndex={selectedIndex}
        onCursor={setSelectedIndex}
        onSelectIndex={handleListSelect}
      />
      <Box marginTop={1} flexDirection="column" flexShrink={0}>
        <Text color="$fg">
          <Text color="$primary" bold>
            Enter
          </Text>{" "}
          rewind ·{" "}
          <Text color="$primary" bold>
            Up/Down
          </Text>{" "}
          change selection ·{" "}
          <Text color="$primary" bold>
            Esc
          </Text>{" "}
          or{" "}
          <Text color="$primary" bold>
            Q
          </Text>{" "}
          cancel
        </Text>
      </Box>
    </Box>
  );
};

export default RewindSelectionPrompt;

interface RewindTurnListProps {
  turns: UIRewindSelection["turns"];
  selectedIndex: number;
  onCursor: (index: number) => void;
  onSelectIndex: (index: number) => void;
}

const RewindTurnList: FC<RewindTurnListProps> = ({
  turns,
  selectedIndex,
  onCursor,
  onSelectIndex,
}) => {
  const { height: rectHeight } = useBoxRect();
  const viewportHeight = Math.max(1, rectHeight);

  return (
    <Box
      marginTop={1}
      flexDirection="column"
      flexGrow={1}
      flexShrink={1}
      minHeight={0}
      minWidth={0}
      overflow="hidden"
    >
      <ListView
        items={turns}
        height={viewportHeight}
        nav
        cursorKey={selectedIndex}
        onCursor={onCursor}
        onSelect={onSelectIndex}
        active
        estimateHeight={2}
        overflowIndicator
        getKey={(turn) => `${turn.turnNumber}-${turn.messageIndex}`}
        renderItem={(turn, _index, meta) => {
          const isSelected = meta.isCursor;

          return (
            <Box
              key={`${turn.turnNumber}-${turn.messageIndex}`}
              flexDirection="column"
              backgroundColor={isSelected ? "$selectionbg" : undefined}
              paddingX={1}
              marginBottom={1}
              minWidth={0}
            >
              <Text color={isSelected ? "$selection" : "$fg"} bold={isSelected}>
                {isSelected ? "›" : " "} Turn {turn.turnNumber}
              </Text>
              <Text color={isSelected ? "$selection" : "$muted"}>
                {turn.preview}
              </Text>
            </Box>
          );
        }}
      />
    </Box>
  );
};
