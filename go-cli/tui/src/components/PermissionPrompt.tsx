import React, { type FC, useMemo, useState } from "react";
import { Box, Text, useInput } from "ink";

type PermissionDecision =
  | "allow"
  | "deny"
  | "always_allow"
  | "allow_all_session";

interface PermissionOption {
  decision: PermissionDecision;
  label: string;
  description: string;
  shortcut: string;
  color: "green" | "red" | "blue" | "magenta";
}

interface PermissionPromptProps {
  tool: string;
  command: string;
  risk: string;
  onRespond: (decision: PermissionDecision) => void;
  onCancel: () => void;
}

const OPTIONS: PermissionOption[] = [
  {
    decision: "allow",
    label: "Allow Once",
    description: "Run this request and ask again next time.",
    shortcut: "Y",
    color: "green",
  },
  {
    decision: "deny",
    label: "Deny",
    description: "Block this request and return control to the agent.",
    shortcut: "N",
    color: "red",
  },
  {
    decision: "always_allow",
    label: "Always Allow",
    description: "Persist approval for matching requests outside this session.",
    shortcut: "A",
    color: "blue",
  },
  {
    decision: "allow_all_session",
    label: "Allow This Session",
    description: "Skip this permission prompt for the rest of the session.",
    shortcut: "S",
    color: "magenta",
  },
];

function getRiskColor(risk: string): "red" | "yellow" | "cyan" {
  if (risk === "destructive") {
    return "red";
  }

  if (risk === "high") {
    return "yellow";
  }

  return "cyan";
}

const PermissionPrompt: FC<PermissionPromptProps> = ({
  tool,
  command,
  risk,
  onRespond,
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
        onRespond(selected.decision);
      }
      return;
    }

    const shortcut = input.toLowerCase();
    const matched = OPTIONS.find(
      (option) => option.shortcut.toLowerCase() === shortcut,
    );
    if (matched) {
      onRespond(matched.decision);
    }
  });

  const riskColor = getRiskColor(risk);
  const selectedOption = OPTIONS[selectedIndex] ?? OPTIONS[0];
  const question = useMemo(() => `Allow ${tool} to continue?`, [tool]);

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor={riskColor}
      paddingX={1}
    >
      <Text bold color={riskColor}>
        Permission Required
      </Text>
      <Box marginTop={1} flexDirection="column">
        <Text>{question}</Text>
        <Text color="gray">
          Tool: <Text color="white">{tool}</Text>
        </Text>
        <Text color="gray">
          Risk: <Text color={riskColor}>{risk || "normal"}</Text>
        </Text>
      </Box>
      <Box
        marginTop={1}
        paddingX={1}
        borderStyle="round"
        borderColor="gray"
        flexDirection="column"
      >
        <Text color="gray">Command</Text>
        <Text>{command}</Text>
      </Box>
      <Box marginTop={1} flexDirection="column">
        {OPTIONS.map((option, index) => {
          const isSelected = index === selectedIndex;

          return (
            <Box key={option.decision} flexDirection="column" marginBottom={1}>
              <Text
                color={isSelected ? option.color : "gray"}
                bold={isSelected}
              >
                {isSelected ? "›" : " "} {option.label}{" "}
                <Text dimColor>[{option.shortcut}]</Text>
              </Text>
              <Text color="gray"> {option.description}</Text>
            </Box>
          );
        })}
      </Box>
      <Box marginTop={1} flexDirection="column">
        <Text dimColor>
          Enter confirm · Up/Down change selection · Esc deny
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
