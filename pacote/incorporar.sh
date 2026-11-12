#!/usr/bin/env bash

# Incorpora a Central do Suporte em web (seduc-central-suporte @VERSAO_CENTRAL@) no
# repositório da imagem Xubuntu SEDUC 26.04, seguindo as regras do CLAUDE.md de lá:
#   - debs/seduc-central-suporte_*.deb (o build.sh instala todos os .deb de debs/);
#   - patches dos scripts da Central em overlay/usr/local/seduc/scripts;
#   - itens de overlay/usr/local/seduc/central.d com X-SEDUC-Web (o yad ignora a chave);
#   - versão nova, changelog, SCRIPT_VERSION dos scripts alterados e
#     config/arquivos-atualizador.txt.
# Sem o pacote instalado, tudo continua no yad: as máquinas que recebem só os scripts
# pelo seduc_setup.sh não quebram.
#
# Uso: ./incorporar.sh <pasta do repositório da imagem> [versão nova]
#      versão nova: padrão X.(Y+1)-0 a partir da atual (ex.: 1.4-1 -> 1.5-0)
# Não faz commit, push nem publicação: confira com "git diff" e siga o LEIAME.

set -euo pipefail

#####################
## VARIABLE GLOBAL ##
#####################
AQUI="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"
REPO="$(cd "${1:?informe a pasta do repositório da imagem}" && pwd)"
SEDUC="$REPO/overlay/usr/local/seduc"
SCRIPTS="$SEDUC/scripts"
ITENS="$SEDUC/central.d"
ATUALIZADOR="$REPO/config/arquivos-atualizador.txt"

# Scripts alterados pelos patches (recebem SCRIPT_VERSION novo)
ALTERADOS=(central_suporte.sh coletar_logs.sh welcome.sh painel_tokens.sh instalar_tokens.sh
           painel_ad.sh painel_impressora_ad.sh)

# Item da Central -> página web
declare -A PAGINAS=(
    [10]=boas-vindas [20]=atualizar [30]=ad [40]=impressoras [50]=tokens [90]=coletar-logs
)

###############
## FUNCTIONS ##
###############
falhar() { echo "ERRO: $*" >&2; exit 1; }

proxima_versao() {
    local X Y
    IFS='.-' read -r X Y _ <<< "$1"
    echo "$X.$((Y + 1))-0"
}

##########
## MAIN ##
##########
[[ -f "$SEDUC/version" && -x "$REPO/build.sh" && -d "$ITENS" ]] \
    || falhar "$REPO não parece o repositório da imagem (falta overlay/usr/local/seduc/version, build.sh ou central.d)"
grep -q '^SCRIPT_VERSION=' "$SCRIPTS/central_suporte.sh" \
    || falhar "a imagem ainda não tem a Central do Suporte (1.4-0 ou mais nova)"
if grep -q 'command -v central-suporte' "$SCRIPTS/central_suporte.sh"; then
    falhar "a Central web já foi incorporada neste repositório"
fi

ATUAL="$(tr -d '[:space:]' < "$SEDUC/version")"
NOVA="${2:-$(proxima_versao "$ATUAL")}"
[[ "$NOVA" =~ ^[0-9]+\.[0-9]+-[0-9]+$ ]] || falhar "versão inválida: $NOVA"
echo "Repositório: $REPO"
echo "Versão:      $ATUAL -> $NOVA"
if git -C "$REPO" rev-parse --git-dir >/dev/null 2>&1 && [[ -n "$(git -C "$REPO" status --porcelain)" ]]; then
    echo "AVISO: há mudanças não commitadas no repositório; elas continuam lá."
fi

echo "==> Conferindo os patches"
for P in "$AQUI"/patches/*.patch; do
    patch -d "$REPO" -p1 --forward --dry-run --quiet < "$P" >/dev/null \
        || falhar "o patch $(basename "$P") não se aplica (o script foi mudado na imagem?)"
done

echo "==> Aplicando os patches"
for P in "$AQUI"/patches/*.patch; do
    patch -d "$REPO" -p1 --forward --quiet --no-backup-if-mismatch < "$P"
    echo "    $(basename "$P")"
done

echo "==> SCRIPT_VERSION $NOVA nos scripts alterados"
for S in "${ALTERADOS[@]}"; do
    sed -i "0,/^SCRIPT_VERSION=\".*\"/s//SCRIPT_VERSION=\"$NOVA\"/" "$SCRIPTS/$S"
    bash -n "$SCRIPTS/$S" || falhar "sintaxe: $S"
done

echo "==> Itens da Central (central.d)"
for N in "${!PAGINAS[@]}"; do
    ITEM="$(ls "$ITENS/$N"-*.desktop 2>/dev/null | head -n1)"
    [[ -n "$ITEM" ]] || { echo "    AVISO: sem item $N-*.desktop"; continue; }
    grep -q '^X-SEDUC-Web=' "$ITEM" || echo "X-SEDUC-Web=${PAGINAS[$N]}" >> "$ITEM"
    echo "    $(basename "$ITEM"): X-SEDUC-Web=${PAGINAS[$N]}"
done
# Nome visível do VDE-CE (o programa continua o mesmo netmonitor-agent-gui)
ITEM="$(ls "$ITENS"/60-*.desktop 2>/dev/null | head -n1)"
if [[ -n "$ITEM" ]]; then
    sed -i -e 's/^Name=.*/Name=VDE-CE/' \
        -e 's/^Comment=.*/Comment=Velocímetro Digital Escolar: status e provisionamento do agente/' "$ITEM"
    echo "    $(basename "$ITEM"): Name=VDE-CE"
fi

echo "==> Pacote em debs/"
cp -f "$AQUI"/debs/seduc-central-suporte_*.deb "$REPO/debs/"
ls "$REPO"/debs/seduc-central-suporte_*.deb | sed 's/^/    /'

echo "==> Versão, changelog e lista do atualizador"
echo "$NOVA" > "$SEDUC/version"
DATA="$(date +%d/%m/%Y)"
{
    echo "## $NOVA - $DATA"
    echo "    - Central do Suporte em web (pacote seduc-central-suporte @VERSAO_CENTRAL@, em debs/):"
    echo "      janela de aplicativo do Chrome no lugar do yad, com as ferramentas na mesma"
    echo "      janela e botão de volta; sem o pacote, ou com --yad, continua no yad"
    echo "    - Páginas web de Boas-vindas e usuários, Atualizar o sistema, Active Directory,"
    echo "      Impressoras SEDUC, Instalador de Tokens e Coletar logs; o trabalho continua"
    echo "      nos mesmos scripts root, pelo pkexec (itens com X-SEDUC-Web em central.d)"
    echo "    - Modos para a Central web: --listar em welcome.sh, painel_tokens.sh,"
    echo "      painel_ad.sh e painel_impressora_ad.sh; --kinit e --padrao em"
    echo "      painel_impressora_ad.sh; --sem-janela em coletar_logs.sh"
    echo "    - Primeiro boot abre as Boas-vindas na Central web quando ela está instalada"
    echo "    - Instalador de Tokens: Assinador SERPRO pelo instalador oficial 4.6.0 (o"
    echo "      repositório APT do SERPRO não responde), sem iniciar com o sistema; tira a"
    echo "      fonte APT antiga do SERPRO"
    echo "    - Item SEDUC NetMonitor da Central passa a se chamar VDE-CE"
    echo
    cat "$SEDUC/changelog"
} > "$SEDUC/changelog.novo"
mv "$SEDUC/changelog.novo" "$SEDUC/changelog"

if ! grep -q 'Central do Suporte em web' "$ATUALIZADOR"; then
    {
        echo
        echo "# Central do Suporte em web ($NOVA): scripts com os modos da Central web; o"
        echo "# programa central-suporte vem do pacote seduc-central-suporte (debs/)"
        for S in painel_tokens.sh instalar_tokens.sh painel_ad.sh painel_impressora_ad.sh; do
            grep -qx "usr/local/seduc/scripts/$S" "$ATUALIZADOR" || echo "usr/local/seduc/scripts/$S"
        done
    } >> "$ATUALIZADOR"
fi

echo
echo "Pronto: versão $NOVA. Confira com: git -C \"$REPO\" diff --stat"
echo "Depois: ./build.sh para a ISO; seduc_setup.sh só quando for publicar (veja o LEIAME)."
