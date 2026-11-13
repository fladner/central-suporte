package main

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Páginas em bash (como o BigBashView, sem o servidor aberto dele): o .desktop de
// central.d com X-SEDUC-Bash=<nome> abre a página feita pelo script
// central.d/paginas/<nome>.sh, sem recompilar a Central.
//
//	GET  /bash/<nome>                 roda "<nome>.sh pagina" e mostra o HTML que ele imprimir
//	POST /bash/<nome>/<ação>          roda "<nome>.sh <ação>" com os campos do formulário;
//	                                  sem saída, volta para a página
//	POST /bash/<nome>/tarefa/<ação>   roda "<nome>.sh <ação>" como tarefa, com barra e saída
//	                                  ao vivo (protocolo do yad --progress, como os outros)
//
// Os campos chegam ao script em variáveis F_<campo> (só letras minúsculas, números e _),
// nunca na linha de comando. O HTML do script entra na página da Central como está: o
// script escapa o que vem do usuário (função esc do central_pagina.sh). A CSP da Central
// continua valendo, então a página não roda JavaScript próprio.
//
// Segurança: o script e a pasta paginas/ têm de ser do root (ou do próprio usuário, no
// desenvolvimento) e não graváveis por grupo e outros; o item segue o X-SEDUC-Acesso.

var (
	reNomeBash  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	reCampoBash = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

	limiteBash       = 60 * time.Second
	limiteTarefaBash = 2 * time.Hour
)

const (
	maxSaidaBash  = 1 << 20
	maxCamposBash = 60
	maxValorBash  = 8 << 10
)

// donoSeguro: arquivo ou pasta do root ou do próprio usuário, sem escrita de grupo e outros.
func donoSeguro(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode().Perm()&0o022 != 0 {
		return false
	}
	return st.Uid == 0 || int(st.Uid) == os.Getuid()
}

// scriptBash devolve o caminho do script da página, se ele existir e for seguro.
func scriptBash(itens, nome string) (string, bool) {
	if !reNomeBash.MatchString(nome) {
		return "", false
	}
	pasta := filepath.Join(itens, "paginas")
	caminho := filepath.Join(pasta, nome+".sh")
	ip, err := os.Lstat(pasta)
	if err != nil || !ip.IsDir() || !donoSeguro(ip) {
		return "", false
	}
	ia, err := os.Lstat(caminho)
	if err != nil || !ia.Mode().IsRegular() || !donoSeguro(ia) {
		return "", false
	}
	return caminho, true
}

// itemBash: o item visível para este usuário que aponta a página.
func (c *Central) itemBash(nome string) (Item, string, bool) {
	for _, it := range listarItens(c.Itens, ehAdmin(c.AdConf)) {
		if it.Bash == nome {
			if caminho, ok := scriptBash(c.Itens, nome); ok {
				return it, caminho, true
			}
		}
	}
	return Item{}, "", false
}

// ambienteBash: ambiente do script, com os campos em F_<campo> (os F_ herdados saem).
func (c *Central) ambienteBash(nome, acao string, campos url.Values) []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "F_") && !strings.HasPrefix(e, "CENTRAL_") {
			env = append(env, e)
		}
	}
	admin := "0"
	if ehAdmin(c.AdConf) {
		admin = "1"
	}
	env = append(env,
		"CENTRAL_PAGINA="+nome,
		"CENTRAL_ACAO="+acao,
		"CENTRAL_ADMIN="+admin,
		"CENTRAL_SCRIPTS="+c.Scripts,
		"CENTRAL_URL=/bash/"+nome,
	)
	chaves := make([]string, 0, len(campos))
	for k := range campos {
		if reCampoBash.MatchString(k) {
			chaves = append(chaves, k)
		}
	}
	sort.Strings(chaves)
	if len(chaves) > maxCamposBash {
		chaves = chaves[:maxCamposBash]
	}
	for _, k := range chaves {
		v := strings.ReplaceAll(strings.Join(campos[k], "\n"), "\x00", "")
		if len(v) > maxValorBash {
			v = v[:maxValorBash]
		}
		env = append(env, "F_"+k+"="+v)
	}
	return env
}

// limitado guarda no máximo n bytes e descarta o resto (o script nunca trava escrevendo).
type limitado struct {
	b bytes.Buffer
	n int
}

func (l *limitado) Write(p []byte) (int, error) {
	if falta := l.n - l.b.Len(); falta > 0 {
		if len(p) > falta {
			l.b.Write(p[:falta])
		} else {
			l.b.Write(p)
		}
	}
	return len(p), nil
}

// rodarBash roda o script (pelo bash: o build da imagem deixa central.d sem execução)
// e devolve a saída padrão, a de erro e o código.
func rodarBash(caminho, acao string, env []string) (saida, erros string, codigo int) {
	ctx, cancelar := context.WithTimeout(context.Background(), limiteBash)
	defer cancelar()
	cmd := execBash(ctx, caminho, acao)
	cmd.Env = env
	cmd.Dir = "/"
	out, errOut := &limitado{n: maxSaidaBash}, &limitado{n: 64 << 10}
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.Stdin = nil
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if err != nil {
		codigo = -1
		var saiu *exec.ExitError
		if errors.As(err, &saiu) {
			codigo = saiu.ExitCode()
		}
		if ctx.Err() == context.DeadlineExceeded {
			errOut.Write([]byte("\ntempo esgotado (" + limiteBash.String() + ")"))
		}
	}
	return out.b.String(), errOut.b.String(), codigo
}

type dadosBash struct {
	Nome    string
	HTML    template.HTML
	Falhou  bool
	Codigo  int
	Detalhe string
}

func (c *Central) mostrarBash(w http.ResponseWriter, it Item, nome string, db dadosBash, tarefa string) {
	d := c.base(it.Nome)
	d.NomePagina, d.Tarefa = nome, tarefa
	db.Nome = nome
	d.Extra = db
	c.mostrar(w, "bash.html", d)
}

func (c *Central) paginaBash(w http.ResponseWriter, r *http.Request, resto string) {
	nome, acao, _ := strings.Cut(resto, "/")
	it, caminho, ok := c.itemBash(nome)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tarefa := false
	if a, ok := strings.CutPrefix(acao, "tarefa/"); ok {
		acao, tarefa = a, true
	}
	switch {
	case acao == "" && r.Method == http.MethodGet:
		acao = "pagina"
	case acao != "" && acao != "pagina" && reNomeBash.MatchString(acao) && r.Method == http.MethodPost:
	default:
		http.NotFound(w, r)
		return
	}
	var campos url.Values
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "formulário inválido", http.StatusBadRequest)
			return
		}
		campos = r.PostForm
	} else {
		campos = r.URL.Query()
	}
	env := c.ambienteBash(nome, acao, campos)

	if tarefa {
		t := c.tarefas.Iniciar("bash/"+nome, Comando{
			Argv:   []string{bashBin, caminho, acao},
			Env:    env,
			Limite: limiteTarefaBash,
			AoVivo: true,
			Fim: func(codigo int, res map[string]string) ([][2]string, string) {
				return nil, res["aviso"]
			},
		})
		http.Redirect(w, r, "/tarefa/"+t.ID, http.StatusSeeOther)
		return
	}

	saida, erros, codigo := rodarBash(caminho, acao, env)
	if codigo == 0 && acao != "pagina" && strings.TrimSpace(saida) == "" {
		// Ação sem resposta própria: volta para a página (e um F5 não repete a ação).
		http.Redirect(w, r, "/bash/"+nome, http.StatusSeeOther)
		return
	}
	db := dadosBash{HTML: template.HTML(saida)}
	if codigo != 0 {
		db.Falhou, db.Codigo = true, codigo
		db.Detalhe = strings.TrimSpace(ultimasLinhas(erros, 30))
	}
	c.mostrarBash(w, it, nome, db, "")
}

// tarefaBash mostra o andamento de uma tarefa de página em bash (Pagina "bash/<nome>").
func (c *Central) tarefaBash(w http.ResponseWriter, r *http.Request, t *Tarefa) bool {
	nome, ok := strings.CutPrefix(t.Pagina, "bash/")
	if !ok {
		return false
	}
	it, _, ok := c.itemBash(nome)
	if !ok {
		http.NotFound(w, r)
		return true
	}
	c.mostrarBash(w, it, nome, dadosBash{}, t.ID)
	return true
}

func ultimasLinhas(s string, n int) string {
	linhas := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(linhas) > n {
		linhas = linhas[len(linhas)-n:]
	}
	return strings.Join(linhas, "\n")
}

var bashBin = "/bin/bash"

func execBash(ctx context.Context, caminho, acao string) *exec.Cmd {
	return exec.CommandContext(ctx, bashBin, caminho, acao)
}
