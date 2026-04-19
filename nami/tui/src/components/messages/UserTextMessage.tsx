import React, { type FC } from "react";
import { Box } from "silvery";
import { DEFAULT_PROMPT_MARKER } from "../../constants/prompt.js";
import type { UIUserMessage } from "../../hooks/useEvents.js";
import MessageRow from "../MessageRow.js";
import PreservedText from "../PreservedText.js";

interface UserTextMessageProps {
  message: UIUserMessage;
  continuation?: boolean;
}

const UserTextMessage: FC<UserTextMessageProps> = ({
  message,
  continuation = false,
}) => {
  return (
    <MessageRow
      marker={continuation ? " " : DEFAULT_PROMPT_MARKER.trimEnd()}
      markerColor="$primary"
      label={null}
      marginBottom={continuation ? 0 : 1}
    >
      <Box width="100%" minWidth={0}>
        <PreservedText text={message.text} />
      </Box>
    </MessageRow>
  );
};

export default UserTextMessage;
