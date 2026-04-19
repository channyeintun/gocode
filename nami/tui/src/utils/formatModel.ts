export function stripProviderPrefix(
  model: string | null | undefined,
): string | null {
  if (typeof model !== "string") {
    return null;
  }

  const compact = model.trim();
  if (!compact) {
    return null;
  }

  const separatorIndex = compact.indexOf("/");
  if (separatorIndex === -1) {
    return compact;
  }

  return compact.slice(separatorIndex + 1).trim() || compact;
}
