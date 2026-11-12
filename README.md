# Central do Suporte SEDUC (web)

A Central do Suporte do Xubuntu SEDUC (imagem 1.4-1 em diante) numa janela web, no
estilo do BigLinux: as ferramentas viram páginas, mas o trabalho continua nos scripts
da imagem (`/usr/local/seduc/scripts`), com o mesmo `pkexec` deles.

Substitui a primeira versão deste repositório (Flask), que ficou no histórico do Git.

## Como funciona

- Um programa em Go, sem dependências (`central-suporte`), roda como o próprio usuário.
- Abre um servidor só em `127.0.0.1`, numa porta livre, e uma janela de aplicativo do
  Chrome da imagem (`--app`, sem barra de endereço). Sem Chrome, abre no navegador padrão.
- Os itens são os mesmos do `central_suporte.sh`: um `.desktop` por item em
  `/usr/local/seduc/central.d`, na ordem do nome do arquivo, com as mesmas regras:
  - `X-SEDUC-Acesso=admin`: só para o grupo sudo e os `GRUPOS_ADMIN` do `ad.conf`;
  - `TryExec`: o item some quando o programa não existe.
- Novo: `X-SEDUC-Web=<página>` abre a página web da Central. O `central_suporte.sh` (yad)
  ignora a chave, então o mesmo `.desktop` serve para as duas centrais. Itens sem a chave
  continuam abrindo o programa do `Exec` (yad), o que permite migrar um item por vez.
- Novo: `X-SEDUC-Mesma-Janela=true` para programas com página web própria, como a tela do
  VDE-CE. A Central os chama com `--sem-navegador --voltar <endereço da Central>`, lê na saída
  a linha `<nome>=http://127.0.0.1:<porta>/...` e leva a mesma janela para lá; o botão de volta
  do programa traz a janela de volta. Use a chave só quando o programa do `Exec` aceitar essas
  opções (no VDE-CE, o agente Go; o 0.7.3 em Python abre o yad).
- Ícones do tema da imagem (Papirus, depois elementary-xfce, Adwaita e hicolor).
- Fechar a janela encerra a Central em cerca de um minuto; parada, em 15 minutos.

## Segurança

- A janela entra por um endereço de uso único (`/entrar/<chave>`), trocado por um
  cookie `HttpOnly` e `SameSite=Strict`; a chave não serve uma segunda vez.
- `Host` conferido em todos os pedidos e `Origin` em todos os POST.
- CSP sem nada de fora (`default-src 'none'`, scripts e estilos só da própria Central).
- Só abre itens de `central.d` (pelo nome do arquivo) e tarefas das páginas conhecidas.
- O que precisa de root continua nos scripts da imagem, com a senha pedida pelo `pkexec`.

## Páginas web

Cada página chama o script da imagem com `--sem-janela`. O script escreve o andamento na
saída padrão no formato do `yad --progress` (`# texto` e percentual) e o resultado em
linhas `RESULTADO_<NOME>=valor`.

| Página | Script | Situação |
|---|---|---|
| `coletar-logs` | `coletar_logs.sh --sem-janela` | pronta |
| `tokens` | `painel_tokens.sh --listar` e `pkexec instalar_tokens.sh` | pronta |
| `ad` e `impressoras` | `painel_ad.sh --listar` e `pkexec ingressar_ad.sh` (senha do AD pela entrada padrão); `painel_impressora_ad.sh --listar/--kinit/--padrao` e `pkexec instalar_impressora_ad.sh` | prontas |
| `atualizar` | `pkexec atualizar_sistema_root.sh` (sem mudança no script): etapa pelas linhas `==> `, saída ao vivo, reiniciar só com `/var/run/reboot-required` | pronta |
| `boas-vindas` | `welcome.sh --listar` e `pkexec criar_usuarios.sh`; no primeiro login o `welcome.sh` abre `central-suporte --abrir boas-vindas` (se instalada) | pronta |

## Compilar e testar

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.25-alpine sh -c 'go vet ./... && go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags "-X main.Versao=0.1.0" -o central-suporte .'
```

Opções: `--sem-janela` (só mostra o endereço de entrada), `--abrir PÁGINA`, `--porta N`, `--itens DIR`,
`--scripts DIR`, `--ad-conf ARQ`, `-version`.
