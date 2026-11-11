package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Item é um ícone da Central, lido de um .desktop de /usr/local/seduc/central.d.
// As regras são as do central_suporte.sh (yad):
//   - X-SEDUC-Acesso=admin: só para administradores (grupo sudo e GRUPOS_ADMIN do ad.conf);
//   - TryExec: o item some quando o programa não existe;
//   - ordem pelo nome do arquivo.
//
// Novo: X-SEDUC-Web=<página> abre a página web da própria Central em vez do Exec.
// O central_suporte.sh ignora a chave, então o mesmo .desktop serve para as duas.
type Item struct {
	ID        string // nome do arquivo sem .desktop (10-boas-vindas)
	Nome      string
	Descricao string
	Exec      string
	Icone     string
	Acesso    string
	TryExec   string
	Web       string
}

func lerDesktop(caminho string) (Item, bool) {
	f, err := os.Open(caminho)
	if err != nil {
		return Item{}, false
	}
	defer f.Close()
	it := Item{ID: strings.TrimSuffix(filepath.Base(caminho), ".desktop")}
	grupo := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		l := strings.TrimSpace(s.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "[") {
			grupo = l
			continue
		}
		if grupo != "[Desktop Entry]" {
			continue
		}
		chave, valor, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(chave) {
		case "Name":
			it.Nome = valor
		case "Comment":
			it.Descricao = valor
		case "Exec":
			it.Exec = valor
		case "Icon":
			it.Icone = valor
		case "X-SEDUC-Acesso":
			it.Acesso = valor
		case "TryExec":
			it.TryExec = valor
		case "X-SEDUC-Web":
			it.Web = valor
		}
	}
	return it, it.Nome != "" && (it.Exec != "" || it.Web != "")
}

// listarItens devolve os itens que este usuário pode ver, na ordem do nome do arquivo.
func listarItens(pasta string, admin bool) []Item {
	arquivos, _ := filepath.Glob(filepath.Join(pasta, "*.desktop"))
	sort.Strings(arquivos)
	var itens []Item
	for _, a := range arquivos {
		it, ok := lerDesktop(a)
		if !ok {
			continue
		}
		if it.Acesso == "admin" && !admin {
			continue
		}
		if it.TryExec != "" && !executavel(it.TryExec) {
			continue
		}
		if it.Web != "" && paginas[it.Web] == nil {
			// Página que esta versão da Central não conhece: usa o Exec, se houver.
			if it.Exec == "" {
				continue
			}
			it.Web = ""
		}
		itens = append(itens, it)
	}
	return itens
}

func executavel(caminho string) bool {
	if !filepath.IsAbs(caminho) {
		_, err := exec.LookPath(caminho)
		return err == nil
	}
	st, err := os.Stat(caminho)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// argumentosExec separa a linha Exec (aspas duplas simples) e tira os códigos %f, %U...
func argumentosExec(linha string) []string {
	var args []string
	var atual strings.Builder
	aspas, tem := false, false
	for _, r := range linha {
		switch {
		case r == '"':
			aspas, tem = !aspas, true
		case (r == ' ' || r == '\t') && !aspas:
			if tem {
				args = append(args, atual.String())
				atual.Reset()
				tem = false
			}
		default:
			atual.WriteRune(r)
			tem = true
		}
	}
	if tem {
		args = append(args, atual.String())
	}
	var limpos []string
	for _, a := range args {
		if len(a) == 2 && a[0] == '%' {
			continue
		}
		limpos = append(limpos, a)
	}
	return limpos
}

// ehAdmin repete o eh_admin do central_suporte.sh: grupo sudo ou um dos GRUPOS_ADMIN do
// ad.conf (lido pelo bash, como no script), comparando sem diferença de maiúsculas e sem
// o prefixo "DOMINIO\" dos grupos do AD.
func ehAdmin(adConf string) bool {
	admin := []string{"sudo"}
	if _, err := os.Stat(adConf); err == nil {
		saida, err := exec.Command("bash", "-c", `GRUPOS_ADMIN=(); . "$1" >/dev/null 2>&1; printf '%s\0' "${GRUPOS_ADMIN[@]}"`, "_", adConf).Output()
		if err == nil {
			for _, g := range strings.Split(string(saida), "\x00") {
				if g != "" {
					admin = append(admin, g)
				}
			}
		}
	}
	saida, err := exec.Command("id", "-nGz").Output()
	if err != nil {
		return false
	}
	for _, g := range strings.Split(string(saida), "\x00") {
		if i := strings.LastIndex(g, `\`); i >= 0 {
			g = g[i+1:]
		}
		for _, a := range admin {
			if g != "" && strings.EqualFold(g, a) {
				return true
			}
		}
	}
	return false
}
