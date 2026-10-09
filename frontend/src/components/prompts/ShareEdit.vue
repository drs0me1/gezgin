<template>
  <div class="card floating" id="share-edit">
    <div class="card-title">
      <h2>{{ t("shares.editTitle") }}</h2>
    </div>

    <div class="card-content">
      <p class="what">
        <strong>{{ name }}</strong>
        · {{ kind }}
      </p>
      <p class="small">{{ t("shares.addressKept") }}</p>

      <h3>{{ t("shares.end") }}</h3>
      <p>
        <input
          type="radio"
          id="share-duration-keep"
          value="keep"
          v-model="form.duration"
        />
        <label for="share-duration-keep">{{
          t("shares.durationKeep", { end: currentEnd })
        }}</label>
      </p>
      <p>
        <input
          type="radio"
          id="share-duration-set"
          value="set"
          v-model="form.duration"
        />
        <label for="share-duration-set">{{ t("shares.durationSet") }}</label>
      </p>
      <div v-if="form.duration === 'set'" class="input-group input duration">
        <vue-number-input
          center
          controls
          size="small"
          :min="1"
          :max="2147483647"
          v-model="form.time"
        />
        <select class="right" v-model="form.unit" :aria-label="t('time.unit')">
          <option value="minutes">{{ t("time.minutes") }}</option>
          <option value="hours">{{ t("time.hours") }}</option>
          <option value="days">{{ t("time.days") }}</option>
        </select>
      </div>
      <p>
        <input
          type="radio"
          id="share-duration-permanent"
          value="permanent"
          v-model="form.duration"
        />
        <label for="share-duration-permanent">{{
          t("shares.permanent")
        }}</label>
      </p>

      <h3>{{ t("shares.password") }}</h3>
      <p>
        <input
          type="radio"
          id="share-password-keep"
          value="keep"
          v-model="form.password"
        />
        <label for="share-password-keep">{{
          link.hasPassword ? t("shares.passwordKeep") : t("shares.passwordNone")
        }}</label>
      </p>
      <p>
        <input
          type="radio"
          id="share-password-set"
          value="set"
          v-model="form.password"
        />
        <label for="share-password-set">{{
          link.hasPassword
            ? t("shares.passwordChange")
            : t("shares.passwordSet")
        }}</label>
      </p>
      <input
        v-if="form.password === 'set'"
        class="input input--block"
        type="password"
        autocomplete="new-password"
        :aria-label="t('shares.newPassword')"
        :placeholder="t('shares.newPassword')"
        v-model.trim="form.newPassword"
      />
      <p v-if="link.hasPassword && !isWebDAV">
        <input
          type="radio"
          id="share-password-remove"
          value="remove"
          v-model="form.password"
        />
        <label for="share-password-remove">{{
          t("shares.passwordRemove")
        }}</label>
      </p>
      <p class="small">
        {{
          isWebDAV ? t("shares.webdavPasswordHint") : t("shares.passwordHint")
        }}
      </p>

      <template v-if="isWebDAV">
        <h3>{{ t("shares.access") }}</h3>
        <p>
          <input type="checkbox" id="share-writable" v-model="form.writable" />
          <label for="share-writable">{{ t("prompts.webdavWritable") }}</label>
        </p>
        <p v-if="form.writable && !link.writable" class="small">
          {{ t("prompts.webdavWritableWarning") }}
        </p>
      </template>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="layoutStore.closeHovers"
        :aria-label="t('buttons.cancel')"
        :title="t('buttons.cancel')"
      >
        {{ t("buttons.cancel") }}
      </button>
      <button
        id="focus-prompt"
        class="button button--flat button--blue"
        @click="submit"
        :disabled="saving"
        :aria-label="t('buttons.save')"
        :title="t('buttons.save')"
      >
        {{ t("buttons.save") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { share as api } from "@/api";
import type { SharedItem } from "@/api/share";
import { StatusError } from "@/api/utils";
import { useLayoutStore } from "@/stores/layout";
import { shareUpdate, type ShareForm } from "@/utils/shares";
import dayjs from "dayjs";
import { computed, inject, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";

// Changes a share where it is (Gezgin, K96): its end counted again from now or taken away, its
// password set, replaced or, for a link, removed, a WebDAV share's write access. The address stays.

const { t } = useI18n();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;

const props = layoutStore.currentPrompt?.props ?? {};
const link: SharedItem = props.link;
const name: string = props.name ?? link.path;

const isWebDAV = computed(() => link.kind === "webdav");
const kind = computed(() =>
  isWebDAV.value
    ? t("shares.webdav", {
        access: link.writable
          ? t("prompts.webdavReadWrite")
          : t("prompts.webdavReadOnly"),
      })
    : t("shares.link")
);

const currentEnd = computed(() => {
  if (!link.expire) return t("shares.permanent");
  if (link.expire * 1000 <= Date.now()) return t("shares.ended");
  return dayjs(link.expire * 1000).fromNow();
});

const form = reactive<ShareForm>({
  duration: "keep",
  time: 7,
  unit: "days",
  password: "keep",
  newPassword: "",
  writable: !!link.writable,
});
const saving = ref<boolean>(false);

const submit = async () => {
  if (
    form.duration === "set" &&
    (!Number.isInteger(form.time) || form.time < 1)
  ) {
    $showError(t("shares.timeInvalid"));
    return;
  }
  if (form.password === "set" && form.newPassword === "") {
    $showError(t("shares.passwordEmpty"));
    return;
  }
  const body = shareUpdate(link, form);
  if (Object.keys(body).length === 0) {
    layoutStore.closeHovers();
    return;
  }
  saving.value = true;
  try {
    const changed = await api.update(link.hash, body);
    layoutStore.currentPrompt?.confirm(changed);
  } catch (e) {
    // Only a read-write WebDAV share is refused this way: its owner may not write.
    if (e instanceof StatusError && e.status === 403 && "writable" in body) {
      $showError(t("shares.ownerCannotWrite"));
    } else {
      $showError(e as Error);
    }
  } finally {
    saving.value = false;
  }
};
</script>

<style scoped>
#share-edit h3 {
  margin: 1.2em 0 0.4em;
  font-size: 1em;
  font-weight: 500;
  color: var(--textSecondary);
}

#share-edit .card-content p {
  margin: 0.3em 0;
}

#share-edit input[type="radio"],
#share-edit input[type="checkbox"] {
  margin: 0 0.5em 0 0;
}

#share-edit .what {
  color: var(--textSecondary);
  word-break: break-word;
}

#share-edit .duration {
  margin: 0.3em 0 0.5em 1.6em;
}

#share-edit .small {
  font-size: 0.85em;
  color: var(--textPrimary);
}
</style>
