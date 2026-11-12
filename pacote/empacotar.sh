#!/usr/bin/env bash

# Gera o pacote de entrega da Central do Suporte em web para a imagem Xubuntu SEDUC:
#   out/seduc-central-suporte_<versão>_amd64.deb   (vai para debs/ do repositório da imagem)
#   out/central-suporte-imagem-<versão>.tar.gz      (.deb + incorporar.sh + patches + LEIAME)
# Compila em Docker (golang), sem rede. Uso: pacote/empacotar.sh <versão>   ex.: 1.0.0

set -euo pipefail

#####################
## VARIABLE GLOBAL ##
#####################
PROJETO="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)"
VERSAO="${1:?informe a versão, ex.: 1.0.0}"
GO_IMAGEM="${GO_IMAGEM:-golang@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59}"
SAIDA="$PROJETO/out"
DEB="$SAIDA/seduc-central-suporte_${VERSAO}_amd64.deb"
ENTREGA="central-suporte-imagem-$VERSAO"

[[ "$VERSAO" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Versão inválida: $VERSAO" >&2; exit 1; }

##########
## MAIN ##
##########
mkdir -p "$SAIDA"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> Testando e compilando $VERSAO"
docker run --rm --network none -v "$PROJETO:/src:ro" -v "$TMP:/saida" -w /src -e CGO_ENABLED=0 \
    -e GOCACHE=/tmp/cache "$GO_IMAGEM" sh -c \
    "test -z \"\$(gofmt -l .)\" && go vet ./... && go test ./... && \
     go build -trimpath -ldflags '-s -w -X main.Versao=$VERSAO' -o /saida/central-suporte ."

echo "==> Montando o .deb"
RAIZ="$TMP/raiz"
install -D -m 755 "$TMP/central-suporte" "$RAIZ/usr/bin/central-suporte"
install -D -m 644 "$PROJETO/README.md" "$RAIZ/usr/share/doc/seduc-central-suporte/README.md"
TAMANHO="$(du -sk "$RAIZ" | cut -f1)"
install -d -m 755 "$RAIZ/DEBIAN"
sed -e "s/@VERSAO@/$VERSAO/" -e "s/@TAMANHO@/$TAMANHO/" "$PROJETO/pacote/control" > "$RAIZ/DEBIAN/control"
fakeroot dpkg-deb --build -Zxz "$RAIZ" "$DEB" >/dev/null
dpkg-deb --info "$DEB" | sed -n '/Package:/,/Description:/p'

echo "==> Montando o pacote de entrega"
E="$TMP/$ENTREGA"
install -D -m 644 "$DEB" "$E/debs/$(basename "$DEB")"
install -D -m 755 "$PROJETO/pacote/incorporar.sh" "$E/incorporar.sh"
install -D -m 644 "$PROJETO/pacote/LEIAME.txt" "$E/LEIAME.txt"
for P in "$PROJETO"/imagem/*.patch; do
    install -D -m 644 "$P" "$E/patches/$(basename "$P")"
done
sed -i "s/@VERSAO_CENTRAL@/$VERSAO/g" "$E/LEIAME.txt" "$E/incorporar.sh"
( cd "$E" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS )
tar -C "$TMP" -czf "$SAIDA/$ENTREGA.tar.gz" "$ENTREGA"
( cd "$SAIDA" && sha256sum "$(basename "$DEB")" "$ENTREGA.tar.gz" )
echo "Pronto: $SAIDA/$ENTREGA.tar.gz"
