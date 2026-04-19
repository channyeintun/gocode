import React, { type FC, useState } from "react";
import { Box, ListView, Text, useBoxRect, useInput } from "silvery";
import type { UIResumeSelection } from "../hooks/useEvents.js";
import { stripProviderPrefix } from "../utils/formatModel.js";

interface ResumeSelectionPromptProps {
  selection: UIResumeSelection;
  onSelect: (sessionId: string) => void;
  onCancel: () => void;
}

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

    const shortcut = input?.toLowerCase();
    if (!shortcut) {
      return;
    }

    if (shortcut === "q") {
      onCancel();
    }
  });

  const handleListSelect = (index: number) => {
    const selected = selection.sessions[index];
    if (selected) {
      onSelect(selected.sessionId);
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
        <Text bold color="$primary">
          Resume Session
        </Text>
        <Box marginTop={1} flexDirection="column" minWidth={0}>
          <Text>Choose a session to resume.</Text>
          <Text color="$muted">
            {selection.sessions.length} available session
            {selection.sessions.length === 1 ? "" : "s"}
          </Text>
        </Box>
      </Box>

      <ResumeSessionList
        sessions={selection.sessions}
        selectedIndex={selectedIndex}
        onCursor={setSelectedIndex}
        onSelectIndex={handleListSelect}
      />
      <Box marginTop={1} flexDirection="column" flexShrink={0}>
        <Text color="$fg">
          <Text color="$primary" bold>
            Enter
          </Text>{" "}
          resume ·{" "}
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

export default ResumeSelectionPrompt;

interface ResumeSessionListProps {
  sessions: UIResumeSelection["sessions"];
  selectedIndex: number;
  onCursor: (index: number) => void;
  onSelectIndex: (index: number) => void;
}

const ResumeSessionList: FC<ResumeSessionListProps> = ({
  sessions,
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
        items={sessions}
        height={viewportHeight}
        nav
        cursorKey={selectedIndex}
        onCursor={onCursor}
        onSelect={onSelectIndex}
        active
        estimateHeight={2}
        overflowIndicator
        getKey={(session) => session.sessionId}
        renderItem={(session, _index, meta) => {
          const isSelected = meta.isCursor;
          const timestamp = formatUpdatedAt(session.updatedAt);

          return (
            <Box
              key={session.sessionId}
              flexDirection="column"
              backgroundColor={isSelected ? "$selectionbg" : undefined}
              paddingX={1}
              marginBottom={1}
              minWidth={0}
            >
              <Text color={isSelected ? "$selection" : "$fg"} bold={isSelected}>
                {isSelected ? "›" : " "} {session.sessionId.slice(0, 8)}{" "}
                {timestamp}
              </Text>
              <Text color={isSelected ? "$selection" : "$muted"}>
                {session.title}
                {session.model
                  ? `  ·  ${stripProviderPrefix(session.model) ?? session.model}`
                  : ""}
                {`  ·  $${session.totalCostUsd.toFixed(4)}`}
              </Text>
            </Box>
          );
        }}
      />
    </Box>
  );
};

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
