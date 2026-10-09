<template>
  <div class="card floating" id="share">
    <div class="card-title">
      <h2>{{ $t("buttons.share") }}</h2>
    </div>

    <template v-if="listing">
      <div class="card-content">
        <table>
          <tr>
            <th>#</th>
            <th>{{ $t("settings.shareDuration") }}</th>
            <th></th>
            <th></th>
            <th></th>
          </tr>

          <tr v-for="link in links" :key="link.hash">
            <td>
              {{ link.hash }}
              <p v-if="link.kind === 'webdav'" class="small">
                WebDAV · {{ link.webdavUser }} ·
                {{
                  link.writable
                    ? $t("prompts.webdavReadWrite")
                    : $t("prompts.webdavReadOnly")
                }}
              </p>
            </td>
            <td>
              <template v-if="link.expire !== 0">{{
                humanTime(link.expire)
              }}</template>
              <template v-else>{{ $t("permanent") }}</template>
            </td>
            <td class="small">
              <button
                class="action"
                :aria-label="$t('buttons.copyToClipboard')"
                :title="$t('buttons.copyToClipboard')"
                @click="copyToClipboard(buildLink(link))"
              >
                <i class="material-icons">content_copy</i>
              </button>
            </td>
            <td class="small">
              <button
                class="action"
                :aria-label="$t('buttons.copyDownloadLinkToClipboard')"
                :title="$t('buttons.copyDownloadLinkToClipboard')"
                :disabled="!!link.hasPassword || link.kind === 'webdav'"
                @click="copyToClipboard(buildDownloadLink(link))"
              >
                <i class="material-icons">content_paste_go</i>
              </button>
            </td>
            <td class="small">
              <button
                class="action"
                @click="deleteLink($event, link)"
                :aria-label="$t('buttons.delete')"
                :title="$t('buttons.delete')"
              >
                <i class="material-icons">delete</i>
              </button>
            </td>
          </tr>
        </table>
      </div>

      <div class="card-action">
        <button
          class="button button--flat button--grey"
          @click="closeHovers"
          :aria-label="$t('buttons.close')"
          :title="$t('buttons.close')"
          tabindex="2"
        >
          {{ $t("buttons.close") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat button--blue"
          @click="() => switchListing()"
          :aria-label="$t('buttons.new')"
          :title="$t('buttons.new')"
          tabindex="1"
        >
          {{ $t("buttons.new") }}
        </button>
      </div>
    </template>

    <template v-else>
      <div class="card-content">
        <template v-if="canWebDAV">
          <p>{{ $t("prompts.shareKind") }}</p>
          <select
            class="input input--block"
            v-model="kind"
            :aria-label="$t('prompts.shareKind')"
          >
            <option value="link">{{ $t("prompts.shareKindLink") }}</option>
            <option value="webdav">{{ $t("prompts.shareKindWebdav") }}</option>
          </select>
        </template>
        <template v-if="kind === 'webdav'">
          <p>{{ $t("prompts.webdavUser") }}</p>
          <input
            class="input input--block"
            type="text"
            autocomplete="off"
            v-model.trim="webdavUser"
          />
        </template>
        <p>{{ $t("settings.shareDuration") }}</p>
        <div class="input-group input">
          <vue-number-input
            center
            controls
            size="small"
            :max="2147483647"
            :min="0"
            @keyup.enter="submit"
            v-model="time"
            tabindex="1"
          />
          <select
            class="right"
            v-model="unit"
            :aria-label="$t('time.unit')"
            tabindex="2"
          >
            <option value="seconds">{{ $t("time.seconds") }}</option>
            <option value="minutes">{{ $t("time.minutes") }}</option>
            <option value="hours">{{ $t("time.hours") }}</option>
            <option value="days">{{ $t("time.days") }}</option>
          </select>
        </div>
        <p class="small">{{ $t("prompts.sharePermanentHint") }}</p>
        <p>
          {{
            kind === "webdav"
              ? $t("prompts.webdavPassword")
              : $t("prompts.optionalPassword")
          }}
        </p>
        <input
          class="input input--block"
          type="password"
          v-model.trim="password"
          tabindex="3"
        />
        <template v-if="kind === 'webdav' && canWrite">
          <p>
            <input type="checkbox" id="share-writable" v-model="writable" />
            <label for="share-writable">{{
              $t("prompts.webdavWritable")
            }}</label>
          </p>
          <p v-if="writable" class="small">
            {{ $t("prompts.webdavWritableWarning") }}
          </p>
        </template>
      </div>

      <div class="card-action">
        <button
          class="button button--flat button--grey"
          @click="() => switchListing()"
          :aria-label="$t('buttons.cancel')"
          :title="$t('buttons.cancel')"
          tabindex="5"
        >
          {{ $t("buttons.cancel") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat button--blue"
          @click="submit"
          :aria-label="$t('buttons.share')"
          :title="$t('buttons.share')"
          tabindex="4"
        >
          {{ $t("buttons.share") }}
        </button>
      </div>
    </template>
  </div>
</template>

<script>
import { mapActions, mapState } from "pinia";
import { useFileStore } from "@/stores/file";
import * as api from "@/api/index";
import dayjs from "dayjs";
import { useLayoutStore } from "@/stores/layout";
import { useAuthStore } from "@/stores/auth";
import { copy } from "@/utils/clipboard";
import { webdavPort } from "@/utils/constants";

// asciiName makes a WebDAV username from a folder's name: some clients send only ASCII in Basic
// authentication (Gezgin).
const asciiName = (name) =>
  (name || "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/ı/g, "i")
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);

export default {
  name: "share",
  data: function () {
    return {
      time: 7,
      unit: "days",
      links: [],
      clip: null,
      password: "",
      listing: true,
      kind: "link",
      webdavUser: "",
      writable: false,
    };
  },
  inject: ["$showError", "$showSuccess"],
  computed: {
    ...mapState(useFileStore, [
      "req",
      "selected",
      "selectedCount",
      "isListing",
    ]),
    ...mapState(useAuthStore, ["user"]),
    // item is what is shared.
    item() {
      if (!this.isListing) {
        return this.req;
      }
      if (this.selectedCount !== 1) {
        return null;
      }
      return this.req.items[this.selected[0]];
    },
    // A folder can be shared over WebDAV when the server has a WebDAV port.
    canWebDAV() {
      return webdavPort !== "" && !!this.item?.isDir;
    },
    // A share may be written to by those whom the user lets write.
    canWrite() {
      const perm = this.user?.perm;
      return !!perm && perm.create && perm.modify && perm.rename && perm.delete;
    },
    url() {
      if (!this.isListing) {
        return this.$route.path;
      }

      if (this.selectedCount === 0 || this.selectedCount > 1) {
        // This shouldn't happen.
        return;
      }

      return this.req.items[this.selected[0]].url;
    },
  },
  async beforeMount() {
    this.webdavUser = asciiName(this.item?.name) || "gezgin";
    try {
      const links = await api.share.get(this.url);
      this.links = links;
      this.sort();

      if (this.links.length == 0) {
        this.listing = false;
      }
    } catch (e) {
      this.$showError(e);
    }
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    copyToClipboard: function (text) {
      copy({ text }).then(
        () => {
          // clipboard successfully set
          this.$showSuccess(this.$t("success.linkCopied"));
        },
        () => {
          // clipboard write failed
          copy({ text }, { permission: true }).then(
            () => {
              // clipboard successfully set
              this.$showSuccess(this.$t("success.linkCopied"));
            },
            (e) => {
              // clipboard write failed
              this.$showError(e);
            }
          );
        }
      );
    },
    submit: async function () {
      const webdav =
        this.kind === "webdav"
          ? {
              kind: "webdav",
              webdavUser: this.webdavUser,
              writable: this.writable,
            }
          : undefined;
      if (webdav && (!this.webdavUser || this.password.length < 8)) {
        this.$showError(this.$t("prompts.webdavIncomplete"));
        return;
      }

      try {
        let res = null;

        if (!this.time) {
          res = await api.share.create(
            this.url,
            this.password,
            "",
            "hours",
            webdav
          );
        } else {
          res = await api.share.create(
            this.url,
            this.password,
            this.time,
            this.unit,
            webdav
          );
        }

        this.links.push(res);
        this.sort();

        this.time = 7;
        this.unit = "days";
        this.password = "";
        this.writable = false;

        this.listing = true;
      } catch (e) {
        this.$showError(e);
      }
    },
    deleteLink: async function (event, link) {
      event.preventDefault();
      try {
        await api.share.remove(link.hash);
        this.links = this.links.filter((item) => item.hash !== link.hash);

        if (this.links.length == 0) {
          this.listing = false;
        }
      } catch (e) {
        this.$showError(e);
      }
    },
    humanTime(time) {
      return dayjs(time * 1000).fromNow();
    },
    buildLink(share) {
      return api.share.getShareURL(share);
    },
    buildDownloadLink(share) {
      return api.pub.getDownloadURL(
        {
          hash: share.hash,
          path: "",
        },
        true
      );
    },
    sort() {
      this.links = this.links.sort((a, b) => {
        if (a.expire === 0) return -1;
        if (b.expire === 0) return 1;
        return new Date(a.expire) - new Date(b.expire);
      });
    },
    switchListing() {
      if (this.links.length == 0 && !this.listing) {
        this.closeHovers();
      }

      this.listing = !this.listing;
    },
  },
};
</script>
