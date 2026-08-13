import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll } from "vitest";
import { cleanup } from "@testing-library/react";
import { server } from "./server";

// input-otp measures its slots with ResizeObserver, which jsdom lacks.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver = globalThis.ResizeObserver ?? ResizeObserverStub;

// input-otp asks for the element under the pointer when dismissing slots;
// jsdom has no layout engine.
if (typeof document.elementFromPoint !== "function") {
  document.elementFromPoint = () => null;
}

// Base UI ScrollArea calls Element.getAnimations() to detect scroll-fade;
// jsdom does not implement the Web Animations API.
if (typeof Element.prototype.getAnimations !== "function") {
  Element.prototype.getAnimations = () => [];
}

afterEach(() => cleanup());

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
