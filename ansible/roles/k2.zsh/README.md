# k2.zsh

Installs Zsh for `root` and the managed K2 user, along with a pinned zinit and a
small server-focused subset of the `wyvernzora/dotfiles` shell ergonomics.

The role includes completion, autosuggestions, fzf-tab, syntax highlighting,
history settings, Starship, zoxide, and basic navigation aliases. It
intentionally excludes Homebrew, mise, direnv, language toolchains, and macOS
configuration. Debian 12 does not package Starship, so PVE 8 hosts use a small
built-in prompt while PVE 9 hosts use the packaged Starship binary.
