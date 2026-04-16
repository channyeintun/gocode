import React, { type FC, useMemo, useState } from "react";
import { Box, Text, useInput } from "silvery";
import type { UIResumeSelection } from "../hooks/useEvents.js";
import { stripProviderPrefix } from "../utils/formatModel.js";

interface ResumeSelectionPromptProps {
  selection: UIResumeSelection;
  onSelect: (sessionId: string) => void;
  onCancel: () => void;
}

const VISIBLE_WINDOW = 8;

const ResumeSelectionPrompt: FC<ResumeSelectionPromptProps> = ({
  selection,
  onSelect,
  onCancel,
}) => {
  const [selectedIndex, setSelectedIndex] = useState(0);

  useInput((input, key) => {
    if (key.escape) {
      onCancel();
      return;
    }

    if (key.upArrow) {
      setSelectedIndex((current) =>
        current === 0 ? selection.sessions.length - 1 : current - 1,
      );
      return;
    }

    if (key.downArrow) {
      setSelectedIndex((current) => (current + 1) % selection.sessions.length);
      return;
    }

    if (key.return) {
      const selected = selection.sessions[selectedIndex];
      if (selected) {
        onSelect(selected.sessionId);
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
    if (selection.sessions.length <= VISIBLE_WINDOW) {
      return 0;
    }
    const centered = selectedIndex - Math.floor(VISIBLE_WINDOW / 2);
    return Math.max(
      0,
      Math.min(centered, selection.sessions.length - VISIBLE_WINDOW),
    );
  }, [selectedIndex, selection.sessions.length]);

  const visibleSessions = selection.sessions.slice(
    startIndex,
    startIndex + VISIBLE_WINDOW,
  );

  return (
    <Box
      flexDirection="column"
      flexGrow={1}
      flexShrink={1}
      minHeight={0}
      backgroundColor="$surface-bg"
      borderStyle="round"
      borderColor="$border"
      overflow="scroll"
      paddingX={1}
    >
      <Text bold color="$primary">
        Resume Session
      </Text>
      <Box marginTop={1} flexDirection="column">
        <Text>Choose a session to resume.</Text>
        <Text color="$muted">
          {selection.sessions.length} available session
          {selection.sessions.length === 1 ? "" : "s"}
        </Text>
      </Box>
      <Box marginTop={1} flexDirection="column">
        {visibleSessions.map((session, index) => {
          const actualIndex = startIndex + index;
          const isSelected = actualIndex === selectedIndex;
          const timestamp = formatUpdatedAt(session.updatedAt);

          return (
            <Box
              key={session.sessionId}
              flexDirection="column"
              marginBottom={1}
            >
              <Text color={isSelected ? "$primary" : "$fg"} bold={isSelected}>
                {isSelected ? "›" : " "} {session.sessionId.slice(0, 8)}{" "}
                {timestamp}
              </Text>
              <Text color="$muted">
                {session.title}
                {session.model
                  ? `  ·  ${stripProviderPrefix(session.model) ?? session.model}`
                  : ""}
                {`  ·  $${session.totalCostUsd.toFixed(4)}`}
              </Text>
            </Box>
          );
        })}
      </Box>
      <Box marginTop={1} flexDirection="column">
        <Text dimColor>
          Enter resume · Up/Down change selection · Esc or Q cancel
        </Text>
      </Box>
    </Box>
  );
};

export default ResumeSelectionPrompt;

function formatUpdatedAt(value: string | null): string {
  if (!value) {
    return "unknown time";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }

  const year = parsed.getFullYear();
  const month = String(parsed.getMonth() + 1).padStart(2, "0");
  const day = String(parsed.getDate()).padStart(2, "0");
  const hours = String(parsed.getHours()).padStart(2, "0");
  const minutes = String(parsed.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hours}:${minutes}`;
}
