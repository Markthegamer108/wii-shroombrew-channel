# Native channel content service

The original Shop v21 WAD stays the client. The existing patched WAD retains the native engine and calls this server. No API key is embedded in the client.

`GET`/`HEAD /ccs/download/<16-digit-title-id>/tmd`, `tmd.<version>`, and `<8-digit-content-id>` serve binary Wii metadata and encrypted content. HTTP byte ranges support resuming downloads. Only filenames listed by an imported manifest are served; tickets and certificates are not public files. ECS PurchaseTitle returns the imported ticket through the existing authenticated SOAP route and refuses catalog-only entries with no native package.

Import an existing homebrew application-channel WAD into a staging store:

    go run ./cmd/import-channel /path/to/homebrew.wad /path/to/staging-channels

The importer checks ticket/TMD identity, content hashes, unique IDs and boot-content presence. It excludes IOS and system-channel title IDs, including the Wii Shop itself. It preserves the source package's encrypted content and ticket key. A matching catalog title ID and version are required by PurchaseTitle. Review and test the package in Dolphin before placing its folder in `/opt/shroombrew/channels/`. This tool does not install anything on a console.

The older tools/package utility creates placeholder JSON; it is not a native WAD builder and must not be used to publish installable packages.

Still required: validated application packages, storefront registration/purchase/download UI integration, complete ticket synchronization, and end-to-end Dolphin install/launch testing. Ordinary homebrew ZIPs are not Wii channel packages. The native installer must not be shown as ready until those steps are complete.

Format reference: https://github.com/OpenShopChannel/wad-synthesizer . Original channel runtime reference: https://github.com/OpenShopChannel/WSC-Patcher .
