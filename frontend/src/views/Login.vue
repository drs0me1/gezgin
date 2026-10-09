<template>
  <div id="login">
    <form v-if="changeMode" @submit="submitChange">
      <img :src="logoURL" alt="Gezgin" />
      <h1>{{ name }}</h1>
      <p class="logout-message">{{ t("login.passwordChangeRequired") }}</p>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <input
        v-if="knownPassword === ''"
        autofocus
        class="input input--block"
        type="password"
        autocomplete="current-password"
        v-model="currentPassword"
        :placeholder="t('settings.currentPassword')"
      />
      <input
        class="input input--block"
        type="password"
        autocomplete="new-password"
        v-model="newPassword"
        :placeholder="t('settings.newPassword')"
      />
      <input
        class="input input--block"
        type="password"
        autocomplete="new-password"
        v-model="newPasswordConfirm"
        :placeholder="t('settings.newPasswordConfirm')"
      />

      <input
        class="button button--block"
        type="submit"
        :value="t('login.changePassword')"
      />

      <p @click="leave">{{ t("sidebar.logout") }}</p>
    </form>

    <form v-else @submit="submit">
      <img :src="logoURL" alt="Gezgin" />
      <h1>{{ name }}</h1>
      <p v-if="reason != null" class="logout-message">
        {{ t(`login.logout_reasons.${reason}`) }}
      </p>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <input
        autofocus
        class="input input--block"
        type="text"
        autocapitalize="off"
        v-model="username"
        :placeholder="t('login.username')"
      />
      <input
        class="input input--block"
        type="password"
        v-model="password"
        :placeholder="t('login.password')"
      />
      <input
        class="button button--block"
        type="submit"
        :value="t('login.submit')"
      />
    </form>
  </div>
</template>

<script setup lang="ts">
import { LoginLimitError, StatusError } from "@/api/utils";
import { users as usersApi } from "@/api";
import { useAuthStore } from "@/stores/auth";
import * as auth from "@/utils/auth";
import { name, logoURL } from "@/utils/constants";
import { serverMessage } from "@/utils/serverErrors";
import { computed, inject, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// Define refs
const error = ref<string>("");
const username = ref<string>("");
const password = ref<string>("");
// The forced password change: the password typed at the login, when the change follows it.
const knownPassword = ref<string>("");
const currentPassword = ref<string>("");
const newPassword = ref<string>("");
const newPasswordConfirm = ref<string>("");

const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();
const { t } = useI18n({});

const changeMode = computed(() => authStore.user?.mustChangePassword === true);

const $showError = inject<IToastError>("$showError")!;

const reason = route.query["logout-reason"] ?? null;

const redirect = () => (route.query.redirect || "/files/") as string;

// statusMessage turns the answers the login and the password change share into a message, or
// returns null for an unexpected error.
const statusMessage = (e: StatusError): string | null => {
  if (e instanceof LoginLimitError) {
    const minutes = Math.max(1, Math.ceil(e.retryAfter / 60));
    return t("login.tooManyAttempts", { minutes });
  }
  if (e.status === 409) return t("login.usernameTaken");
  if (e.status === 403) return t("login.wrongCredentials");
  if (e.status === 400) return serverMessage(e.message) ?? e.message;
  return null;
};

const fail = (e: unknown) => {
  if (e instanceof StatusError) {
    const message = statusMessage(e);
    if (message !== null) {
      error.value = message;
      return;
    }
  }
  $showError(e as Error);
};

const submit = async (event: Event) => {
  event.preventDefault();
  event.stopPropagation();
  error.value = "";

  try {
    await auth.login(username.value, password.value);
    if (authStore.user?.mustChangePassword) {
      knownPassword.value = password.value;
      password.value = "";
      return;
    }
    router.replace({ path: redirect() });
  } catch (e) {
    fail(e);
  }
};

const submitChange = async (event: Event) => {
  event.preventDefault();
  event.stopPropagation();
  error.value = "";

  const user = authStore.user;
  if (user === null) return;
  if (newPassword.value !== newPasswordConfirm.value) {
    error.value = t("login.passwordsDontMatch");
    return;
  }

  try {
    await usersApi.update(
      { id: user.id, password: newPassword.value },
      ["password"],
      knownPassword.value || currentPassword.value
    );
    // The change ended the session it was made in; the new password opens the next one.
    await auth.login(user.username, newPassword.value);
    router.replace({ path: redirect() });
  } catch (e) {
    fail(e);
  } finally {
    newPassword.value = newPasswordConfirm.value = "";
  }
};

const leave = () => {
  knownPassword.value = currentPassword.value = "";
  auth.logout();
};
</script>
