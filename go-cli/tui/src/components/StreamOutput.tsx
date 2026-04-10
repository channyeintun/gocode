import React, { type FC } from "react";
import { Box, Text } from "ink";
import Spinner from "ink-spinner";
import type { UIMessage } from "../hooks/useEvents.js";
import MarkdownText from "./MarkdownText.js";

function truncateThinking(text: string): string {
  const lines = text.split("\n").filter((l) => l.trim().length > 0);
  const tail = lines.slice(-4);
  return tail.join("\n");
}

interface StreamOutputProps {
  messages: UIMessage[];
  liveText: string;
  liveThinkingText: string;
  isStreaming: boolean;
}

const StreamOutput: FC<StreamOutputProps> = ({
  messages,
  liveText,
  liveThinkingText,
  isStreaming,
}) => {
  if (messages.length === 0 && !liveText && !liveThinkingText && !isStreaming) {
    return null;
  }

  return (
    <Box flexDirection="column" paddingLeft={1} marginTop={1}>
      {messages.map((message) => (
        <Box key={message.id} flexDirection="column" marginBottom={1}>
          <Text color={message.role === "user" ? "cyan" : "green"} bold>
            {message.role === "user" ? "You" : "Assistant"}
          </Text>
          {message.role === "assistant" ? (
            <MarkdownText text={message.text} />
          ) : (
            <Text>{message.text}</Text>
          )}
        </Box>
      ))}

      {isStreaming && (
        <Box flexDirection="column">
          <Text color="green" bold>
            Assistant
          </Text>
          <Text color="gray">
            <Spinner type="dots" />{" "}
            {liveText
              ? "Responding"
              : liveThinkingText
                ? "Thinking"
                : "Working"}
          </Text>
          {liveThinkingText && !liveText && (
            <Text color="gray">{truncateThinking(liveThinkingText)}</Text>
          )}
          {liveText && <MarkdownText text={liveText} />}
        </Box>
      )}
    </Box>
  );
};

export default StreamOutput;
