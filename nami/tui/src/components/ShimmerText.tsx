import React, { type FC, useEffect, useState } from "react";
import { Text } from "silvery";

interface ShimmerTextProps {
  text: string;
}

const ShimmerText: FC<ShimmerTextProps> = ({ text }) => {
  const [frame, setFrame] = useState(0);

  useEffect(() => {
    // The range of frame is [0, text.length + shimmerWidth + pause]
    // shimmerWidth is 4, pause is 6.
    const totalFrames = text.length + 10;
    const timer = setInterval(() => {
      setFrame((f) => (f + 1) % totalFrames);
    }, 60);

    return () => clearInterval(timer);
  }, [text.length]);

  return (
    <>
      {text.split("").map((char, i) => {
        // We want the shimmer to move from left to right.
        // A character is highlighted if it's within the 'shimmer' window.
        const shimmerWidth = 4;
        const dist = i - frame + shimmerWidth;
        const isHighlight = dist >= 0 && dist < shimmerWidth;

        // Optionally, we could have different levels of brightness,
        // but simple toggle between $primary and $muted is a good start.
        return (
          <Text key={i} color={isHighlight ? "$primary" : "$muted"} italic>
            {char}
          </Text>
        );
      })}
    </>
  );
};

export default ShimmerText;
