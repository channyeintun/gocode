export function formatSubagentType(subagentType: string): string {
  switch (subagentType) {
    case "verification":
      return "Verification";
    case "general-purpose":
      return "General Purpose";
    case "Explore":
    case "explore":
      return "Explore";
    default:
      return subagentType;
  }
}
