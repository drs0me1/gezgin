# Customization

Gezgin has a fixed brand: its name is Gezgin, and File Browser's instance name, theme colour,
branding folder (custom styles and replacement images) and "disable external links" option are
gone. A database that still holds them loads without them.

What remains is under **Ayarlar → Genel → Görünüm**, or on the
[CLI](cli/filebrowser-config-set.md):

- **Tema**: "Sistem" (the default), "Açık" or "Koyu".
- **Kenar çubuğunda disk kullanımını göster**: shows the disk usage in the sidebar (on by
  default; `--branding.disableUsedPercentage` turns it off).

```sh
filebrowser config set --branding.theme dark --branding.disableUsedPercentage
```
