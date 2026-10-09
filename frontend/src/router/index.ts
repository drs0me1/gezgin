import type { RouteLocation } from "vue-router";
import { createRouter, createWebHistory } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Share from "@/views/Share.vue";
import Users from "@/views/settings/Users.vue";
import User from "@/views/settings/User.vue";
import Settings from "@/views/Settings.vue";
import GlobalSettings from "@/views/settings/Global.vue";
import ProfileSettings from "@/views/settings/Profile.vue";
import ServerSettings from "@/views/settings/Server.vue";
import Errors from "@/views/Errors.vue";
import Trash from "@/views/Trash.vue";
import Favorites from "@/views/Favorites.vue";
import Shares from "@/views/Shares.vue";
import SearchResults from "@/views/SearchResults.vue";
import { useAuthStore } from "@/stores/auth";
import { baseURL, name } from "@/utils/constants";
import i18n from "@/i18n";
import { validateLogin } from "@/utils/auth";

const titles = {
  Login: "sidebar.login",
  Share: "buttons.share",
  Files: "files.files",
  Trash: "trash.title",
  Favorites: "favorites.title",
  Shares: "shares.title",
  Search: "search.title",
  Settings: "sidebar.settings",
  ProfileSettings: "settings.tabs.account",
  GlobalSettings: "settings.tabs.general",
  Users: "settings.tabs.users",
  User: "settings.user",
  ServerSettings: "settings.tabs.server",
  Forbidden: "errors.forbidden",
  NotFound: "errors.notFound",
  InternalServerError: "errors.internal",
};

const routes = [
  {
    path: "/login",
    name: "Login",
    component: Login,
  },
  {
    path: "/share",
    component: Layout,
    children: [
      {
        path: ":path*",
        name: "Share",
        component: Share,
      },
    ],
  },
  {
    path: "/files",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: ":path*",
        name: "Files",
        component: Files,
      },
    ],
  },
  {
    path: "/favorites",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "Favorites",
        component: Favorites,
      },
    ],
  },
  {
    // The search results of a folder (Gezgin, K111).
    path: "/search",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: ":path*",
        name: "Search",
        component: SearchResults,
      },
    ],
  },
  {
    path: "/shares",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "Shares",
        component: Shares,
      },
    ],
  },
  {
    path: "/trash",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "Trash",
        component: Trash,
      },
    ],
  },
  {
    path: "/settings",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: "",
        name: "Settings",
        component: Settings,
        redirect: {
          path: "/settings/profile",
        },
        children: [
          {
            path: "profile",
            name: "ProfileSettings",
            component: ProfileSettings,
          },
          {
            // The shares have their own page now (Gezgin, K94).
            path: "shares",
            redirect: "/shares",
          },
          {
            path: "global",
            name: "GlobalSettings",
            component: GlobalSettings,
            meta: {
              requiresAdmin: true,
            },
          },
          {
            path: "users",
            name: "Users",
            component: Users,
            meta: {
              requiresAdmin: true,
            },
          },
          {
            path: "users/:id",
            name: "User",
            component: User,
            meta: {
              requiresAdmin: true,
            },
          },
          {
            path: "server",
            name: "ServerSettings",
            component: ServerSettings,
            meta: {
              requiresAdmin: true,
            },
          },
        ],
      },
    ],
  },
  {
    path: "/403",
    name: "Forbidden",
    component: Errors,
    props: {
      errorCode: 403,
      showHeader: true,
    },
  },
  {
    path: "/404",
    name: "NotFound",
    component: Errors,
    props: {
      errorCode: 404,
      showHeader: true,
    },
  },
  {
    path: "/500",
    name: "InternalServerError",
    component: Errors,
    props: {
      errorCode: 500,
      showHeader: true,
    },
  },
  {
    path: "/:catchAll(.*)*",
    redirect: (to: RouteLocation) => {
      const catchAll = to.params.catchAll;
      if (!catchAll) return "/files/";
      return `/files/${Array.isArray(catchAll) ? catchAll.join("/") : catchAll}`;
    },
  },
];

async function initAuth() {
  await validateLogin();
}

const router = createRouter({
  history: createWebHistory(baseURL),
  routes,
});

router.beforeResolve(async (to, from) => {
  const title = i18n.global.t(titles[to.name as keyof typeof titles]);
  document.title = title + " - " + name;

  const authStore = useAuthStore();

  // this will only be null on first route
  if (from.name == null) {
    try {
      await initAuth();
    } catch (error) {
      console.error(error);
    }
  }

  // A user who must change their password stays on the login page, which asks for the new one.
  const mustChangePassword = authStore.user?.mustChangePassword === true;

  if (
    to.path.endsWith("/login") &&
    authStore.isLoggedIn &&
    !mustChangePassword
  ) {
    return { path: "/files/" };
  }

  if (to.matched.some((record) => record.meta.requiresAuth)) {
    if (!authStore.isLoggedIn || mustChangePassword) {
      return {
        path: "/login",
        query: { redirect: to.fullPath },
      };
    }

    if (to.matched.some((record) => record.meta.requiresAdmin)) {
      if (authStore.user === null || !authStore.user.perm.admin) {
        return { path: "/403" };
      }
    }
  }

  return;
});

export { router, router as default };
