// Package tech infers technologies from commands and classifies noise.
package tech

import (
	"sort"

	"github.com/ManuelLuchage/apura/internal/cmdparse"
)

// programTech maps a program name to a technology tag. Extend freely; keep
// tags lowercase and stable because they are stored and searched.
var programTech = map[string]string{
	// containers & orchestration
	"docker": "docker", "docker-compose": "docker", "podman": "podman", "buildah": "podman",
	"kubectl": "kubernetes", "k9s": "kubernetes", "kubectx": "kubernetes", "kubens": "kubernetes",
	"helm": "helm", "k3s": "k3s", "k3d": "k3s", "kind": "kubernetes", "minikube": "kubernetes",
	"nerdctl": "containerd", "ctr": "containerd", "crictl": "kubernetes",
	// infra / cloud
	"terraform": "terraform", "tofu": "terraform", "ansible": "ansible", "ansible-playbook": "ansible",
	"packer": "packer", "vagrant": "vagrant", "pulumi": "pulumi",
	"aws": "aws", "gcloud": "gcp", "gsutil": "gcp", "az": "azure", "hcloud": "hetzner",
	"doctl": "digitalocean", "flyctl": "fly", "fly": "fly", "vercel": "vercel", "wrangler": "cloudflare",
	// system
	"systemctl": "systemd", "journalctl": "systemd", "systemd-analyze": "systemd",
	"apt": "apt", "apt-get": "apt", "dpkg": "apt", "dnf": "dnf", "yum": "dnf", "rpm": "dnf",
	"pacman": "pacman", "yay": "pacman", "paru": "pacman", "apk": "apk", "zypper": "zypper",
	"brew": "homebrew", "nix": "nix", "nix-shell": "nix", "nixos-rebuild": "nix", "snap": "snap", "flatpak": "flatpak",
	"ufw": "firewall", "iptables": "firewall", "nft": "firewall", "firewall-cmd": "firewall",
	"ssh": "ssh", "scp": "ssh", "ssh-keygen": "ssh", "ssh-copy-id": "ssh", "rsync": "rsync",
	"nginx": "nginx", "caddy": "caddy", "apachectl": "apache", "certbot": "letsencrypt",
	"crontab": "cron", "mount": "storage", "lsblk": "storage", "mkfs": "storage", "fdisk": "storage", "zfs": "zfs",
	"openssl": "openssl", "gpg": "gpg", "wg": "wireguard", "wg-quick": "wireguard", "tailscale": "tailscale",
	// languages & toolchains
	"go": "go", "gofmt": "go", "cargo": "rust", "rustc": "rust", "rustup": "rust",
	"python": "python", "python3": "python", "pip": "python", "pip3": "python", "pipx": "python",
	"uv": "python", "poetry": "python", "pytest": "python",
	"node": "node", "npm": "node", "npx": "node", "pnpm": "node", "yarn": "node", "bun": "bun", "deno": "deno",
	"java": "java", "mvn": "maven", "gradle": "gradle", "dotnet": "dotnet",
	"ruby": "ruby", "gem": "ruby", "bundle": "ruby", "rails": "rails",
	"php": "php", "composer": "php", "elixir": "elixir", "mix": "elixir",
	"gcc": "c", "clang": "c", "make": "make", "cmake": "cmake",
	// data
	"psql": "postgres", "pg_dump": "postgres", "pg_restore": "postgres", "createdb": "postgres",
	"mysql": "mysql", "mysqldump": "mysql", "mariadb": "mysql", "redis-cli": "redis",
	"mongosh": "mongodb", "mongo": "mongodb", "sqlite3": "sqlite",
	// vcs & misc
	"git": "git", "gh": "github", "glab": "gitlab", "jq": "jq", "yq": "yq", "curl": "http", "wget": "http",
	"fish": "fish", "zsh": "zsh", "bash": "bash",
}

// noise are read-only / navigation programs that a distilled run does not need.
// `cd` is noise because each step records its working directory and the
// renderer emits `cd` itself when the directory changes.
var noise = map[string]bool{
	"ls": true, "ll": true, "la": true, "l": true, "lsd": true, "eza": true, "exa": true, "tree": true,
	"pwd": true, "cd": true, "pushd": true, "popd": true, "z": true, "zoxide": true,
	"clear": true, "reset": true, "history": true, "exit": true,
	"cat": true, "bat": true, "less": true, "more": true, "head": true, "tail": true,
	"echo": true, "printf": true, "which": true, "type": true, "whereis": true, "whoami": true,
	"man": true, "tldr": true, "help": true, "date": true, "uptime": true,
	"top": true, "htop": true, "btop": true, "df": true, "du": true, "free": true, "ps": true,
	"grep": true, "rg": true, "fd": true, "find": true,
}

// Infer returns the sorted, de-duplicated technologies used by a command line.
func Infer(line string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range cmdparse.Programs(line) {
		if t, ok := programTech[p]; ok && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// IsNoise reports whether every program of the line is read-only/navigation.
// Lines with no recognizable program are not noise.
func IsNoise(line string) bool {
	ps := cmdparse.Programs(line)
	if len(ps) == 0 {
		return false
	}
	for _, p := range ps {
		if !noise[p] {
			return false
		}
	}
	return true
}
