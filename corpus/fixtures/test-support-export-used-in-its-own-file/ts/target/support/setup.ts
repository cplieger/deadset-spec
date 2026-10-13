import { vi } from "vitest";

// Only this file reads fakeRequest, so its export is wider than its use.
export const fakeRequest = vi.fn();

Object.defineProperty(globalThis, "XMLHttpRequest", { value: fakeRequest });

// A test imports fakeNow.
export const fakeNow = 1664379699000;

// Nothing reads fakeNever.
export const fakeNever = 2;
