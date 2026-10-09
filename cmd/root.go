package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/frontend"
	fbhttp "github.com/filebrowser/filebrowser/v2/http"
	"github.com/filebrowser/filebrowser/v2/img"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/trash"
	"github.com/filebrowser/filebrowser/v2/users"
)

var (
	flagNamesMigrations = map[string]string{
		"file-mode":               "fileMode",
		"dir-mode":                "dirMode",
		"hide-login-button":       "hideLoginButton",
		"create-user-dir":         "createUserDir",
		"minimum-password-length": "minimumPasswordLength",
		"socket-perm":             "socketPerm",
		"disable-thumbnails":      "disableThumbnails",
		"disable-preview-resize":  "disablePreviewResize",
		"disable-exec":            "disableExec",
		"img-processors":          "imageProcessors",
		"cache-dir":               "cacheDir",
		"token-expiration-time":   "tokenExpirationTime",
		"baseurl":                 "baseURL",
	}

	warnedFlags = map[string]bool{}
)

// TODO(remove): remove after July 2026.
func migrateFlagNames(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if newName, ok := flagNamesMigrations[name]; ok {

		if !warnedFlags[name] {
			warnedFlags[name] = true
			log.Printf("DEPRECATION NOTICE: Flag --%s has been deprecated, use --%s instead\n", name, newName)
		}

		name = newName
	}

	return pflag.NormalizedName(name)
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.SetGlobalNormalizationFunc(migrateFlagNames)

	cobra.MousetrapHelpText = ""

	rootCmd.SetVersionTemplate("File Browser version {{printf \"%s\" .Version}}\n")

	// Flags available across the whole program
	persistent := rootCmd.PersistentFlags()
	persistent.StringP("config", "c", "", "config file path")
	persistent.StringP("database", "d", "./filebrowser.db", "database path")

	// Runtime flags for the root command
	flags := rootCmd.Flags()
	flags.String("username", "admin", "username for the first user when using quick setup")
	flags.String("password", "", "hashed password for the first user when using quick setup")
	flags.Uint32("socketPerm", 0666, "unix socket file permissions")
	flags.String("cacheDir", "", "file cache directory (disabled if empty)")
	flags.Int("imageProcessors", 4, "image processors count")
	addServerFlags(flags)
}

// addServerFlags adds server related flags to the given FlagSet. These flags are available
// in both the root command, config set and config init commands.
func addServerFlags(flags *pflag.FlagSet) {
	flags.StringP("address", "a", "127.0.0.1", "address to listen on")
	flags.StringP("log", "l", "stdout", "log output")
	flags.StringP("port", "p", "8080", "port to listen on")
	flags.StringP("cert", "t", "", "tls certificate")
	flags.StringP("key", "k", "", "tls key")
	flags.StringP("root", "r", ".", "root to prepend to relative paths")
	flags.String("socket", "", "socket to listen to (cannot be used with address, port, cert nor key flags)")
	flags.StringP("baseURL", "b", "", "base url")
	flags.String("tokenExpirationTime", "2h", "user session timeout")
	flags.Bool("disableThumbnails", false, "disable image thumbnails")
	flags.Bool("disablePreviewResize", false, "disable resize of image previews")
	// Gezgin has no command runner: the flag is accepted and ignored, so that an old
	// invocation still starts.
	flags.Bool("disableExec", true, "")
	_ = flags.MarkHidden("disableExec")
	flags.Bool("disableImageResolutionCalc", false, "disables image resolution calculation by reading image files")
	flags.Bool("followExternalSymlinks", false, "follow symlinks whose target is outside the user scope (unsafe)")
	flags.String("webdavPort", "", "port to serve WebDAV shares on (off if empty)")
}

var rootCmd = &cobra.Command{
	Use:   "filebrowser",
	Short: "A stylish web-based file browser",
	Long: `File Browser CLI lets you create the database to use with File Browser,
manage your users and all the configurations without accessing the
web interface.

If you've never run File Browser, you'll need to have a database for
it. Don't worry: you don't need to setup a separate database server.
We're using Bolt DB which is a single file database and all managed
by ourselves.

For this command, all flags are available as environmental variables,
except for "--config", which specifies the configuration file to use.
The environment variables are prefixed by "FB_" followed by the flag name in
UPPER_SNAKE_CASE. For example, the flag "--disablePreviewResize" is available
as FB_DISABLE_PREVIEW_RESIZE.

If "--config" is not specified, File Browser will look for a configuration
file named .filebrowser.{json, toml, yaml, yml} in the following directories:

- ./
- $HOME/
- /etc/filebrowser/

**Note:** Only the options listed below can be set via the config file or
environment variables. Other configuration options live exclusively in the
database and so they must be set by the "config set" or "config
import" commands.

The precedence of the configuration values are as follows:

- Flags
- Environment variables
- Configuration file
- Database values
- Defaults

Also, if the database path doesn't exist, File Browser will enter into
the quick setup mode and a new database will be bootstrapped and a new
user created with the credentials from options "username" and "password".`,
	RunE: withViperAndStore(func(_ *cobra.Command, _ []string, v *viper.Viper, st *store) error {
		if !st.databaseExisted {
			err := quickSetup(v, st.Storage)
			if err != nil {
				return err
			}
		}

		// build img service
		imgWorkersCount := v.GetInt("imageProcessors")
		if imgWorkersCount < 1 {
			return errors.New("image resize workers count could not be < 1")
		}
		imageService := img.New(imgWorkersCount)

		var fileCache diskcache.Interface = diskcache.NewNoOp()
		cacheDir := v.GetString("cacheDir")
		if cacheDir != "" {
			if err := os.MkdirAll(cacheDir, 0700); err != nil {
				return fmt.Errorf("can't make directory %s: %w", cacheDir, err)
			}
			fileCache = diskcache.New(afero.NewOsFs(), cacheDir)
		}

		server, err := getServerSettings(v, st.Storage)
		if err != nil {
			return err
		}
		setupLog(server.Log)

		// A database written by File Browser may still name an auth method Gezgin dropped (noauth,
		// hook, proxy): refuse to serve rather than answer every request with an error.
		set, err := st.Settings.Get()
		if err != nil {
			return err
		}
		if _, err = st.Auth.Get(set.AuthMethod); err != nil {
			return fmt.Errorf("auth method %q: %w; switch to json with 'config set --auth.method json'", set.AuthMethod, err)
		}

		log.Println("NOTICE: File Browser is being wound down.")
		log.Println("NOTICE: The project is archived on 2026-09-01, after which there will be no")
		log.Println("NOTICE: further releases and no security fixes. Known unfixed issues are at")
		log.Println("NOTICE: https://github.com/filebrowser/filebrowser/security/advisories")

		root, err := filepath.Abs(server.Root)
		if err != nil {
			return err
		}
		server.Root = root

		uploadCache := fbhttp.NewUploadCache(filepath.Join(server.Root, fbhttp.UploadsDir))
		defer uploadCache.Close()
		archiveJobs := fbhttp.NewArchiveJobs(filepath.Join(server.Root, fbhttp.ArchiveDir))
		defer archiveJobs.Close()

		adr := server.Address + ":" + server.Port

		var listener net.Listener

		switch {
		case server.Socket != "":
			listener, err = net.Listen("unix", server.Socket)
			if err != nil {
				return err
			}
			socketPerm := v.GetUint32("socketPerm")
			err = os.Chmod(server.Socket, os.FileMode(socketPerm))
			if err != nil {
				return err
			}
		case server.TLSKey != "" && server.TLSCert != "":
			cer, err := tls.LoadX509KeyPair(server.TLSCert, server.TLSKey)
			if err != nil {
				return err
			}
			listener, err = tls.Listen("tcp", adr, &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cer}},
			)
			if err != nil {
				return err
			}
		default:
			listener, err = net.Listen("tcp", adr)
			if err != nil {
				return err
			}
		}

		// The WebDAV shares have a port of their own (Gezgin), with the same TLS as the rest.
		var davListener net.Listener
		if server.WebDAVPort != "" {
			davListener, err = davListen(server)
			if err != nil {
				return err
			}
			defer davListener.Close()
		}

		assetsFs, err := fs.Sub(frontend.Assets(), "dist")
		if err != nil {
			panic(err)
		}

		handler, err := fbhttp.NewHandler(imageService, fileCache, uploadCache, archiveJobs, st.Storage, server, assetsFs)
		if err != nil {
			return err
		}

		go sweepTrash(st.Storage, server.Root)

		defer listener.Close()

		log.Println("Listening on", listener.Addr().String())
		srv := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 60 * time.Second,
		}

		go func() {
			if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("HTTP server error: %v", err)
			}

			log.Println("Stopped serving new connections.")
		}()

		var davSrv *http.Server
		if davListener != nil {
			log.Println("Serving WebDAV shares on", davListener.Addr().String())
			davSrv = &http.Server{
				Handler:           fbhttp.NewWebDAVHandler(fileCache, st.Storage, server),
				ReadHeaderTimeout: 60 * time.Second,
			}
			go func() {
				if err := davSrv.Serve(davListener); !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("WebDAV server error: %v", err)
				}
			}()
		}

		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc,
			os.Interrupt,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT,
		)
		sig := <-sigc
		log.Println("Got signal:", sig)

		shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownRelease()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("HTTP shutdown error: %v", err)
		}
		if davSrv != nil {
			if err := davSrv.Shutdown(shutdownCtx); err != nil {
				log.Printf("WebDAV shutdown error: %v", err)
			}
		}
		log.Println("Graceful shutdown complete.")

		return nil
	}, storeOptions{allowsNoDatabase: true}),
}

// davListen opens the port of the WebDAV shares, with TLS when the server has a certificate.
func davListen(server *settings.Server) (net.Listener, error) {
	adr := server.Address + ":" + server.WebDAVPort
	if server.TLSKey == "" || server.TLSCert == "" {
		return net.Listen("tcp", adr)
	}
	cer, err := tls.LoadX509KeyPair(server.TLSCert, server.TLSKey)
	if err != nil {
		return nil, err
	}
	return tls.Listen("tcp", adr, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cer},
	})
}

// sweepTrash deletes the trash items whose time is up, at start-up and then every hour (Gezgin).
func sweepTrash(s *storage.Storage, root string) {
	for {
		if set, err := s.Settings.Get(); err != nil {
			log.Printf("trash sweep: %v", err)
		} else if days := set.TrashKeepDays(); days > 0 {
			removed, err := trash.Sweep(root, time.Duration(days)*24*time.Hour, time.Now())
			if err != nil {
				log.Printf("trash sweep: %v", err)
			}
			if removed > 0 {
				log.Printf("trash sweep: %d expired items deleted", removed)
			}
		}
		time.Sleep(time.Hour)
	}
}

func getServerSettings(v *viper.Viper, st *storage.Storage) (*settings.Server, error) {
	server, err := st.Settings.GetServer()
	if err != nil {
		return nil, err
	}

	isSocketSet := false
	isAddrSet := false

	if v.IsSet("address") {
		server.Address = v.GetString("address")
		isAddrSet = true
	}

	if v.IsSet("log") {
		server.Log = v.GetString("log")
	}

	if v.IsSet("port") {
		server.Port = v.GetString("port")
		isAddrSet = true
	}

	if v.IsSet("cert") {
		server.TLSCert = v.GetString("cert")
		isAddrSet = true
	}

	if v.IsSet("key") {
		server.TLSKey = v.GetString("key")
		isAddrSet = true
	}

	if v.IsSet("root") {
		server.Root = v.GetString("root")
	}

	if v.IsSet("socket") {
		server.Socket = v.GetString("socket")
		isSocketSet = true
	}

	if v.IsSet("baseURL") {
		server.BaseURL = v.GetString("baseURL")
		// TODO(remove): remove after July 2026.
	} else if v := os.Getenv("FB_BASEURL"); v != "" {
		log.Println("DEPRECATION NOTICE: Environment variable FB_BASEURL has been deprecated, use FB_BASE_URL instead")
		server.BaseURL = v
	}

	if v.IsSet("tokenExpirationTime") {
		server.TokenExpirationTime = v.GetString("tokenExpirationTime")
	}

	if v.IsSet("disableThumbnails") {
		server.EnableThumbnails = !v.GetBool("disableThumbnails")
	}

	if v.IsSet("disablePreviewResize") {
		server.ResizePreview = !v.GetBool("disablePreviewResize")
	}

	if v.IsSet("disableImageResolutionCalc") {
		server.ImageResolutionCal = !v.GetBool("disableImageResolutionCalc")
	}

	if v.IsSet("followExternalSymlinks") {
		server.FollowExternalSymlinks = v.GetBool("followExternalSymlinks")
	}

	if v.IsSet("webdavPort") {
		server.WebDAVPort = v.GetString("webdavPort")
	}

	if isAddrSet && isSocketSet {
		return nil, errors.New("--socket flag cannot be used with --address, --port, --key nor --cert")
	}

	// Do not use saved Socket if address was manually set.
	if isAddrSet && server.Socket != "" {
		server.Socket = ""
	}

	if server.FollowExternalSymlinks {
		log.Println("WARNING: Following external symlinks enabled!")
		log.Println("WARNING: Symlinks pointing outside a user's scope will be followed,")
		log.Println("WARNING: which can expose files outside that scope. Only enable this if")
		log.Println("WARNING: you fully understand and trust the contents of every user scope.")
	}

	return server, nil
}

func setupLog(logMethod string) {
	switch logMethod {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	case "":
		log.SetOutput(io.Discard)
	default:
		log.SetOutput(&lumberjack.Logger{
			Filename:   logMethod,
			MaxSize:    100,
			MaxAge:     14,
			MaxBackups: 10,
		})
	}
}

// generatedPasswordBytes is the randomness of quick setup's admin password: 16 characters.
const generatedPasswordBytes = 12

func quickSetup(v *viper.Viper, s *storage.Storage) error {
	log.Println("Performing quick setup")

	set := &settings.Settings{
		Key:                   generateKey(),
		HideLoginButton:       true,
		CreateUserDir:         false,
		MinimumPasswordLength: settings.DefaultMinimumPasswordLength,
		UserHomeBasePath:      settings.DefaultUsersHomeBasePath,
		Defaults: settings.UserDefaults{
			Scope:                 ".",
			Locale:                "tr",
			SingleClick:           false,
			RedirectAfterCopyMove: true,
			AceEditorTheme:        v.GetString("defaults.aceEditorTheme"),
			Perm: users.Permissions{
				Admin:    false,
				Create:   true,
				Rename:   true,
				Modify:   true,
				Delete:   true,
				Share:    true,
				Download: true,
			},
		},
		AuthMethod: "",
		Branding:   settings.Branding{},
		Tus: settings.Tus{
			ChunkSize:  settings.DefaultTusChunkSize,
			RetryCount: settings.DefaultTusRetryCount,
		},
		Rules: nil,
	}

	set.AuthMethod = auth.MethodJSONAuth
	err := s.Auth.Save(&auth.JSONAuth{})
	if err != nil {
		return err
	}

	err = s.Settings.Save(set)
	if err != nil {
		return err
	}

	ser := &settings.Server{
		BaseURL:                v.GetString("baseURL"),
		Port:                   v.GetString("port"),
		Log:                    v.GetString("log"),
		TLSKey:                 v.GetString("key"),
		TLSCert:                v.GetString("cert"),
		Address:                v.GetString("address"),
		Root:                   v.GetString("root"),
		TokenExpirationTime:    v.GetString("tokenExpirationTime"),
		EnableThumbnails:       !v.GetBool("disableThumbnails"),
		ResizePreview:          !v.GetBool("disablePreviewResize"),
		ImageResolutionCal:     !v.GetBool("disableImageResolutionCalc"),
		FollowExternalSymlinks: v.GetBool("followExternalSymlinks"),
	}

	err = s.Settings.SaveServer(ser)
	if err != nil {
		return err
	}

	username := v.GetString("username")
	password := v.GetString("password")
	generated := password == ""

	if generated {
		var pwd string
		// The generated password does not follow the minimum length, which may be lower.
		pwd, err = users.RandomPwd(generatedPasswordBytes)
		if err != nil {
			return err
		}

		log.Printf("User '%s' initialized with randomly generated password: %s (to be changed at the first login)\n", username, pwd)
		password, err = users.ValidateAndHashPwd(pwd, set.MinimumPasswordLength)
		if err != nil {
			return err
		}
	} else {
		log.Printf("User '%s' initialize wth user-provided password\n", username)
	}

	if username == "" || password == "" {
		log.Fatal("username and password cannot be empty during quick setup")
	}

	user := &users.User{
		Username:     username,
		Password:     password,
		LockPassword: false,
		// The generated password stays in the log; the first login replaces it.
		MustChangePassword: generated,
	}

	set.Defaults.Apply(user)
	user.Perm.Admin = true

	return s.Users.Save(user)
}
