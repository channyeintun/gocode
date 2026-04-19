import React, { type FC, useMemo, useRef } from "react";
import { Box } from "silvery";
import MarkdownTable from "./MarkdownTable.js";
import PreservedText from "./PreservedText.js";
import { renderMarkdownBlocks, cachedLexer } from "../utils/markdown.js";

interface MarkdownTextProps {
  text: string;
  streaming?: boolean;
}

const MarkdownTextBody: FC<Pick<MarkdownTextProps, "text">> = ({ text }) => {
  const rendered = useMemo(() => {
    return renderMarkdownBlocks(text);
  }, [text]);

  return (
    <Box flexDirection="column" width="100%" minWidth={0}>
      {rendered.map((block, index) => {
        if (block.kind === "table") {
          return (
            <Box key={`table-${index}`} marginTop={index === 0 ? 0 : 1}>
              <MarkdownTable token={block.token} />
            </Box>
          );
        }

        return (
          <Box key={`text-${index}`} marginTop={index === 0 ? 0 : 1}>
            <PreservedText text={block.content} />
          </Box>
        );
      })}
    </Box>
  );
};

const StreamingMarkdownText: FC<Pick<MarkdownTextProps, "text">> = ({
  text,
}) => {
  const stablePrefixRef = useRef("");
  const normalized = text.replace(/\r\n/g, "\n");

  if (!normalized.startsWith(stablePrefixRef.current)) {
    stablePrefixRef.current = "";
  }

  const boundary = stablePrefixRef.current.length;
  const tokens = cachedLexer(normalized.slice(boundary));
  let lastContentIndex = tokens.length - 1;

  while (lastContentIndex >= 0 && tokens[lastContentIndex]?.type === "space") {
    lastContentIndex -= 1;
  }

  let advance = 0;
  for (let index = 0; index < lastContentIndex; index += 1) {
    advance += tokens[index]?.raw.length ?? 0;
  }

  if (advance > 0) {
    stablePrefixRef.current = normalized.slice(0, boundary + advance);
  }

  const stablePrefix = stablePrefixRef.current;
  const unstableSuffix = normalized.slice(stablePrefix.length);

  return (
    <Box flexDirection="column" width="100%" minWidth={0}>
      {stablePrefix ? <MarkdownTextBody text={stablePrefix} /> : null}
      {unstableSuffix ? <MarkdownTextBody text={unstableSuffix} /> : null}
    </Box>
  );
};

const MarkdownText: FC<MarkdownTextProps> = ({ text, streaming }) => {
  if (streaming) {
    return <StreamingMarkdownText text={text} />;
  }

  return <MarkdownTextBody text={text} />;
};

export default MarkdownText;
