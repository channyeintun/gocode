const APPROX_MAX_RESERVED_OUTPUT_TOKENS = 20_000;
const APPROX_AUTOCOMPACT_BUFFER_TOKENS = 13_000;
const APPROX_WARNING_THRESHOLD_BUFFER_TOKENS = 20_000;
const APPROX_MANUAL_COMPACT_BUFFER_TOKENS = 3_000;

export function inferContextWindow(model: string): number {
  const normalized = model.toLowerCase();

  if (normalized.includes("claude")) {
    return 200_000;
  }
  if (normalized.includes("gemini")) {
    return 1_000_000;
  }
  if (normalized.includes("deepseek")) {
    return 64_000;
  }
  if (normalized.includes("qwen") || normalized.includes("llama-4")) {
    return 131_072;
  }
  if (normalized.includes("glm") || normalized.includes("mistral")) {
    return 128_000;
  }
  if (normalized.includes("gemma") || normalized.includes("ollama")) {
    return 32_000;
  }
  if (
    normalized.includes("gpt") ||
    normalized.includes("o1") ||
    normalized.includes("o3") ||
    normalized.includes("o4")
  ) {
    return 128_000;
  }

  return 128_000;
}

export function getApproxEffectiveContextWindow(model: string): number {
  return Math.max(
    0,
    inferContextWindow(model) - APPROX_MAX_RESERVED_OUTPUT_TOKENS,
  );
}

export function getApproxCompactThreshold(model: string): number {
  return Math.max(
    0,
    getApproxEffectiveContextWindow(model) - APPROX_AUTOCOMPACT_BUFFER_TOKENS,
  );
}

export function calculateApproxTokenWarningState(
  tokenUsage: number,
  model: string,
): {
  percentLeft: number;
  isWarning: boolean;
  isError: boolean;
  effectiveContextWindow: number;
  compactThreshold: number;
} {
  const effectiveContextWindow = getApproxEffectiveContextWindow(model);
  const compactThreshold = getApproxCompactThreshold(model);
  const warningThreshold = Math.max(
    0,
    compactThreshold - APPROX_WARNING_THRESHOLD_BUFFER_TOKENS,
  );
  const blockingLimit = Math.max(
    0,
    effectiveContextWindow - APPROX_MANUAL_COMPACT_BUFFER_TOKENS,
  );
  const percentLeft = Math.max(
    0,
    compactThreshold > 0
      ? Math.round(((compactThreshold - tokenUsage) / compactThreshold) * 100)
      : 0,
  );

  return {
    percentLeft,
    isWarning: compactThreshold > 0 && tokenUsage >= warningThreshold,
    isError: effectiveContextWindow > 0 && tokenUsage >= blockingLimit,
    effectiveContextWindow,
    compactThreshold,
  };
}

export function formatTokenCount(value: number): string {
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(1)}M`;
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(1)}k`;
  }
  return `${value}`;
}
