# Customization

Gezgin has a fixed brand: its name is Gezgin, and File Browser's instance name, theme colour,
branding folder (custom styles and replacement images) and "disable external links" option are
gone. A database that still holds them loads without them.

What remains is under **Settings → Global Settings → Appearance**, or on the
[CLI](cli/filebrowser-config-set.md):

- **Theme**: light, dark, or the system's (the default).
- **Disable used disk percentage graph**: hides the disk usage on the sidebar.

```sh
filebrowser config set --branding.theme dark --branding.disableUsedPercentage
```
