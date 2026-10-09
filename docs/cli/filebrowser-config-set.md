# filebrowser config set

Updates the configuration

## Synopsis

Updates the configuration. Set the flags for the options
you want to change. Other options will remain unchanged.

```
filebrowser config set [flags]
```

## Options

```
      --aceEditorTheme string            ace editor's syntax highlighting theme for users
  -a, --address string                   address to listen on (default "127.0.0.1")
      --auth.method string               authentication type (json) (default "json")
  -b, --baseURL string                   base url
      --branding.disableUsedPercentage   disable used disk percentage graph
      --branding.theme string            set the theme: light, dark, or empty to follow the system
  -t, --cert string                      tls certificate
      --createUserDir                    generate user's home directory automatically
      --dateFormat                       use date format (true for absolute time, false for relative)
      --dirMode string                   mode bits that new directories are created with (default "0o750")
      --disableImageResolutionCalc       disables image resolution calculation by reading image files
      --disablePreviewResize             disable resize of image previews
      --disableThumbnails                disable image thumbnails
      --fileMode string                  mode bits that new files are created with (default "0o640")
      --followExternalSymlinks           follow symlinks whose target is outside the user scope (unsafe)
  -h, --help                             help for set
      --hideDotfiles                     hide dotfiles in file listings
  -k, --key string                       tls key
      --locale string                    locale for users (default "tr")
      --lockPassword                     lock password
  -l, --log string                       log output (default "stdout")
      --minimumPasswordLength uint       minimum password length for new users (default 8)
      --perm.admin                       admin perm for users
      --perm.create                      create perm for users (default true)
      --perm.delete                      delete perm for users (default true)
      --perm.download                    download perm for users (default true)
      --perm.modify                      modify perm for users (default true)
      --perm.rename                      rename perm for users (default true)
      --perm.share                       share perm for users (default true)
  -p, --port string                      port to listen on (default "8080")
      --redirectAfterCopyMove            redirect to destination after copy/move
  -r, --root string                      root to prepend to relative paths (default ".")
      --scope string                     scope for users (default ".")
      --singleClick                      use single clicks only
      --socket string                    socket to listen to (cannot be used with address, port, cert nor key flags)
      --sorting.asc                      sorting by ascending order
      --sorting.by string                sorting mode (name, size or modified) (default "name")
      --tokenExpirationTime string       user session timeout (default "2h")
      --tus.chunkSize uint               the tus chunk size (default 10485760)
      --tus.retryCount uint16            the tus retry count (default 5)
      --viewMode string                  view mode for users (default "list")
      --webdavPort string                port to serve WebDAV shares on (off if empty)
```

## Options inherited from parent commands

```
  -c, --config string     config file path
  -d, --database string   database path (default "./filebrowser.db")
```

## See Also

* [filebrowser config](filebrowser-config.md)	 - Configuration management utility

