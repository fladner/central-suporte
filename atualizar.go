package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Atualizar o sistema: o mesmo atualizar_sistema_root.sh pelo pkexec, como no yad. A
// página mostra a etapa atual (linhas "==> ") e a saída enquanto roda; no fim, oferece
// reiniciar só quando o sistema pede (/var/run/reboot-required).

var (
	logAtualizar     = "/usr/local/seduc/logs/atualizar_sistema.log"
	reinicioPendente = "/var/run/reboot-required"
	cmdlineKernel    = "/proc/cmdline"
)

type dadosAtualizar struct {
	Live bool
}

func modoLive() bool {
	b, _ := os.ReadFile(cmdlineKernel)
	return strings.Contains(string(b), "casper")
}

func fimAtualizar(codigo int, res map[string]string) ([][2]string, string) {
	switch {
	case senhaRecusada(codigo):
		return nil, "A senha do administrador não foi confirmada. Nada foi atualizado."
	case codigo != 0:
		return nil, "A atualização terminou com erros. Detalhes em " + logAtualizar + "."
	}
	if _, err := os.Stat(reinicioPendente); err == nil {
		res["reiniciar"] = "1"
		return nil, "Sistema atualizado. Reinicie o computador para concluir a atualização."
	}
	return nil, "Sistema atualizado."
}

var paginaAtualizar = &Pagina{
	Titulo: "Atualizar o sistema",
	Modelo: "atualizar.html",
	Dados:  func(c *Central) any { return dadosAtualizar{Live: modoLive()} },
	Iniciar: func(c *Central, r *http.Request) (Comando, error) {
		if modoLive() {
			return Comando{}, errFormulario("No modo live (pendrive) o sistema não é atualizado.")
		}
		return Comando{
			Argv:     []string{pkexec, filepath.Join(c.Scripts, "atualizar_sistema_root.sh")},
			Limite:   3 * time.Hour,
			Fim:      fimAtualizar,
			Mensagem: "Iniciando... o sistema vai pedir a senha do administrador.",
			AoVivo:   true,
			Etapa:    "==> ",
		}, nil
	},
	Acoes: map[string]func(*Central, map[string]string) error{
		"reiniciar": func(c *Central, res map[string]string) error {
			if res["reiniciar"] != "1" {
				return errSemResultado
			}
			return reiniciar()
		},
	},
}
