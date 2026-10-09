import dayjs from "dayjs";
import { createI18n } from "vue-i18n";

import("dayjs/locale/en");
import("dayjs/locale/tr");

// All i18n resources specified in the plugin `include` option can be loaded
// at once using the import syntax
import messages from "@intlify/unplugin-vue-i18n/messages";

// Gezgin speaks Turkish and English only (K142): the other languages lacked about half of its
// texts, and every page carried all of them.
export const locales = ["tr", "en"];

// detectLocale picks the browser's language when Gezgin has it, else English.
export function detectLocale() {
  // locale is an RFC 5646 language tag
  // https://developer.mozilla.org/en-US/docs/Web/API/Navigator/language
  return /^tr\b/.test(navigator.language.toLowerCase()) ? "tr" : "en";
}

export const i18n = createI18n({
  locale: detectLocale(),
  fallbackLocale: "en",
  messages,
  // expose i18n.global for outside components
  legacy: true,
});

export function setLocale(locale: string) {
  // An account may still name a language Gezgin no longer has.
  if (!locales.includes(locale)) locale = "en";
  dayjs.locale(locale);
  // according to doc u only need .value if legacy: false but they lied
  // https://vue-i18n.intlify.dev/guide/essentials/scope.html#local-scope-1
  // @ts-expect-error incorrect type when legacy
  i18n.global.locale.value = locale;
}

export function setHtmlLocale(locale: string) {
  document.documentElement.lang = locales.includes(locale) ? locale : "en";
}

export default i18n;
