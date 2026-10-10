import { describe, it, expect, vi, afterEach } from "vitest";
import type { HttpRequest, HttpResponse, HttpStack } from "tus-js-client";
import {
  FileReadError,
  WatchdogHttpStack,
  memoryFileReader,
  readChunk,
} from "@/api/tus";

// tus.ts reaches window-bound settings and the stores at module load; the parts tested here
// use none of them.
vi.mock("@/utils/constants", () => ({}));
vi.mock("@/stores/auth", () => ({ useAuthStore: vi.fn() }));
vi.mock("@/api/utils", () => ({ removePrefix: (value: string) => value }));
vi.mock("@/i18n", () => ({ default: { global: { t: (key: string) => key } } }));

afterEach(() => {
  vi.useRealTimers();
});

// A request whose answer and progress the test gives by hand.
interface FakeState {
  progress: (bytes: number) => void;
  answer: (res: HttpResponse) => void;
  fail: (err: Error) => void;
  aborted: boolean;
}

function fakeStack() {
  const state: FakeState = {
    progress: () => {},
    answer: () => {},
    fail: () => {},
    aborted: false,
  };
  const request: HttpRequest = {
    getMethod: () => "PATCH",
    getURL: () => "/api/tus/a.dng",
    setHeader: () => {},
    getHeader: () => undefined,
    setProgressHandler: (handler) => {
      state.progress = handler;
    },
    send: () =>
      new Promise<HttpResponse>((resolve, reject) => {
        state.answer = resolve;
        state.fail = reject;
      }),
    abort: async () => {
      state.aborted = true;
    },
    getUnderlyingObject: () => null,
  };
  const stack: HttpStack = {
    createRequest: () => request,
    getName: () => "fake",
  };
  return { stack, state };
}

const response = { getStatus: () => 204 } as HttpResponse;

describe("WatchdogHttpStack", () => {
  it("aborts a request that sends nothing and gets no answer", async () => {
    vi.useFakeTimers();
    const { stack, state } = fakeStack();
    const req = new WatchdogHttpStack(45000, stack).createRequest("PATCH", "");
    const sent = req.send(new Blob(["x"]));
    const failed = expect(sent).rejects.toThrow("stalled");

    await vi.advanceTimersByTimeAsync(45000);

    await failed;
    expect(state.aborted).toBe(true);
  });

  it("keeps a request alive while its bytes move", async () => {
    vi.useFakeTimers();
    const { stack, state } = fakeStack();
    const req = new WatchdogHttpStack(45000, stack).createRequest("PATCH", "");
    const seen: number[] = [];
    req.setProgressHandler((bytes) => seen.push(bytes));
    const sent = req.send(new Blob(["x"]));

    await vi.advanceTimersByTimeAsync(30000);
    state.progress(1000);
    await vi.advanceTimersByTimeAsync(30000);
    state.progress(2000);
    await vi.advanceTimersByTimeAsync(30000);
    state.answer(response);

    await expect(sent).resolves.toBe(response);
    expect(seen).toEqual([1000, 2000]);
    expect(state.aborted).toBe(false);
  });

  it("passes on the request's own failure", async () => {
    const { stack, state } = fakeStack();
    const req = new WatchdogHttpStack(45000, stack).createRequest("HEAD", "");
    const sent = req.send(null);
    state.fail(new Error("network"));

    await expect(sent).rejects.toThrow("network");
  });
});

describe("memoryFileReader", () => {
  it("hands over each chunk as a Blob of its own", async () => {
    const file = new Blob([new Uint8Array(25)]);
    const source = await memoryFileReader.openFile(file, 10);

    const first = await source.slice(0, 10);
    const last = await source.slice(20, 25);

    expect(source.size).toBe(25);
    expect(first.value).toBeInstanceOf(Blob);
    expect(first.value.size).toBe(10);
    expect(first.done).toBe(false);
    expect(last.value.size).toBe(5);
    expect(last.done).toBe(true);
  });
});

describe("readChunk", () => {
  it("fails with FileReadError when the file does not come", async () => {
    vi.useFakeTimers();
    const stuck = { arrayBuffer: () => new Promise<ArrayBuffer>(() => {}) };
    const read = readChunk(stuck as Blob, 20000);
    const failed = expect(read).rejects.toBeInstanceOf(FileReadError);

    await vi.advanceTimersByTimeAsync(20000);

    await failed;
  });

  it("fails with FileReadError when the browser refuses the file", async () => {
    const refused = {
      arrayBuffer: () => Promise.reject(new Error("NotReadableError")),
    };

    await expect(readChunk(refused as Blob)).rejects.toBeInstanceOf(
      FileReadError
    );
  });
});
