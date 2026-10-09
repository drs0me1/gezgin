import { describe, expect, it, vi } from "vitest";

vi.mock("@/i18n", () => ({
  default: {
    global: {
      t: (key: string, values?: Record<string, unknown>) =>
        values ? `${key} ${JSON.stringify(values)}` : key,
    },
  },
}));

const { serverMessage } = await import("../serverErrors");

// The texts have to match the server's (settings.Validate, users.ValidateAndHashPwd).
describe("serverMessage", () => {
  it("translates the errors a user can cause", () => {
    expect(
      serverMessage(
        "400 Bad Request (invalid request params: the minimum password length must be from 8 to 32)"
      )
    ).toBe('settings.errors.passwordLength {"min":"8","max":"32"}');
    expect(
      serverMessage(
        "400 Bad Request (invalid request params: the chunk size must be from 1048576 to 1073741824 bytes)"
      )
    ).toBe('settings.errors.chunkSize {"min":"1 MiB","max":"1 GiB"}');
    expect(
      serverMessage(
        "400 Bad Request (invalid request params: the retry count must be from 0 to 20)"
      )
    ).toBe('settings.errors.retryCount {"min":"0","max":"20"}');
    expect(
      serverMessage(
        "400 Bad Request (invalid request params: a scope cannot lie in Gezgin's folders)"
      )
    ).toBe("settings.errors.reservedScope");
    expect(
      serverMessage(
        "400 Bad Request (password is too long, maximum length is 72 bytes)"
      )
    ).toBe('login.passwordTooLong {"max":"72"}');
    expect(
      serverMessage(
        "400 Bad Request (password is too short, minimum length is 8)"
      )
    ).toBe('login.passwordTooShort {"min":"8"}');
    expect(
      serverMessage(
        "400 Bad Request (invalid request params: at most 20 favourites)"
      )
    ).toBe('favorites.tooMany {"max":"20"}');
  });

  it("leaves other errors alone", () => {
    expect(serverMessage("500 Internal Server Error")).toBeNull();
  });
});
