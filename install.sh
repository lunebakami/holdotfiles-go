#!/bin/sh
# Instala a versão publicada ou o checkout local, sem sudo nem alterar o shell.
set -eu

usage() {
  printf '%s\n' 'Uso: sh install.sh [--prefix DIRETORIO]' \
    'Ou: curl -fsSL https://raw.githubusercontent.com/lunebakami/holdotfiles-go/main/install.sh | sh' \
    'Padrão: ~/.local (executável em ~/.local/bin/hdt)' \
    'Modo curl requer curl, tar, sha256sum e Linux amd64/arm64; checkout requer Go 1.24.1+.'
}

prefix="${HOME}/.local"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --prefix)
      [ "$#" -ge 2 ] || { usage >&2; exit 1; }
      prefix=$2
      shift 2
      ;;
    --help|-h) usage; exit 0 ;;
    *) usage >&2; exit 1 ;;
  esac
done
case "$prefix" in /*) ;; *) printf '%s\n' 'O prefixo deve ser um caminho absoluto.' >&2; exit 1 ;; esac
install_tmp=
download_tmp=
source_dir=
local_build=0
cleanup() {
  if [ -n "$install_tmp" ]; then
    rm -f "$install_tmp/hdt"
    rmdir "$install_tmp"
  fi
  if [ -n "$download_tmp" ]; then rm -rf "$download_tmp"; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd || true)
if [ "${0##*/}" != sh ] && [ "${0##*/}" != bash ] && [ -n "$script_dir" ] && [ -f "$script_dir/go.mod" ]; then
  source_dir=$script_dir
  local_build=1
  command -v go >/dev/null 2>&1 || {
    printf '%s\n' 'Instale Go 1.24.1+ (https://go.dev/dl/) para compilar o checkout.' >&2
    exit 1
  }
else
  command -v curl >/dev/null 2>&1 || { printf '%s\n' 'O modo curl | sh requer curl.' >&2; exit 1; }
  command -v tar >/dev/null 2>&1 || { printf '%s\n' 'O modo curl | sh requer tar.' >&2; exit 1; }
  command -v sha256sum >/dev/null 2>&1 || { printf '%s\n' 'Instale coreutils (sha256sum) para verificar o download.' >&2; exit 1; }
  case "$(uname -s):$(uname -m)" in
    Linux:x86_64) asset_arch=amd64 ;;
    Linux:aarch64|Linux:arm64) asset_arch=arm64 ;;
    *) printf '%s\n' 'A release pronta oferece Linux amd64 e arm64.' >&2; exit 1 ;;
  esac
  download_tmp=$(mktemp -d "${TMPDIR:-/tmp}/holdotfiles-release-XXXXXX")
  archive="hdt-linux-$asset_arch.tar.gz"
  release_url="https://github.com/lunebakami/holdotfiles-go/releases/latest/download"
  printf 'Baixando Holdotfiles para Linux %s...\n' "$asset_arch"
  curl -fsSL "$release_url/$archive" -o "$download_tmp/$archive"
  curl -fsSL "$release_url/SHA256SUMS" -o "$download_tmp/SHA256SUMS"
  checksum_line=$(grep "  $archive\$" "$download_tmp/SHA256SUMS" || true)
  [ -n "$checksum_line" ] || { printf '%s\n' 'O checksum do arquivo não foi encontrado.' >&2; exit 1; }
  printf '%s\n' "$checksum_line" | (cd "$download_tmp" && sha256sum -c -)
  tar -xzf "$download_tmp/$archive" -C "$download_tmp"
  source_dir=$download_tmp
fi
bin_dir="$prefix/bin"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/holdotfiles"
mkdir -p "$bin_dir"
install_tmp=$(mktemp -d "$bin_dir/.hdt-install-XXXXXX")
if [ "$local_build" -eq 1 ]; then
  printf '%s\n' 'Compilando checkout local...'
  (cd "$source_dir" && go build -trimpath -o "$install_tmp/hdt" ./cmd/hdt)
else
  cp "$source_dir/hdt" "$install_tmp/hdt"
fi
chmod 755 "$install_tmp/hdt"
[ ! -d "$bin_dir/hdt" ] || { printf '%s\n' 'O destino hdt é um diretório.' >&2; exit 1; }
mv -f "$install_tmp/hdt" "$bin_dir/hdt"
(
  umask 077
  mkdir -p "$config_dir"
  if [ ! -e "$config_dir/.env" ] && [ ! -L "$config_dir/.env" ]; then
    cp "$source_dir/.env.example" "$config_dir/.env"
  fi
  if [ ! -e "$config_dir/paths.example" ] && [ ! -L "$config_dir/paths.example" ]; then
    cp "$source_dir/.hdtconfig.example" "$config_dir/paths.example"
  fi
)
printf 'Instalado: %s/hdt\nConfiguração: %s/.env\n' "$bin_dir" "$config_dir"
printf '%s\n' 'Preencha as credenciais e configure ~/.hdtconfig para enviar backups.'
case ":$PATH:" in
  *":$bin_dir:"*) printf '%s\n' 'Execute hdt para abrir a interface.' ;;
  *) printf 'Adicione ao PATH no seu shell: export PATH="%s:$PATH"\n' "$bin_dir" ;;
esac
