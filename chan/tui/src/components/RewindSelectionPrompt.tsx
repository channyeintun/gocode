import React, { type FC, useMemo, useState } from "react";
import { Box, Text, useInput } from "silvery";
import type { UIRewindSelection } from "../hooks/useEvents.js";

interface RewindSelectionPromptProps {
  selection: UIRewindSelection;
  onSelect: (messageIndex: number) => void;
  onCancel: () => void;
}

const VISIBLE_WINDOW = 8;

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

    if (key.upArrow) {
      setSelectedIndex((current) =>
        current === 0 ? selection.turns.length - 1 : current - 1,
      );
      return;
    }

    if (key.downArrow) {
      setSelectedIndex((current) => (current + 1) % selection.turns.length);
      return;
    }

    if (key.return) {
      const selected = selection.turns[selectedIndex];
      if (selected) {
        onSelect(selected.messageIndex);
      }
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

  const startIndex = useMemo(() => {
    if (selection.turns.length <= VISIBLE_WINDOW) {
      return 0;
    }
    const centered = selectedIndex - Math.floor(VISIBLE_WINDOW / 2);
    return Math.max(
      0,
      Math.min(centered, selection.turns.length - VISIBLE_WINDOW),
    );
  }, [selectedIndex, selection.turns.length]);

  const visibleTurns = selection.turns.slice(
    startIndex,
    startIndex + VISIBLE_WINDOW,
  );

  return (
    <Box
      flexDirection="column"
      flexGrow={1}
      flexShrink={1}
      minHeight={0}
      borderStyle="round"
      borderColor="$border"
      overflow="scroll"
      paddingX={1}
    >
      <Text bold color="$warning">
        Rewind Conversation
      </Text>
      <Box marginTop={1} flexDirection="column">
        <Text>
          Choose the user turn to keep. Later messages will be dropped.
        </Text>
        <Text color="$muted">
          {selection.turns.length} available turn
          {selection.turns.length === 1 ? "" : "s"}
        </Text>
      </Box>
      <Box marginTop={1} flexDirection="column">
        {visibleTurns.map((turn, index) => {
          const actualIndex = startIndex + index;
          const isSelected = actualIndex === selectedIndex;

          return (
            <Box
              key={`${turn.turnNumber}-${turn.messageIndex}`}
              marginBottom={1}
            >
              <Text color={isSelected ? "$warning" : "$fg"} bold={isSelected}>
                {isSelected ? "›" : " "} Turn {turn.turnNumber}
              </Text>
              <Text color="$muted">{turn.preview}</Text>
            </Box>
          );
        })}
      </Box>
      <Box marginTop={1} flexDirection="column">
        <Text dimColor>
          Enter rewind · Up/Down change selection · Esc or Q cancel
        </Text>
      </Box>
    </Box>
  );
};

export default RewindSelectionPrompt;
