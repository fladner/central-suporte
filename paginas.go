package main

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

// Pagina é uma ferramenta da Central feita em web (X-SEDUC-Web=<nome> no .desktop).
// A página só mostra e conduz; o trabalho continua nos scripts da imagem, chamados com
// --sem-janela (andamento no protocolo do yad --progress, ver tarefas.go).
type Pagina struct {
	Titulo  string
	Modelo  string // template de web/
	Iniciar func(c *Central, r *http.Request) (argv []string, limite time.Duration)
	// Acoes depois do fim: nome -> função (recebe o resultado da tarefa).
	Acoes map[string]func(c *Central, resultado map[string]string) error
}

var paginas = map[string]*Pagina{
	"coletar-logs": {
		Titulo: "Coletar logs para o suporte",
		Modelo: "coletar-logs.html",
		Iniciar: func(c *Central, r *http.Request) ([]string, time.Duration) {
			return []string{filepath.Join(c.Scripts, "coletar_logs.sh"), "--sem-janela"}, 10 * time.Minute
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
}
