# Interface references

The frontend code in page.html, shop.css and shop.js is a Shroombrew implementation for the existing Go server. It uses the native Wii Shop Channel JavaScript objects, not emulated audio or a replacement channel executable.

Native API references: https://docs.oscwii.org/wii-shop-channel and https://github.com/OpenShopChannel/WSC-Web .

The small interface images in shop/assets are preserved Wii Shop Channel artwork from https://github.com/WiiMart/WiiMart, commit 1ed60c77cfeb26cfa787fe4dbef3e081ece3c208, path www/oss/oss/common/images/banner. These original third-party graphics retain their original owners' rights and are not relicensed under the Go server's AGPL license. Nintendo catalog packages, third-party server code and credentials are not included.

Music and sound effects are played by the user's installed Wii Shop Channel through wiiSound.playBGM and playSE. No music file is hosted or included in this source archive.

## Public reference feature inventory

On 2026-09-12, the public WiiShopChannel.net shell and its directly linked
resources were inspected as behavioral reference material. The site is a static
608x456 Shop recreation with a boot ring, connecting state, welcome/recommended
titles screen, Shop home, browse/list/detail pages, search keyboard, points,
account history, settings, help and error/report screens. It also provides
cursor and sound preferences, region/filter state, and a separate Mario-style
download sequence.

Shroombrew will implement equivalent flows using its own Go handlers, catalog,
state and native Shop client APIs. The crawl output is deliberately excluded
from this source tree and is not used as source or asset input.
