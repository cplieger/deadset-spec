import { jest } from "@jest/globals";
import { vi } from "vitest";

export class ByGlobalThis {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

export class ByWindow {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

export class BySelf {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

export class ByErasedReceiver {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

export class ByDefineProperty {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

export class ByStubGlobal {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

export class ByReplaceProperty {
  send(data: string): void {
    void data;
  }
  spare(): void {}
}

globalThis.WebSocket = ByGlobalThis as unknown as typeof WebSocket;
window.WebSocket = ByWindow as unknown as typeof WebSocket;
self.WebSocket = BySelf as unknown as typeof WebSocket;
(globalThis as any).WebSocket = ByErasedReceiver;
Object.defineProperty(globalThis, "WebSocket", { value: ByDefineProperty });
vi.stubGlobal("WebSocket", ByStubGlobal);
jest.replaceProperty(globalThis, "WebSocket", ByReplaceProperty as unknown as typeof WebSocket);
