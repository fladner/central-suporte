package main

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

// Pagina é uma ferramenta da Central feita em web (X-SEDUC-Web=<nome> no .desktop).
// A página só mostra e conduz; o trabalho continua nos scripts da imagem, chamados com
// --sem-janela ou direto o script root via pkexec (andamento no protocolo do yad
// --progress, ver tarefas.go).
type Pagina struct {
	Titulo string
	Modelo string // template de web/
	// Dados para o formulário da página (opcional), em .Extra no template.
	Dados func(c *Central) any
	// Iniciar confere o formulário e devolve o comando da tarefa. Um erro volta para o
	// formulário com a mensagem.
	Iniciar func(c *Central, r *http.Request) (Comando, error)
	// Acoes depois do fim: nome -> função (recebe o resultado da tarefa).
	Acoes map[string]func(c *Central, resultado map[string]string) error
	// Formularios: ações rápidas sem tarefa (POST /pagina/<nome>/<ação>); um erro volta
	// para o formulário com a mensagem, o sucesso volta para a página.
	Formularios map[string]func(c *Central, r *http.Request) error
}

type Comando struct {
	Argv   []string
	Limite time.Duration
	Fim    AoTerminar
	// Entrada vai para a entrada padrão do comando (senha), nunca para o argv.
	Entrada string
}

// errFormulario é um erro de formulário que a página mostra como está.
type errFormulario string

func (e errFormulario) Error() string { return string(e) }

var paginas = map[string]*Pagina{
	"coletar-logs": {
		Titulo: "Coletar logs para o suporte",
		Modelo: "coletar-logs.html",
		Iniciar: func(c *Central, r *http.Request) (Comando, error) {
			return Comando{Argv: []string{filepath.Join(c.Scripts, "coletar_logs.sh"), "--sem-janela"}, Limite: 10 * time.Minute}, nil
		},
		Acoes: map[string]func(*Central, map[string]string) error{
			// Abre a pasta onde ficou o arquivo (só a pasta do resultado da tarefa).
			"abrir-pasta": func(c *Central, res map[string]string) error {
				arquivo := res["arquivo"]
				if arquivo == "" || !filepath.IsAbs(arquivo) {
					return errSemResultado
				}
				return exec.Command("xdg-open", filepath.Dir(arquivo)).Start()
			},
		},
	},
	"tokens":      paginaTokens,
	"ad":          paginaAD,
	"impressoras": paginaImpressoras,
}
