import i18n from "@/i18n";
import { filesize } from "@/utils";

// Gezgin's server words its errors in English (they also reach its log). The ones a user can
// cause are shown in the interface's language.
const known: [RegExp, (m: RegExpMatchArray) => string][] = [
  [
    /minimum length is (\d+)/,
    (m) => i18n.global.t("login.passwordTooShort", { min: m[1] }),
  ],
  [
    /password is too long, maximum length is (\d+) bytes/,
    (m) => i18n.global.t("login.passwordTooLong", { max: m[1] }),
  ],
  [/must differ/, () => i18n.global.t("login.passwordUnchanged")],
  [
    /current password is incorrect/,
    () => i18n.global.t("login.currentPasswordIncorrect"),
  ],
  [
    /minimum password length must be from (\d+) to (\d+)/,
    (m) =>
      i18n.global.t("settings.errors.passwordLength", { min: m[1], max: m[2] }),
  ],
  [
    /chunk size must be from (\d+) to (\d+) bytes/,
    (m) =>
      i18n.global.t("settings.errors.chunkSize", {
        min: filesize(Number(m[1])),
        max: filesize(Number(m[2])),
      }),
  ],
  [
    /retry count must be from (\d+) to (\d+)/,
    (m) =>
      i18n.global.t("settings.errors.retryCount", { min: m[1], max: m[2] }),
  ],
  [
    /a scope cannot lie in Gezgin's folders/,
    () => i18n.global.t("settings.errors.reservedScope"),
  ],
  [/a share lasts at most 10 years/, () => i18n.global.t("shares.tooLong")],
  // The browser's own: the server could not be reached, or the connection broke.
  [
    /^00[01] (No connection|Connection aborted)/,
    () => i18n.global.t("errors.connection"),
  ],
  [
    /at most (\d+) favourites/,
    (m) => i18n.global.t("favorites.tooMany", { max: m[1] }),
  ],
];

// serverMessage returns the translated text of a known server error, or null.
export function serverMessage(message: string): string | null {
  for (const [pattern, text] of known) {
    const match = message.match(pattern);
    if (match) return text(match);
  }
  return null;
}
