<template>
  <div
    class="card floating"
    id="share"
    :class="{ 'menu-open': menuFor !== null }"
  >
    <div class="share-head">
      <icon
        class="share-head-icon"
        :name="item?.isDir ? 'folder_open' : 'insert_drive_file'"
      />
      <div class="share-head-text">
        <h2>{{ t("shares.dialogTitle", { name: itemName }) }}</h2>
        <p>
          {{ item?.isDir ? t("shares.folderKind") : t("shares.fileKind") }} ·
          {{ itemPlace }}
        </p>
      </div>
      <button
        class="action"
        @click="layoutStore.closeHovers"
        :aria-label="t('buttons.close')"
        :title="t('buttons.close')"
      >
        <icon name="close" />
      </button>
    </div>

    <div class="card-content">
      <!-- The item's shares, each with its copy and the rest in a menu (K159). -->
      <template v-if="view === 'list'">
        <p class="share-section">
          {{ item?.isDir ? t("shares.ofFolder") : t("shares.ofFile") }}
        </p>
        <div v-for="link in links" :key="link.hash" class="share-row">
          <span class="share-icon">
            <icon :name="link.kind === 'webdav' ? 'dns' : 'link'" />
          </span>
          <div class="share-main">
            <div class="share-kind">
              <template v-if="link.kind === 'webdav'">
                WebDAV
                <span class="share-badge">{{ accessOf(link) }}</span>
              </template>
              <template v-else>
                {{ t("shares.link") }}
                <icon
                  v-if="link.hasPassword"
                  name="lock"
                  class="share-lock"
                  :title="t('shares.protected')"
                />
              </template>
            </div>
            <div class="share-end" :class="endState(link)">
              {{ endLine(link) }}
            </div>
          </div>
          <button
            class="button button--flat share-copy"
            @click="copyText(urlOf(link), t('success.linkCopied'))"
            :title="
              link.kind === 'webdav'
                ? t('shares.copyAddress')
                : t('shares.copyLink')
            "
          >
            <icon name="content_copy" />{{ t("shares.copy") }}
          </button>
          <div class="share-menu">
            <button
              class="action"
              @click.stop="menuFor = menuFor === link.hash ? null : link.hash"
              :aria-label="t('shares.more')"
              :title="t('shares.more')"
              aria-haspopup="menu"
              :aria-expanded="menuFor === link.hash"
            >
              <icon name="more_vert" />
            </button>
            <div v-if="menuFor === link.hash" class="share-pop" role="menu">
              <button role="menuitem" @click="edit(link)">
                <icon name="settings" />{{ t("shares.edit") }}
              </button>
              <button role="menuitem" @click="info(link)">
                <icon name="info" />{{ t("shares.info") }}
              </button>
              <button
                v-if="link.kind === 'webdav'"
                role="menuitem"
                @click="copyText(link.webdavUser ?? '', t('shares.userCopied'))"
              >
                <icon name="person" />{{ t("shares.copyUser") }}
              </button>
              <button
                v-else
                role="menuitem"
                :disabled="!!link.hasPassword"
                :title="
                  link.hasPassword ? t('shares.downloadNeedsNoPassword') : ''
                "
                @click="copyText(downloadOf(link), t('success.linkCopied'))"
              >
                <icon name="file_download" />{{ t("shares.copyDownload") }}
              </button>
              <hr />
              <button role="menuitem" class="danger" @click="remove(link)">
                <icon name="share_off" />{{ t("shares.remove") }}
              </button>
            </div>
          </div>
        </div>
        <button class="button button--flat share-add" @click="openForm">
          <icon name="add" />{{ t("shares.newShare") }}
        </button>
      </template>

      <!-- A new share in the same window (K160-K162). -->
      <form v-else-if="view === 'form'" @submit.prevent="submit">
        <p class="share-section">{{ t("shares.newShare") }}</p>
        <div v-if="canWebDAV" class="segmented share-kinds" role="radiogroup">
          <button
            type="button"
            role="radio"
            :aria-checked="kind === 'link'"
            :class="{ active: kind === 'link' }"
            @click="kind = 'link'"
          >
            <icon name="link" />{{ t("shares.link") }}
          </button>
          <button
            type="button"
            role="radio"
            :aria-checked="kind === 'webdav'"
            :class="{ active: kind === 'webdav' }"
            @click="chooseWebDAV"
          >
            <icon name="dns" />WebDAV
          </button>
        </div>
        <p class="setting-help">
          {{
            kind === "webdav" ? t("shares.webdavHelp") : t("shares.linkHelp")
          }}
        </p>

        <p class="share-label">{{ t("shares.duration") }}</p>
        <div class="share-chips" role="radiogroup">
          <button
            v-for="option in durations"
            :key="option.key"
            type="button"
            role="radio"
            :aria-checked="duration === option.key"
            :class="{ active: duration === option.key }"
            @click="duration = option.key"
          >
            {{ option.label }}
          </button>
        </div>
        <div v-if="duration === 'custom'" class="share-custom">
          <vue-number-input
            controls
            size="small"
            :min="1"
            :max="9999"
            v-model.number="customTime"
          />
          <select
            class="input"
            v-model="customUnit"
            :aria-label="t('time.unit')"
          >
            <option value="minutes">{{ t("time.minutes") }}</option>
            <option value="hours">{{ t("time.hours") }}</option>
            <option value="days">{{ t("time.days") }}</option>
          </select>
        </div>

        <template v-if="kind === 'link'">
          <setting-row
            :label="t('shares.passwordOn')"
            :help="t('shares.passwordOnHelp')"
          >
            <toggle-switch
              v-model="withPassword"
              :label="t('shares.passwordOn')"
            />
          </setting-row>
          <div v-if="withPassword" class="share-field">
            <input
              class="input"
              :type="showPassword ? 'text' : 'password'"
              v-model.trim="linkPassword"
              autocomplete="new-password"
              :placeholder="t('settings.password')"
              :aria-label="t('settings.password')"
            />
            <button
              type="button"
              class="action"
              @click="showPassword = !showPassword"
              :aria-label="
                showPassword
                  ? t('shares.hidePassword')
                  : t('shares.showPassword')
              "
              :title="
                showPassword
                  ? t('shares.hidePassword')
                  : t('shares.showPassword')
              "
            >
              <icon :name="showPassword ? 'visibility_off' : 'preview'" />
            </button>
          </div>
        </template>

        <template v-else>
          <label class="share-label" for="share-webdav-user">
            {{ t("prompts.webdavUser") }}
          </label>
          <input
            id="share-webdav-user"
            class="input input--block"
            type="text"
            autocomplete="off"
            v-model.trim="webdavUser"
          />
          <label class="share-label" for="share-webdav-password">
            {{ t("shares.webdavPassword") }}
          </label>
          <div class="share-field">
            <input
              id="share-webdav-password"
              class="input share-mono"
              type="text"
              autocomplete="off"
              v-model.trim="davPassword"
            />
            <button
              type="button"
              class="button button--flat"
              @click="davPassword = generatePassword()"
            >
              <icon name="refresh" />{{ t("shares.generate") }}
            </button>
          </div>
          <template v-if="canWrite">
            <p class="share-label">{{ t("shares.access") }}</p>
            <div class="segmented" role="radiogroup">
              <button
                type="button"
                role="radio"
                :aria-checked="!writable"
                :class="{ active: !writable }"
                @click="writable = false"
              >
                {{ t("shares.readOnly") }}
              </button>
              <button
                type="button"
                role="radio"
                :aria-checked="writable"
                :class="{ active: writable }"
                @click="writable = true"
              >
                {{ t("shares.readWrite") }}
              </button>
            </div>
            <p v-if="writable" class="share-warning">
              {{ t("prompts.webdavWritableWarning") }}
            </p>
          </template>
        </template>
        <button type="submit" hidden></button>
      </form>

      <!-- The new share's address at once, to be copied (K163). -->
      <template v-else-if="view === 'done' && created">
        <p class="share-ready">
          <icon name="check_circle" />
          {{
            created.kind === "webdav"
              ? t("shares.readyWebdav")
              : t("shares.readyLink")
          }}
        </p>
        <label class="share-label" for="share-created-url">
          {{
            created.kind === "webdav" ? t("shares.address") : t("shares.link")
          }}
        </label>
        <div class="share-field">
          <input
            id="share-created-url"
            class="input share-mono"
            readonly
            :value="urlOf(created)"
            @focus="selectAll"
          />
          <button
            class="button share-copy-main"
            @click="copyText(urlOf(created), t('success.linkCopied'))"
          >
            <icon name="content_copy" />{{ t("shares.copy") }}
          </button>
        </div>
        <template v-if="created.kind === 'webdav'">
          <label class="share-label" for="share-created-user">
            {{ t("prompts.webdavUser") }}
          </label>
          <div class="share-field">
            <input
              id="share-created-user"
              class="input share-mono"
              readonly
              :value="created.webdavUser"
              @focus="selectAll"
            />
            <button
              class="button button--flat"
              @click="
                copyText(created.webdavUser ?? '', t('shares.userCopied'))
              "
            >
              <icon name="content_copy" />{{ t("shares.copy") }}
            </button>
          </div>
          <label class="share-label" for="share-created-password">
            {{ t("settings.password") }}
          </label>
          <div class="share-field">
            <input
              id="share-created-password"
              class="input share-mono"
              readonly
              :value="createdPassword"
              @focus="selectAll"
            />
            <button
              class="button button--flat"
              @click="copyText(createdPassword, t('shares.passwordCopied'))"
            >
              <icon name="content_copy" />{{ t("shares.copy") }}
            </button>
          </div>
          <p class="setting-help">{{ t("shares.passwordOnce") }}</p>
        </template>
        <p class="setting-help">
          {{ endLine(created) }} ·
          {{
            created.hasPassword
              ? t("shares.withPassword")
              : t("shares.withoutPassword")
          }}. {{ t("shares.laterHelp") }}
        </p>
      </template>
    </div>

    <div class="card-action">
      <template v-if="view === 'list'">
        <button
          id="focus-prompt"
          class="button button--flat button--grey"
          @click="layoutStore.closeHovers"
        >
          {{ t("buttons.close") }}
        </button>
      </template>
      <template v-else-if="view === 'form'">
        <button class="button button--flat button--grey" @click="cancelForm">
          {{ t("buttons.cancel") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat button--blue"
          :disabled="creating"
          @click="submit"
        >
          {{ t("shares.create") }}
        </button>
      </template>
      <template v-else>
        <button class="button button--flat button--grey" @click="view = 'list'">
          {{ t("shares.backToList") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat button--blue"
          @click="layoutStore.closeHovers"
        >
          {{ t("buttons.ok") }}
        </button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import * as api from "@/api/index";
import type { WebDAVShare } from "@/api/share";
import Icon from "@/components/Icon.vue";
import SettingRow from "@/components/settings/SettingRow.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { webdavPort } from "@/utils/constants";
import { generatePassword, shareEnd } from "@/utils/shares";
import dayjs from "dayjs";
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";

// Sharing an item (Gezgin, K158-K164): its name at the top, its shares as rows with a copy each
// and the rest in a menu, a new share in the same window (link or WebDAV, the end as presets, the
// password), and once made its address, to be copied at once.

const { t } = useI18n();
const route = useRoute();
const fileStore = useFileStore();
const authStore = useAuthStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

// asciiName makes a WebDAV username from a folder's name: some clients send only ASCII in Basic
// authentication (Gezgin).
const asciiName = (name: string) =>
  (name || "")
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/ı/g, "i")
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);

// item is what is shared: the item selected in a listing, else the file open.
const item = computed(() => {
  const req = fileStore.req;
  if (!fileStore.isListing) return req;
  if (fileStore.selectedCount !== 1) return null;
  return req?.items[fileStore.selected[0]] ?? null;
});

const url = computed(() =>
  fileStore.isListing ? (item.value?.url ?? "") : route.path
);

const itemName = computed(() => item.value?.name || t("sidebar.myFiles"));
const itemPlace = computed(() => {
  const path = item.value?.path ?? "/";
  return path === "/" ? t("sidebar.myFiles") : path;
});

// A folder can be shared over WebDAV when the server has a WebDAV port.
const canWebDAV = computed(() => webdavPort !== "" && !!item.value?.isDir);
// A share may be written to by those whom the user lets write.
const canWrite = computed(() => {
  const perm = authStore.user?.perm;
  return !!perm && perm.create && perm.modify && perm.rename && perm.delete;
});

type View = "list" | "form" | "done";
type Duration = "1d" | "7d" | "30d" | "permanent" | "custom";

const view = ref<View>("list");
const links = ref<Share[]>([]);
const menuFor = ref<string | null>(null);
const now = ref<number>(Date.now() / 1000);

const kind = ref<"link" | "webdav">("link");
const duration = ref<Duration>("7d");
const customTime = ref<number>(12);
const customUnit = ref<"minutes" | "hours" | "days">("hours");
const withPassword = ref<boolean>(false);
const showPassword = ref<boolean>(false);
const linkPassword = ref<string>("");
const webdavUser = ref<string>("");
const davPassword = ref<string>("");
const writable = ref<boolean>(false);
const creating = ref<boolean>(false);

const created = ref<Share | null>(null);
const createdPassword = ref<string>("");

const durations = computed<{ key: Duration; label: string }[]>(() => [
  { key: "1d", label: t("shares.days", 1) },
  { key: "7d", label: t("shares.days", 7) },
  { key: "30d", label: t("shares.days", 30) },
  { key: "permanent", label: t("shares.permanent") },
  { key: "custom", label: t("shares.custom") },
]);

const sort = () => {
  links.value = [...links.value].sort((a, b) => {
    if (!a.expire) return -1;
    if (!b.expire) return 1;
    return a.expire - b.expire;
  });
};

const urlOf = (link: Share) => api.share.getShareURL(link);
const downloadOf = (link: Share) =>
  api.pub.getDownloadURL({ hash: link.hash, path: "" } as any, true);

const accessOf = (link: Share) =>
  link.writable ? t("prompts.webdavReadWrite") : t("prompts.webdavReadOnly");

const endState = (link: Share) => shareEnd(link.expire, now.value);

// endWhen is when a share ends, as the Info window and the shares page write it.
const endWhen = (link: Share) => {
  switch (endState(link)) {
    case "permanent":
      return t("shares.permanent");
    case "ended":
      return t("shares.ended");
    default:
      return dayjs(link.expire * 1000).fromNow();
  }
};

const endText = (link: Share) =>
  ["permanent", "ended"].includes(endState(link))
    ? endWhen(link)
    : t("shares.endsIn", { when: endWhen(link) });

const endLine = (link: Share) =>
  link.kind === "webdav"
    ? t("shares.webdavUser", { user: link.webdavUser }) + " · " + endText(link)
    : endText(link);

const resetForm = () => {
  kind.value = "link";
  duration.value = "7d";
  customTime.value = 12;
  customUnit.value = "hours";
  withPassword.value = false;
  showPassword.value = false;
  linkPassword.value = "";
  davPassword.value = "";
  writable.value = false;
  webdavUser.value = asciiName(item.value?.name ?? "") || "gezgin";
};

const openForm = () => {
  resetForm();
  view.value = "form";
};

const cancelForm = () => {
  if (links.value.length === 0) layoutStore.closeHovers();
  else view.value = "list";
};

// A WebDAV share needs a password: it comes made, to be kept or made again.
const chooseWebDAV = () => {
  kind.value = "webdav";
  if (davPassword.value === "") davPassword.value = generatePassword();
};

// The listed item's shared mark follows its shares (Gezgin, K115).
const countShare = (link: Share, step: number) => {
  const target = item.value as any;
  if (!target) return;
  const key = link.kind === "webdav" ? "sharedDav" : "sharedLinks";
  target[key] = Math.max(0, (target[key] || 0) + step);
};

const copyText = (text: string, done: string) => {
  menuFor.value = null;
  copy({ text }).then(
    () => $showSuccess(done),
    () =>
      copy({ text }, { permission: true }).then(
        () => $showSuccess(done),
        (e) => $showError(e)
      )
  );
};

const selectAll = (event: FocusEvent) =>
  (event.target as HTMLInputElement).select();

const timing = (): [string, string] => {
  switch (duration.value) {
    case "1d":
      return ["1", "days"];
    case "7d":
      return ["7", "days"];
    case "30d":
      return ["30", "days"];
    case "permanent":
      return ["", "hours"];
    default:
      return [String(customTime.value), customUnit.value];
  }
};

const submit = async () => {
  if (creating.value) return;
  if (
    duration.value === "custom" &&
    (!Number.isInteger(customTime.value) || customTime.value < 1)
  ) {
    $showError(t("shares.timeInvalid"));
    return;
  }
  let password = "";
  let webdav: WebDAVShare | undefined;
  if (kind.value === "webdav") {
    if (!webdavUser.value || davPassword.value.length < 8) {
      $showError(t("prompts.webdavIncomplete"));
      return;
    }
    password = davPassword.value;
    webdav = {
      kind: "webdav",
      webdavUser: webdavUser.value,
      writable: writable.value,
    };
  } else if (withPassword.value) {
    if (linkPassword.value === "") {
      $showError(t("shares.passwordMissing"));
      return;
    }
    password = linkPassword.value;
  }

  creating.value = true;
  try {
    const [time, unit] = timing();
    const res = (await api.share.create(
      url.value,
      password,
      time,
      unit,
      webdav
    )) as Share;
    links.value.push(res);
    sort();
    countShare(res, 1);
    created.value = res;
    createdPassword.value = kind.value === "webdav" ? password : "";
    now.value = Date.now() / 1000;
    view.value = "done";
  } catch (e: any) {
    $showError(e);
  } finally {
    creating.value = false;
  }
};

const edit = (link: Share) => {
  menuFor.value = null;
  layoutStore.showHover({
    prompt: "share-edit",
    props: { link, name: itemName.value },
    confirm: (changed: Share) => {
      layoutStore.closeHovers();
      links.value = links.value.map((l) =>
        l.hash === changed.hash ? { ...l, ...changed } : l
      );
      sort();
      now.value = Date.now() / 1000;
      $showSuccess(t("shares.updated", { name: itemName.value }));
    },
  });
};

const info = (link: Share) => {
  menuFor.value = null;
  const path = item.value?.path ?? "/";
  const folder = path.slice(0, path.lastIndexOf("/")) || "/";
  layoutStore.showHover({
    prompt: "share-info",
    props: {
      share: link,
      name: itemName.value,
      folder:
        path === "/" ? "" : folder === "/" ? t("sidebar.myFiles") : folder,
      kind:
        link.kind === "webdav"
          ? t("shares.webdav", { access: accessOf(link) })
          : t("shares.link"),
      url: urlOf(link),
      end: endWhen(link),
      endDate: link.expire ? dayjs(link.expire * 1000).format("LLL") : "",
      owner: "",
    },
  });
};

// A share ends only after its question, as on the shares page (K131).
const remove = (link: Share) => {
  menuFor.value = null;
  layoutStore.showHover({
    prompt: "confirm",
    props: {
      message: t("shares.removeConfirm", { name: itemName.value }),
      confirm: t("shares.remove"),
      danger: true,
    },
    confirm: async () => {
      layoutStore.closeHovers();
      try {
        await api.share.remove(link.hash);
        links.value = links.value.filter((l) => l.hash !== link.hash);
        countShare(link, -1);
        $showSuccess(t("shares.removed", { name: itemName.value }));
        if (links.value.length === 0) openForm();
      } catch (e: any) {
        $showError(e);
      }
    },
  });
};

// A click elsewhere closes a share's menu.
const closeMenu = () => (menuFor.value = null);
watch(menuFor, (open) => {
  if (open) document.addEventListener("click", closeMenu);
  else document.removeEventListener("click", closeMenu);
});
onBeforeUnmount(() => document.removeEventListener("click", closeMenu));

onMounted(async () => {
  resetForm();
  try {
    links.value = (await api.share.get(url.value)) as unknown as Share[];
    sort();
  } catch (e: any) {
    $showError(e);
  }
  view.value = links.value.length === 0 ? "form" : "list";
});
</script>

<style scoped>
#share.card.floating {
  max-width: 30em;
}

#share .share-head {
  display: flex;
  align-items: center;
  gap: 0.75em;
  padding: 1.25em 1em 0.25em 1.25em;
}

#share .share-head-icon {
  color: var(--blue);
  font-size: 1.8em;
}

#share .share-head-text {
  flex: 1;
  min-width: 0;
}

#share .share-head-text h2 {
  margin: 0;
  font-size: 1.15em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

#share .share-head-text p {
  margin: 0.15em 0 0;
  font-size: 0.85em;
  color: var(--textPrimary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

#share .share-section {
  margin: 0 0 0.4em;
  font-size: 0.85em;
  color: var(--textPrimary);
}

#share .share-row {
  display: flex;
  align-items: center;
  gap: 0.6em;
  padding: 0.55em 0;
}

#share .share-row + .share-row {
  border-top: 1px solid var(--divider);
}

#share .share-icon {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 2em;
  height: 2em;
  border-radius: 50%;
  background: rgba(33, 150, 243, 0.15);
  color: var(--blue);
}

#share .share-icon i.material-icons {
  font-size: 1.1em;
}

#share .share-main {
  flex: 1;
  min-width: 0;
}

#share .share-kind {
  display: flex;
  align-items: center;
  gap: 0.4em;
  color: var(--textSecondary);
}

#share .share-lock {
  font-size: 1em;
  color: var(--textPrimary);
}

#share .share-badge {
  padding: 0.05em 0.5em;
  border-radius: 1em;
  background: var(--surfaceSecondary);
  font-size: 0.75em;
}

#share .share-end {
  font-size: 0.85em;
  color: var(--textPrimary);
}

#share .share-end.soon {
  color: #b77a00;
}

:root.dark #share .share-end.soon {
  color: var(--icon-yellow);
}

#share .share-end.ended {
  color: var(--red);
}

#share .button i.material-icons,
#share .share-pop i.material-icons {
  font-size: 1.15em;
  vertical-align: middle;
  margin-right: 0.3em;
}

#share .share-copy {
  padding: 0.4em 0.6em;
}

#share .share-menu {
  position: relative;
}

#share .share-pop {
  position: absolute;
  right: 0;
  top: 2.4em;
  z-index: 5;
  min-width: 15em;
  padding: 0.3em 0;
  background: var(--surfacePrimary);
  border: 1px solid var(--borderSecondary);
  border-radius: 0.4em;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.3);
}

#share .share-pop button {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 0.55em 0.9em;
  border: 0;
  background: none;
  color: var(--textSecondary);
  font: inherit;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
}

#share .share-pop button:hover:not(:disabled) {
  background: var(--hover);
}

#share .share-pop button:disabled {
  opacity: 0.45;
  cursor: default;
}

#share .share-pop button.danger {
  color: var(--red);
}

#share .share-pop hr {
  margin: 0.3em 0;
  border: 0;
  border-top: 1px solid var(--divider);
}

/* A share's menu may reach past the card, which scrolls otherwise. */
#share.card.menu-open,
#share.card.menu-open .card-content {
  overflow: visible;
}

#share .share-add {
  width: 100%;
  margin-top: 0.6em;
  justify-content: center;
}

#share .share-kinds {
  display: flex;
}

#share .share-kinds button {
  flex: 1;
}

#share .segmented button i.material-icons {
  font-size: 1.1em;
  vertical-align: middle;
  margin-right: 0.3em;
}

#share .share-label {
  display: block;
  margin: 0.9em 0 0.35em;
  font-size: 0.85em;
  color: var(--textPrimary);
}

#share .share-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4em;
}

#share .share-chips button {
  padding: 0.35em 0.85em;
  border: 1px solid var(--borderSecondary);
  border-radius: 1em;
  background: none;
  color: var(--textSecondary);
  font: inherit;
  cursor: pointer;
}

#share .share-chips button.active {
  border-color: var(--blue);
  background: rgba(33, 150, 243, 0.15);
  color: var(--blue);
}

#share .share-custom {
  display: flex;
  gap: 0.5em;
  margin-top: 0.6em;
}

#share .share-custom .vue-number-input {
  width: 9rem;
}

#share .share-custom select.input {
  width: auto;
  margin: 0;
}

#share .share-field {
  display: flex;
  align-items: center;
  gap: 0.4em;
}

#share .share-field .input {
  flex: 1;
  min-width: 0;
  margin: 0;
}

#share .share-mono {
  font-family: monospace;
  font-size: 0.9em;
}

#share .share-warning {
  margin: 0.6em 0 0;
  padding: 0.6em 0.75em;
  border-radius: 0.4em;
  background: rgba(253, 188, 75, 0.2);
  font-size: 0.85em;
  line-height: 1.45;
}

#share .share-ready {
  display: flex;
  align-items: center;
  gap: 0.4em;
  margin: 0 0 0.5em;
  color: var(--icon-green);
  font-weight: 500;
}

#share .share-ready i.material-icons {
  font-size: 1.3em;
}

#share .setting-row {
  margin-top: 0.4em;
}
</style>
