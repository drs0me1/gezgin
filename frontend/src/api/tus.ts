import * as tus from "tus-js-client";
import { baseURL, tusEndpoint, tusSettings, origin } from "@/utils/constants";
import { useAuthStore } from "@/stores/auth";
import { removePrefix } from "@/api/utils";
import i18n from "@/i18n";

const RETRY_BASE_DELAY = 1000;
const RETRY_MAX_DELAY = 20000;
const CURRENT_UPLOAD_LIST: { [key: string]: tus.Upload } = {};

// A request that sends no bytes and gets no answer for this long is cut and tried again
// (Gezgin, K169); the server ends a silent chunk sooner, after 30 seconds.
const STALL_TIMEOUT = 45000;
// Reading a chunk into memory takes a moment even on a phone; longer means it will not come.
const READ_TIMEOUT = 20000;
// Chunks up to this size are read into memory before they are sent (Gezgin, K168).
const MEMORY_CHUNK_LIMIT = 64 * 1024 * 1024;

type UploadOptions = ConstructorParameters<typeof tus.Upload>[1];
type FileReader = NonNullable<UploadOptions["fileReader"]>;

// FileReadError is a chunk the browser did not hand over from the file.
export class FileReadError extends Error {}

export function readChunk(
  blob: Blob,
  timeout = READ_TIMEOUT
): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new FileReadError("the file could not be read in time")),
      timeout
    );
    blob.arrayBuffer().then(
      (data) => {
        clearTimeout(timer);
        resolve(data);
      },
      (err) => {
        clearTimeout(timer);
        reject(new FileReadError(String(err)));
      }
    );
  });
}

// memoryFileReader sends each chunk from memory (Gezgin, K168). Safari on an iPhone stopped for
// good before the second chunk of a photo picked from the library: the request never left the
// phone. A chunk read by the page first is sent without going back to the file, and a file that
// cannot be read fails with a reason instead of hanging.
export const memoryFileReader: FileReader = {
  openFile: async (file: Blob) => ({
    size: file.size,
    slice: async (start: number, end: number) => ({
      value: new Blob([await readChunk(file.slice(start, end))]),
      done: end >= file.size,
    }),
    close: () => {},
  }),
};

// WatchdogHttpStack gives the browser's requests, which wait for ever, a limit (Gezgin, K169):
// a request that sends no bytes and gets no answer for the timeout is aborted and fails, and
// tus tries again from the offset the server holds.
export class WatchdogHttpStack implements tus.HttpStack {
  constructor(
    private timeout = STALL_TIMEOUT,
    private stack: tus.HttpStack = new tus.DefaultHttpStack({})
  ) {}

  createRequest(method: string, url: string): tus.HttpRequest {
    return new WatchdogRequest(
      this.stack.createRequest(method, url),
      this.timeout
    );
  }

  getName() {
    return "WatchdogHttpStack";
  }
}

class WatchdogRequest implements tus.HttpRequest {
  private progress: (bytesSent: number) => void = () => {};
  private alive: () => void = () => {};

  constructor(
    private req: tus.HttpRequest,
    private timeout: number
  ) {
    req.setProgressHandler((bytesSent) => {
      this.alive();
      this.progress(bytesSent);
    });
  }

  getMethod() {
    return this.req.getMethod();
  }

  getURL() {
    return this.req.getURL();
  }

  setHeader(header: string, value: string) {
    this.req.setHeader(header, value);
  }

  getHeader(header: string) {
    return this.req.getHeader(header);
  }

  setProgressHandler(handler: (bytesSent: number) => void) {
    this.progress = handler;
  }

  send(body: unknown): Promise<tus.HttpResponse> {
    return new Promise((resolve, reject) => {
      let timer: ReturnType<typeof setTimeout> | undefined;
      const end = () => {
        clearTimeout(timer);
        this.alive = () => {};
      };
      this.alive = () => {
        clearTimeout(timer);
        timer = setTimeout(() => {
          end();
          this.req.abort().catch(() => {});
          reject(new Error("tus: the request stalled"));
        }, this.timeout);
      };
      this.alive();
      this.req.send(body).then(
        (res) => {
          end();
          resolve(res);
        },
        (err) => {
          end();
          reject(err);
        }
      );
    });
  }

  abort() {
    return this.req.abort();
  }

  getUnderlyingObject() {
    return this.req.getUnderlyingObject();
  }
}

export async function upload(
  filePath: string,
  content: ApiContent = "",
  overwrite = false,
  onupload: any
) {
  if (!tusSettings) {
    // Shouldn't happen as we check for tus support before calling this function
    throw new Error("Tus.io settings are not defined");
  }

  filePath = removePrefix(filePath);
  const resourcePath = `${tusEndpoint}${filePath}?override=${overwrite}`;

  const authStore = useAuthStore();

  // Exit early because of typescript, tus content can't be a string
  if (content === "") {
    return false;
  }
  return new Promise<void | string>((resolve, reject) => {
    const upload = new tus.Upload(content, {
      endpoint: `${origin}${baseURL}${resourcePath}`,
      chunkSize: tusSettings.chunkSize,
      retryDelays: computeRetryDelays(tusSettings),
      parallelUploads: 1,
      storeFingerprintForResuming: false,
      httpStack: new WatchdogHttpStack(),
      ...(tusSettings.chunkSize <= MEMORY_CHUNK_LIMIT && {
        fileReader: memoryFileReader,
      }),
      headers: {
        "X-Auth": authStore.jwt,
      },
      onShouldRetry: function (err, retryAttempt) {
        const status = err.originalResponse
          ? err.originalResponse.getStatus()
          : 0;

        // Do not retry for file conflict, nor when the disk is full.
        if (status === 409 || status === 507) {
          return false;
        }

        // A file the browser did not hand over is read once more, not at every retry.
        if (err.causingError instanceof FileReadError && retryAttempt > 0) {
          return false;
        }

        return true;
      },
      onError: function (error: Error | tus.DetailedError) {
        delete CURRENT_UPLOAD_LIST[filePath];

        if (error.message === "Upload aborted") {
          return reject(error);
        }

        const message =
          error instanceof tus.DetailedError
            ? error.causingError instanceof FileReadError
              ? i18n.global.t("errors.fileUnreadable")
              : error.originalResponse === null
                ? "000 No connection"
                : error.originalResponse.getStatus() === 507
                  ? i18n.global.t("errors.noSpace")
                  : error.originalResponse.getBody()
            : "Upload failed";

        console.error(error);

        reject(new Error(message));
      },
      onProgress: function (bytesUploaded) {
        if (typeof onupload === "function") {
          onupload({ loaded: bytesUploaded });
        }
      },
      onSuccess: function () {
        delete CURRENT_UPLOAD_LIST[filePath];
        resolve();
      },
    });
    CURRENT_UPLOAD_LIST[filePath] = upload;
    upload.start();
  });
}

function computeRetryDelays(tusSettings: TusSettings): number[] | undefined {
  if (!tusSettings.retryCount || tusSettings.retryCount < 1) {
    // Disable retries altogether
    return undefined;
  }
  // The tus client expects our retries as an array with computed backoffs
  // E.g.: [0, 3000, 5000, 10000, 20000]
  const retryDelays = [];
  let delay = 0;

  for (let i = 0; i < tusSettings.retryCount; i++) {
    retryDelays.push(Math.min(delay, RETRY_MAX_DELAY));
    delay =
      delay === 0 ? RETRY_BASE_DELAY : Math.min(delay * 2, RETRY_MAX_DELAY);
  }

  return retryDelays;
}

export async function useTus(content: ApiContent) {
  return isTusSupported() && content instanceof Blob;
}

function isTusSupported() {
  return tus.isSupported === true;
}

export function abortAllUploads() {
  for (const filePath in CURRENT_UPLOAD_LIST) {
    if (CURRENT_UPLOAD_LIST[filePath]) {
      CURRENT_UPLOAD_LIST[filePath].abort(true);
      CURRENT_UPLOAD_LIST[filePath].options!.onError!(
        new Error("Upload aborted")
      );
    }
    delete CURRENT_UPLOAD_LIST[filePath];
  }
}
