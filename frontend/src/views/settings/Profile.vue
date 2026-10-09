<template>
  <div v-if="user">
    <div class="settings-card account-card">
      <span class="avatar" aria-hidden="true">{{ initial }}</span>
      <div>
        <div class="account-name">{{ user.username }}</div>
        <div class="setting-help">
          {{
            user.perm.admin ? t("settings.administrator") : t("settings.user")
          }}
        </div>
      </div>
    </div>

    <div class="settings-card">
      <h3>{{ t("settings.preferences") }}</h3>
      <setting-row :label="t('settings.hideDotfiles')">
        <toggle-switch
          v-model="prefs.hideDotfiles"
          :label="t('settings.hideDotfiles')"
        />
      </setting-row>
      <setting-row
        :label="t('settings.singleClick')"
        :help="t('settings.singleClickHelp')"
      >
        <toggle-switch
          v-model="prefs.singleClick"
          :label="t('settings.singleClick')"
        />
      </setting-row>
      <setting-row :label="t('settings.redirectAfterCopyMove')">
        <toggle-switch
          v-model="prefs.redirectAfterCopyMove"
          :label="t('settings.redirectAfterCopyMove')"
        />
      </setting-row>
      <setting-row
        :label="t('settings.setDateFormat')"
        :help="t('settings.dateFormatHelp')"
      >
        <toggle-switch
          v-model="prefs.dateFormat"
          :label="t('settings.setDateFormat')"
        />
      </setting-row>
      <setting-row :label="t('settings.language')" label-for="locale">
        <languages id="locale" class="input" v-model:locale="prefs.locale" />
      </setting-row>
    </div>

    <form class="settings-card" @submit.prevent="updatePassword">
      <h3>{{ t("settings.password") }}</h3>
      <template v-if="!user.lockPassword">
        <p class="setting-help">{{ t("settings.passwordHelp") }}</p>
        <div class="password-fields">
          <input
            class="input input--block"
            type="password"
            :placeholder="t('settings.currentPassword')"
            :aria-label="t('settings.currentPassword')"
            v-model="currentPassword"
            name="current_password"
            autocomplete="current-password"
          />
          <input
            :class="passwordClass"
            type="password"
            :placeholder="t('settings.newPassword')"
            :aria-label="t('settings.newPassword')"
            v-model="password"
            name="password"
            autocomplete="new-password"
          />
          <input
            :class="passwordClass"
            type="password"
            :placeholder="t('settings.newPasswordConfirm')"
            :aria-label="t('settings.newPasswordConfirm')"
            v-model="passwordConf"
            name="passwordConf"
            autocomplete="new-password"
          />
        </div>
        <div class="card-buttons">
          <button class="button" type="submit">
            {{ t("settings.changePassword") }}
          </button>
        </div>
      </template>
      <p v-else class="setting-help">{{ t("settings.passwordLocked") }}</p>
    </form>

    <div class="settings-card">
      <setting-row
        :label="t('settings.sessions')"
        :help="t('settings.sessionsHelp')"
        stack
      >
        <button
          class="button button--flat button--red"
          type="button"
          @click="closeSessions"
        >
          {{ t("settings.closeAllSessions") }}
        </button>
      </setting-row>
    </div>

    <save-bar :visible="dirty" @cancel="reset" @save="savePreferences" />
  </div>
</template>

<script setup lang="ts">
import { users as api } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import SaveBar from "@/components/settings/SaveBar.vue";
import SettingRow from "@/components/settings/SettingRow.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { locales } from "@/i18n";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import * as auth from "@/utils/auth";
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The account's page (Gezgin, K150-K152): who is signed in, the preferences with one save, the
// password, and the sessions.
const layoutStore = useLayoutStore();
const authStore = useAuthStore();
const { t, locale } = useI18n();

const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const $showError = inject<IToastError>("$showError")!;

const user = computed(() => authStore.user);
const initial = computed(() =>
  (user.value?.username ?? "?").charAt(0).toLocaleUpperCase(locale.value)
);

const password = ref<string>("");
const passwordConf = ref<string>("");
const currentPassword = ref<string>("");

type Preferences = Pick<
  IUser,
  "hideDotfiles" | "singleClick" | "redirectAfterCopyMove" | "dateFormat"
> & { locale: string };

const read = (): Preferences => ({
  hideDotfiles: user.value?.hideDotfiles ?? false,
  singleClick: user.value?.singleClick ?? false,
  redirectAfterCopyMove: user.value?.redirectAfterCopyMove ?? false,
  dateFormat: user.value?.dateFormat ?? false,
  // A language Gezgin no longer has reads as English (K142).
  locale: locales.includes(user.value?.locale ?? "")
    ? user.value!.locale
    : "en",
});

const saved = ref<Preferences>(read());
const prefs = ref<Preferences>(read());
const dirty = computed(
  () => JSON.stringify(prefs.value) !== JSON.stringify(saved.value)
);

const reset = () => {
  prefs.value = { ...saved.value };
};

const passwordClass = computed(() => {
  const baseClass = "input input--block";

  if (password.value === "" && passwordConf.value === "") {
    return baseClass;
  }

  if (password.value === passwordConf.value) {
    return `${baseClass} input--green`;
  }

  return `${baseClass} input--red`;
});

onMounted(() => {
  layoutStore.loading = false;
  saved.value = read();
  prefs.value = read();
});

const updatePassword = async () => {
  if (
    password.value !== passwordConf.value ||
    password.value === "" ||
    currentPassword.value === "" ||
    authStore.user === null
  ) {
    return;
  }

  try {
    const data = {
      ...authStore.user,
      id: authStore.user.id,
      password: password.value,
    };
    await api.update(data, ["password"], currentPassword.value);
    // The change ended every session, this one too; the new password opens the next one.
    await auth.login(authStore.user.username, password.value);
    $showSuccess(t("settings.passwordUpdated"));
  } catch (e: any) {
    $showError(e);
  } finally {
    password.value = passwordConf.value = currentPassword.value = "";
  }
};

// Closing every session ends this one too, so it asks first.
const closeSessions = () => {
  layoutStore.showHover({
    prompt: "confirm",
    props: {
      message: t("settings.closeAllSessionsConfirm"),
      confirm: t("settings.closeAllSessions"),
      danger: true,
    },
    confirm: async () => {
      layoutStore.closeHovers();
      if (authStore.user === null) return;
      try {
        await api.closeSessions(authStore.user.id);
        auth.logout("sessions");
      } catch (e: any) {
        $showError(e);
      }
    },
  });
};

const savePreferences = async () => {
  if (authStore.user === null) return;

  try {
    const data = {
      ...authStore.user,
      id: authStore.user.id,
      ...prefs.value,
    };

    await api.update(data, [
      "locale",
      "hideDotfiles",
      "singleClick",
      "redirectAfterCopyMove",
      "dateFormat",
    ]);
    authStore.updateUser(data);
    saved.value = { ...prefs.value };
    $showSuccess(t("settings.settingsUpdated"));
  } catch (e: any) {
    $showError(e);
  }
};
</script>
