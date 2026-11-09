export type HardwareTier = "light" | "media" | "ai";

export interface HardwareCalculatorState {
  dms: boolean;
  audio: boolean;
  video: boolean;
  disc: boolean;
  ai: boolean;
}

export function calculateHardwareTier(features?: HardwareCalculatorState | null): HardwareTier {
  if (!features) {
    return "light";
  }
  if (features.ai) {
    return "ai";
  }
  if (features.video || features.disc) {
    return "media";
  }
  return "light";
}
