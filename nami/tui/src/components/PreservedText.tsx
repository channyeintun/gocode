import React, { type ComponentProps, type FC } from "react";
import { Box, Text } from "silvery";

interface PreservedTextProps {
  text: string;
  color?: ComponentProps<typeof Text>["color"];
  dimColor?: boolean;
  bold?: boolean;
}

const PreservedText: FC<PreservedTextProps> = ({
  text,
  color,
  dimColor,
  bold,
}) => {
  const lines = text.replace(/\r\n/g, "\n").split("\n");

  return (
    <Box flexDirection="column" width="100%" minWidth={0}>
      {lines.map((line, index) => (
        <Text
          key={`line-${index}`}
          color={color}
          dimColor={dimColor}
          bold={bold}
          wrap="wrap"
        >
          {line.length > 0 ? line : " "}
        </Text>
      ))}
    </Box>
  );
};

export default PreservedText;
