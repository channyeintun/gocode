import path from "node:path";
import React, { type FC, useMemo, useState } from "react";
import { Box, Text, useInput } from "silvery";
import type { PermissionResponseDecision } from "../protocol/types.js";

type PermissionDecision = PermissionResponseDecision;

interface PermissionOption {
  decision: PermissionDecision;
  label: string;
  description: string;
  shortcut: string;
  color: "$success" | "$error" | "$primary" | "$accent";
}

interface PermissionPromptProps {
  tool: string;
  command: string;
  risk: string;
  riskReason?: string;
  permissionLevel?: string;
  targetKind?: string;
  targetValue?: string;
  workingDir?: string;
  onRespond: (decision: PermissionDecision, feedback?: string) => void;
  onCancelTurn: () => void;
}

const OPTIONS: PermissionOption[] = [
  {
    decision: "allow",
    label: "Allow Once",
    description: "Run this request and ask again next time.",
    shortcut: "Y",
    color: "$success",
  },
  {
    decision: "deny",
    label: "Deny",
    description: "Block this request and return control to the agent.",
    shortcut: "N",
    color: "$error",
  },
  {
    decision: "always_allow",
    label: "Always Allow",
    description: "Persist approval for matching requests outside this session.",
    shortcut: "A",
    color: "$primary",
  },
  {
    decision: "allow_all_session",
    label: "Allow Safe This Session",
    description:
      "Auto-approve future non-destructive, non-sensitive requests in this session.",
    shortcut: "S",
    color: "$accent",
  },
];

function getRiskColor(risk: string): "$error" | "$warning" | "$info" {
  if (risk === "destructive") {
    return "$error";
  }

  if (risk === "high") {
    return "$warning";
  }

  return "$info";
}

const PermissionPrompt: FC<PermissionPromptProps> = ({
  tool,
  command,
  risk,
  riskReason,
  permissionLevel,
  targetKind,
  targetValue,
  workingDir,
  onRespond,
  onCancelTurn,
}) => {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [feedback, setFeedback] = useState("");
  const [feedbackCursorOffset, setFeedbackCursorOffset] = useState(0);
  const [isEditingFeedback, setIsEditingFeedback] = useState(false);

  useInput((input, key) => {
    const text = key.text ?? input;

    if (key.escape) {
      onCancelTurn();
      return;
    }

    if (key.tab) {
      setIsEditingFeedback((current) => !current);
      return;
    }

    if (isEditingFeedback) {
      if (key.return) {
        const selected = OPTIONS[selectedIndex];
        if (selected) {
          onRespond(selected.decision, trimFeedback(feedback));
        }
        return;
      }

      if (key.leftArrow || (key.ctrl && input === "b")) {
        setFeedbackCursorOffset((current) => Math.max(0, current - 1));
        return;
      }

      if (key.rightArrow || (key.ctrl && input === "f")) {
        setFeedbackCursorOffset((current) =>
          Math.min(feedback.length, current + 1),
        );
        return;
      }

      if (key.home || (key.ctrl && input === "a")) {
        setFeedbackCursorOffset(0);
        return;
      }

      if (key.end || (key.ctrl && input === "e")) {
        setFeedbackCursorOffset(feedback.length);
        return;
      }

      if (key.backspace || (key.ctrl && input === "h")) {
        if (feedbackCursorOffset === 0) {
          return;
        }
        setFeedback((current) =>
          replaceRange(
            current,
            feedbackCursorOffset - 1,
            feedbackCursorOffset,
            "",
          ),
        );
        setFeedbackCursorOffset((current) => Math.max(0, current - 1));
        return;
      }

      if (key.delete) {
        setFeedback((current) =>
          replaceRange(
            current,
            feedbackCursorOffset,
            feedbackCursorOffset + 1,
            "",
          ),
        );
        return;
      }

      if (key.ctrl && input === "u") {
        setFeedback("");
        setFeedbackCursorOffset(0);
        return;
      }

      if (text && !key.ctrl && !key.meta) {
        setFeedback((current) =>
          replaceRange(
            current,
            feedbackCursorOffset,
            feedbackCursorOffset,
            text,
          ),
        );
        setFeedbackCursorOffset((current) => current + text.length);
      }
      return;
    }

    if (key.upArrow) {
      setSelectedIndex((current) =>
        current === 0 ? OPTIONS.length - 1 : current - 1,
      );
      return;
    }

    if (key.downArrow) {
      setSelectedIndex((current) => (current + 1) % OPTIONS.length);
      return;
    }

    if (key.return) {
      const selected = OPTIONS[selectedIndex];
      if (selected) {
        onRespond(selected.decision, trimFeedback(feedback));
      }
      return;
    }

    const shortcut = input?.toLowerCase();
    if (!shortcut) {
      return;
    }

    const matched = OPTIONS.find(
      (option) => option.shortcut.toLowerCase() === shortcut,
    );
    if (matched) {
      onRespond(matched.decision, trimFeedback(feedback));
    }
  });

  const riskColor = getRiskColor(risk);
  const selectedOption = OPTIONS[selectedIndex] ?? OPTIONS[0];
  const detailValue = targetValue?.trim() || command;
  const question = useMemo(
    () => buildQuestion(tool, targetKind, detailValue),
    [detailValue, targetKind, tool],
  );
  const detailLabel = useMemo(() => buildDetailLabel(targetKind), [targetKind]);
  const toolLabel = useMemo(() => formatToolLabel(tool), [tool]);
  const accessLabel = permissionLevel?.trim() || inferAccessLabel(tool);

  return (
    <Box
      flexDirection="column"
      flexGrow={1}
      flexShrink={1}
      minHeight={0}
      borderStyle="round"
      borderColor={riskColor}
      overflow="scroll"
      paddingX={1}
      userSelect="contain"
    >
      <Text bold color={riskColor}>
        Permission Required
      </Text>
      <Box marginTop={1} flexDirection="column">
        <Text>{question}</Text>
        <Text color="$muted">
          Tool: <Text color="$fg">{toolLabel}</Text>
        </Text>
        <Text color="$muted">
          Access: <Text color="$fg">{accessLabel}</Text>
        </Text>
        <Text color="$muted">
          Risk: <Text color={riskColor}>{risk || "normal"}</Text>
        </Text>
        {riskReason?.trim() ? (
          <Text color="$warning">Policy: {riskReason}</Text>
        ) : null}
        {workingDir ? (
          <Text color="$muted">
            Cwd: <Text color="$fg">{workingDir}</Text>
          </Text>
        ) : null}
      </Box>
      <Box
        marginTop={1}
        paddingX={1}
        borderStyle="round"
        borderColor="$border"
        flexDirection="column"
      >
        <Text color="$muted">{detailLabel}</Text>
        <Text>{detailValue}</Text>
      </Box>
      <Box
        marginTop={1}
        paddingX={1}
        borderStyle="round"
        borderColor={isEditingFeedback ? "$focusborder" : "$border"}
        flexDirection="column"
      >
        <Text color="$muted">Note (optional)</Text>
        {feedback.length === 0 && !isEditingFeedback ? (
          <Text color="$muted">
            Add context for the agent before this decision is applied.
          </Text>
        ) : (
          <Text>
            {renderFeedbackValue(
              feedback,
              feedbackCursorOffset,
              isEditingFeedback,
            )}
          </Text>
        )}
      </Box>
      <Box marginTop={1} flexDirection="column">
        {OPTIONS.map((option, index) => {
          const isSelected = index === selectedIndex;

          return (
            <Box key={option.decision} flexDirection="column" marginBottom={1}>
              <Text
                color={isSelected ? option.color : "$muted"}
                bold={isSelected}
              >
                {isSelected ? "›" : " "} {option.label}{" "}
                <Text dimColor>[{option.shortcut}]</Text>
              </Text>
              <Text color="$muted"> {option.description}</Text>
            </Box>
          );
        })}
      </Box>
      <Box marginTop={1} flexDirection="column">
        <Text dimColor>
          Enter confirm · Up/Down change selection · Tab{" "}
          {isEditingFeedback ? "return to actions" : "edit note"} · Esc cancel
          turn
        </Text>
        <Text dimColor>
          Selected:{" "}
          <Text color={selectedOption.color}>{selectedOption.label}</Text>
        </Text>
      </Box>
    </Box>
  );
};

export default PermissionPrompt;

function buildQuestion(
  tool: string,
  targetKind: string | undefined,
  targetValue: string,
): string {
  if (targetKind === "file" && targetValue.trim()) {
    const fileName = path.basename(targetValue.trim());
    if (tool === "file_edit") {
      return `Allow edits to ${fileName}?`;
    }
    if (tool === "apply_patch") {
      return `Allow patch updates to ${fileName}?`;
    }
    if (tool === "create_file") {
      return `Allow creation of ${fileName}?`;
    }
    if (tool === "file_write") {
      return `Allow overwrite of ${fileName}?`;
    }
    return `Allow access to ${fileName}?`;
  }

  if (tool === "bash") {
    return "Allow shell command to run?";
  }

  if (targetKind === "url" && targetValue.trim()) {
    return `Allow access to ${targetValue.trim()}?`;
  }

  return `Allow ${formatToolLabel(tool)} to continue?`;
}

function buildDetailLabel(targetKind: string | undefined): string {
  switch (targetKind) {
    case "file":
      return "File";
    case "url":
      return "URL";
    case "query":
      return "Query";
    case "pattern":
      return "Pattern";
    case "command":
      return "Command";
    default:
      return "Target";
  }
}

function formatToolLabel(tool: string): string {
  switch (tool) {
    case "bash":
      return "Bash";
    case "apply_patch":
      return "Apply Patch";
    case "create_file":
      return "Create File";
    case "file_write":
      return "File Write";
    case "file_edit":
      return "File Edit";
    default:
      return tool.replace(/_/g, " ");
  }
}

function inferAccessLabel(tool: string): string {
  if (tool === "bash") {
    return "execute";
  }
  if (
    tool === "apply_patch" ||
    tool === "create_file" ||
    tool === "file_write" ||
    tool === "file_edit"
  ) {
    return "write";
  }
  return "ask";
}

function renderFeedbackValue(
  value: string,
  cursorOffset: number,
  isEditing: boolean,
): string {
  if (!isEditing) {
    return value;
  }

  const clampedOffset = Math.max(0, Math.min(value.length, cursorOffset));
  return value.slice(0, clampedOffset) + "█" + value.slice(clampedOffset);
}

function replaceRange(
  value: string,
  start: number,
  end: number,
  replacement: string,
): string {
  const safeStart = Math.max(0, Math.min(value.length, start));
  const safeEnd = Math.max(safeStart, Math.min(value.length, end));
  return value.slice(0, safeStart) + replacement + value.slice(safeEnd);
}

function trimFeedback(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}
