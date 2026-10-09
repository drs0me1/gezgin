package fbhttp

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
)

type modifyRequest struct {
	What            string   `json:"what"`             // Answer to: what data type?
	Which           []string `json:"which"`            // Answer to: which fields?
	CurrentPassword string   `json:"current_password"` // Answer to: user logged password
}

func NewHandler(
	imgSvc ImgService,
	fileCache FileCache,
	uploadCache *UploadCache,
	archiveJobs *ArchiveJobs,
	store *storage.Storage,
	server *settings.Server,
	assetsFs fs.FS,
) (http.Handler, error) {
	server.Clean()
	server.CaseInsensitiveFs = files.CaseInsensitive(afero.NewOsFs(), server.Root)

	r := mux.NewRouter()
	index, static := getStaticHandlers(store, server, assetsFs)

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, store, server)
	}

	r.HandleFunc("/health", healthHandler)
	r.HandleFunc("/manifest.webmanifest", manifestHandler(server))
	r.PathPrefix("/static").Handler(static)
	r.NotFoundHandler = index

	api := r.PathPrefix("/api").Subrouter()
	// An unknown API path is not the page (Gezgin).
	api.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	})

	tokenExpirationTime := server.GetTokenExpirationTime(DefaultTokenExpirationTime)
	api.Handle("/login", monkey(loginHandler(tokenExpirationTime, newLoginLimiter()), ""))
	api.Handle("/renew", monkey(renewHandler(tokenExpirationTime), ""))

	users := api.PathPrefix("/users").Subrouter()
	users.Handle("", monkey(usersGetHandler, "")).Methods("GET")
	users.Handle("", monkey(userPostHandler, "")).Methods("POST")
	users.Handle("/{id:[0-9]+}", monkey(userPutHandler, "")).Methods("PUT")
	users.Handle("/{id:[0-9]+}", monkey(userGetHandler, "")).Methods("GET")
	users.Handle("/{id:[0-9]+}", monkey(userDeleteHandler, "")).Methods("DELETE")
	users.Handle("/{id:[0-9]+}/sessions", monkey(userSessionsDeleteHandler, "")).Methods("DELETE")

	api.PathPrefix("/resources/recursive").Handler(monkey(resourceGetRecursiveHandler, "/api/resources/recursive")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(resourcePostHandler(fileCache), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(monkey(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH")

	api.PathPrefix("/tus").Handler(monkey(tusPostHandler(uploadCache, fileCache), "/api/tus")).Methods("POST")
	api.PathPrefix("/tus").Handler(monkey(tusHeadHandler(uploadCache), "/api/tus")).Methods("HEAD", "GET")
	api.PathPrefix("/tus").Handler(monkey(tusPatchHandler(uploadCache, fileCache), "/api/tus")).Methods("PATCH")
	api.PathPrefix("/tus").Handler(monkey(tusDeleteHandler(uploadCache), "/api/tus")).Methods("DELETE")

	api.Handle("/favorites", monkey(favoritesGetHandler, "")).Methods("GET")
	api.Handle("/favorites", monkey(favoritesPostHandler, "")).Methods("POST")
	api.Handle("/favorites", monkey(favoritesDeleteHandler, "")).Methods("DELETE")

	api.Handle("/trash", monkey(trashListHandler, "")).Methods("GET")
	api.Handle("/trash", monkey(trashEmptyHandler, "")).Methods("DELETE")
	api.Handle("/trash/restore", monkey(trashRestoreHandler, "")).Methods("POST")
	api.Handle("/trash/purge", monkey(trashPurgeHandler, "")).Methods("POST")
	api.Handle("/trash/all", monkey(trashUsageHandler, "")).Methods("GET")
	api.Handle("/trash/all", monkey(trashEmptyAllHandler, "")).Methods("DELETE")

	api.PathPrefix("/usage").Handler(monkey(diskUsage, "/api/usage")).Methods("GET")

	api.Handle("/shares", monkey(shareListHandler, "")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(shareGetsHandler, "/api/share")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(sharePostHandler, "/api/share")).Methods("POST")
	api.PathPrefix("/share").Handler(monkey(shareDeleteHandler, "/api/share")).Methods("DELETE")
	api.PathPrefix("/share").Handler(monkey(sharePatchHandler, "/api/share")).Methods("PATCH")

	api.Handle("/settings", monkey(settingsGetHandler, "")).Methods("GET")
	api.Handle("/settings", monkey(settingsPutHandler, "")).Methods("PUT")

	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")
	api.PathPrefix("/search").Handler(monkey(searchHandler, "/api/search")).Methods("GET")
	api.Handle("/archive", monkey(archiveListHandler(archiveJobs), "")).Methods("GET")
	api.Handle("/archive", monkey(archiveStartHandler(archiveJobs), "")).Methods("POST")
	api.Handle("/archive/{id:[0-9a-f]+}", monkey(archiveCancelHandler(archiveJobs), "")).Methods("DELETE")
	api.PathPrefix("/subtitle").Handler(monkey(subtitleHandler, "/api/subtitle")).Methods("GET")

	// Share passwords are limited like logins, in budgets of their own.
	shareLimiter := newLoginLimiter()
	public := api.PathPrefix("/public").Subrouter()
	public.PathPrefix("/dl").Handler(monkey(publicDlHandler(shareLimiter), "/api/public/dl/")).Methods("GET")
	public.PathPrefix("/share").Handler(monkey(publicShareHandler(shareLimiter), "/api/public/share/")).Methods("GET")

	return stripPrefix(server.BaseURL, withSecurityHeaders(r)), nil
}

// pageCSP is the policy of Gezgin's pages, and of every answer that does not set its own (Gezgin):
// scripts come only from Gezgin's files, and no other site may frame Gezgin. Ace and the video
// player run workers from blob: URLs, some stylesheets carry their icon fonts as data: URLs, and
// the PDF preview is an <object>.
const pageCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; worker-src 'self' blob:; object-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// rawCSP is the policy of a user's file served as it is: it runs no script, and only Gezgin may
// frame it, for the PDF preview.
const rawCSP = "default-src 'self'; style-src 'unsafe-inline'; script-src 'none'; frame-ancestors 'self'"

// withSecurityHeaders sets the headers of every answer. It wraps the whole router, so that the
// page, which the router serves as its not-found handler, has them too; a middleware does not
// reach that handler.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", pageCSP)
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// manifestHandler serves the web app manifest, which the page used to build in an inline script.
func manifestHandler(server *settings.Server) http.HandlerFunc {
	static := path.Join(server.BaseURL, "/static")
	manifest, _ := json.Marshal(map[string]any{
		"name":       "Gezgin",
		"short_name": "Gezgin",
		"icons": []map[string]string{
			{"src": static + "/img/icons/android-chrome-192x192.png", "sizes": "192x192", "type": "image/png"},
			{"src": static + "/img/icons/android-chrome-512x512.png", "sizes": "512x512", "type": "image/png"},
		},
		"start_url":        server.BaseURL + "/",
		"display":          "standalone",
		"background_color": "#ffffff",
		"theme_color":      "#455a64",
	})
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		_, _ = w.Write(manifest)
	}
}
