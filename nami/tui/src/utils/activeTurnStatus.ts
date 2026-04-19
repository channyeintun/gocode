import type {
  UIActiveTurnStatus,
  UIAssistantBlock,
} from "../hooks/useEvents.js";

export function activeTurnStatusLabel(
  blocks: UIAssistantBlock[],
  activeTurnStatus: UIActiveTurnStatus,
): string {
  switch (activeTurnStatus) {
    case "thinking":
      return "Thinking";
    case "responding":
      return "Responding";
    case "running_tools":
      return "Running tools";
    case "waiting_permission":
      return "Waiting for permission";
    case "cancelling":
      return "Cancelling";
    case "working":
    case "idle":
    default:
      break;
  }

  const lastBlock = blocks[blocks.length - 1];
  if (!lastBlock) {
    return "Working";
  }

  return lastBlock.kind === "thinking" ? "Thinking" : "Responding";
}
