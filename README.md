# Wii Shroombrew Channel

A Wii Shop Channel revival, rebuilt for homebrew. Under development. Nothing here is functional yet.

## What happened to RaspberrySoft

RaspberrySoft was announced and never shipped. The plan kept shifting, progress stalled, and eventually it went quiet without an explanation. That was a failure to follow through, and if you were waiting on it, you deserved better. This project is its replacement, built in the open from day one.

## The name

Shomebrew sounded like a mushroom, and the name stuck. Like the item it's named after, the goal is to make the console more than it was out of the box.

## What this is

Nintendo shut the Wii Shop Channel down in 2019. This project brings that experience back: the same channel, the same look and feel, patched to talk to independent servers instead of Nintendo's. You open it on your Wii, browse the catalog, download something, and it appears on your system menu ready to launch.

At launch the catalog is homebrew: applications, emulators, utilities, and games made by the community. WiiWare and other Nintendo-channel content is a maybe for later. Projects like WiiLink's Wii Room, WiiMart, and others have shown there is interest in that side of the Shop Channel experience, and this project keeps the door open to it down the line, handled the same careful way those projects handle it.

The current code is based on WiiSOAP, the server implementation by the Open Shop Channel team that speaks the Wii Shop Channel protocol. Their work is the reason any of this is possible, and their license (AGPLv3, see LICENSE) carries over to this project. The module path has been updated to this repository, but the foundation is theirs.

## What to expect

When this works, expect the full Shop Channel loop:

- Open the channel on a real Wii and browse titles by category, the way the original worked
- Title pages with descriptions, ratings, and version info before you download
- One-press downloads that install a forwarder channel straight to the system menu
- A download history tied to your console, so anything you grabbed before can be redownloaded later
- Update notifications when a new version of something you own is published
- Search, rankings, and new release listings, matching the sections the original frontend had

None of that exists yet. Right now this is the server skeleton being reworked. But that list is the bar for calling this done.

## What this is not

- Not a paid store. No Wii Points, no prices, no password accounts. Everything is free.
- Not a redesign. The original interface is the point. It stays as close to the 2008 experience as possible, minus the parts that required Nintendo's billing servers.

## Status

The UI is partly finished but i decided to focus on the downloading mechanism first instead of the UI so i dont waste as much time trying to design something ill probably mess up later.

- Mark

Note to the Open Shop Channel team, if you're reading: thank you. You kept the Shop Channel alive after Nintendo shut it down. This project builds on the foundation you left behind.
