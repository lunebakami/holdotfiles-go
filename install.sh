#!/bin/sh
# Instala a partir do checkout, sem sudo e sem modificar arquivos de shell.
set -eu

usage() {
  printf '%s\n' 'Uso: sh install.sh [--prefix DIRETORIO]' \
    'Padrão: ~/.local (executável em ~/.local/bin/hdt)' \
    'Requer Go 1.24.1+; configuração existente é preservada.'
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
command -v go >/dev/null 2>&1 || {
  printf '%s\n' 'Instale Go 1.24.1+ (https://go.dev/dl/) e execute novamente.' >&2
  exit 1
}
source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
[ -f "$source_dir/go.mod" ] || { printf '%s\n' 'Execute o instalador dentro de um clone do repositório.' >&2; exit 1; }
bin_dir="$prefix/bin"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/holdotfiles"
mkdir -p "$bin_dir"
install_tmp=$(mktemp -d "$bin_dir/.hdt-install-XXXXXX")
cleanup() { rm -f "$install_tmp/hdt"; rmdir "$install_tmp"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf '%s\n' 'Compilando Holdotfiles...'
(cd "$source_dir" && go build -trimpath -o "$install_tmp/hdt" ./cmd/hdt)
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
