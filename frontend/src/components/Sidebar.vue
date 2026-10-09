<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <!-- At the top, in view when the sidebar scrolls: the favourites and the shares; the user's
           files under them, and the trash at the bottom. The account's settings and logout are
           at the header's right, new folders and files in the folder's right-click menu (Gezgin,
           K117-K120). -->
      <div class="pinned">
        <button
          class="action"
          @click="toFavorites"
          :aria-label="$t('favorites.title')"
          :title="$t('favorites.title')"
        >
          <i class="material-icons">star</i>
          <span>{{ $t("favorites.title") }}</span>
        </button>
        <button
          v-if="user.perm.share"
          class="action"
          @click="toShares"
          :aria-label="$t('shares.title')"
          :title="$t('shares.title')"
        >
          <i class="material-icons">share</i>
          <span>{{ $t("shares.title") }}</span>
        </button>
      </div>

      <div>
        <button
          class="action"
          @click="toRoot"
          :aria-label="$t('sidebar.myFiles')"
          :title="$t('sidebar.myFiles')"
        >
          <i class="material-icons">folder</i>
          <span>{{ $t("sidebar.myFiles") }}</span>
        </button>
      </div>
    </template>

    <!-- At the bottom, in view on every page: the disk use over the trash, and the version
         (Gezgin, K122). -->
    <div class="last">
      <div v-if="isLoggedIn && !disableUsedPercentage" class="usage">
        <progress-bar :val="usage.usedPercentage" size="small"></progress-bar>
        <p>
          {{ $t("sidebar.diskUsed", { used: usage.used, total: usage.total }) }}
        </p>
      </div>
      <button
        v-if="isLoggedIn"
        class="action"
        @click="toTrash"
        :aria-label="$t('trash.title')"
        :title="$t('trash.title')"
      >
        <i class="material-icons">delete</i>
        <span>{{ $t("trash.title") }}</span>
      </button>
      <p class="credits">
        <span>
          <span>{{ name }}</span>
          <span> {{ " " }} {{ version }}</span>
        </span>
        <span>
          <a @click="help">{{ $t("sidebar.help") }}</a>
        </span>
      </p>
    </div>
  </nav>
</template>

<script>
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";

import { name, version, disableUsedPercentage } from "@/utils/constants";
import { files as api } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive(USAGE_DEFAULT);
    return { usage, usageAbortController: new AbortController() };
  },
  components: {
    ProgressBar,
  },
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    name: () => name,
    version: () => version,
    disableUsedPercentage: () => disableUsedPercentage,
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers", "showHover"]),
    abortOngoingFetchUsage() {
      this.usageAbortController.abort();
    },
    async fetchUsage() {
      // The open folder's disk in "Dosyalarım", the user's files' elsewhere (Gezgin, K122).
      const route = this.$route.name === "Files" ? this.$route.path : "/files/";
      const path = route.endsWith("/") ? route : route + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        this.abortOngoingFetchUsage();
        this.usageAbortController = new AbortController();
        const usage = await api.usage(path, this.usageAbortController.signal);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } finally {
        return Object.assign(this.usage, usageStats);
      }
    },
    toRoot() {
      this.$router.push({ path: "/files" });
      this.closeHovers();
    },
    toFavorites() {
      this.$router.push({ path: "/favorites" });
      this.closeHovers();
    },
    toShares() {
      this.$router.push({ path: "/shares" });
      this.closeHovers();
    },
    toTrash() {
      this.$router.push({ path: "/trash" });
      this.closeHovers();
    },
    help() {
      this.showHover("help");
    },
  },
  watch: {
    $route: {
      handler(to, from) {
        // Every page shows it; it is read again in "Dosyalarım" and on coming to a page.
        if (to.name === "Files" || to.name !== from?.name) {
          this.fetchUsage();
        }
      },
      immediate: true,
    },
  },
  unmounted() {
    this.abortOngoingFetchUsage();
  },
};
</script>
