package main

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Atualizar o sistema: o mesmo atualizar_sistema_root.sh pelo pkexec, como no yad. A
// página mostra a etapa atual (linhas "==> ") e a saída enquanto roda; no fim, oferece
// reiniciar só quando o sistema pede (/var/run/reboot-required). Antes de começar, mostra
// como está a atualização automática (seduc-atualizar.timer, atualizar_pacotes.sh), pelo
// arquivo de status que o script grava.

var (
	logAtualizar     = "/usr/local/seduc/logs/atualizar_sistema.log"
	reinicioPendente = "/var/run/reboot-required"
	cmdlineKernel    = "/proc/cmdline"
	statusAuto       = "/var/lib/seduc-atualizar/status"
	timerAuto        = "/etc/systemd/system/seduc-atualizar.timer"
)

type dadosAtualizar struct {
	Live bool
	Auto *estadoAuto
}

// Atualização automática: Existe sem Data quando ainda não rodou
type estadoAuto struct {
	Data       string
	Origem     string
	Situacao   string
	OK         bool
	Mensagem   string
	Bloqueados string
	UltimoOK   string
	Reiniciar  bool
}

var situacoesAuto = map[string]string{
	"ok":       "Tudo atualizado",
	"parcial":  "Atualização incompleta",
	"sem-rede": "Sem acesso aos repositórios",
	"erro":     "Atualização com erros",
}

// lerStatusAuto: nil quando a máquina não tem a atualização automática (antes da 1.7-0)
func lerStatusAuto() *estadoAuto {
	if _, err := os.Stat(timerAuto); err != nil {
		return nil
	}
	e := &estadoAuto{}
	f, err := os.Open(statusAuto)
	if err != nil {
		return e
	}
	defer f.Close()
	v := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, val, ok := strings.Cut(sc.Text(), "="); ok {
			v[k] = val
		}
	}
	e.Data = v["DATA_TEXTO"]
	e.Origem = "automática"
	if v["ORIGEM"] == "manual" {
		e.Origem = "pelo Atualizar o sistema"
	}
	e.OK = v["RESULTADO"] == "ok"
	e.Situacao = situacoesAuto[v["RESULTADO"]]
	if e.Situacao == "" {
		e.Situacao = v["RESULTADO"]
	}
	e.Mensagem = v["MENSAGEM"]
	e.Bloqueados = v["BLOQUEADOS"]
	e.UltimoOK = v["ULTIMO_OK_DATA"]
	e.Reiniciar = v["REINICIAR"] == "1"
	return e
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
	Dados:  func(c *Central) any { return dadosAtualizar{Live: modoLive(), Auto: lerStatusAuto()} },
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
