package main

import (
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Impressoras SEDUC, para o usuário do AD já logado: o painel da imagem lista as filas do
// servidor de impressão com o ticket Kerberos do usuário (painel_impressora_ad.sh --listar),
// pede a senha do AD quando falta o ticket (--kinit, senha na entrada padrão) e marca a
// padrão do usuário (--padrao). Instalar e remover é o mesmo instalar_impressora_ad.sh pelo
// pkexec, sem senha para o usuário da sessão ativa (polkit br.gov.ce.seduc.impressora-ad).

var logImpressoras = "/usr/local/seduc/logs/instalar_impressora_ad.log"

type filaImpressora struct {
	Nome, Descricao string
	Instalada       bool
}

type dadosImpressoras struct {
	Servidor  string
	SemTicket bool
	Filas     []filaImpressora
	Erro      string
}

func lerImpressoras(c *Central) dadosImpressoras {
	linhas, err := listarPainel(c, "painel_impressora_ad.sh")
	d := dadosImpressoras{}
	if err != nil {
		d.Erro = "Não foi possível consultar as impressoras (painel_impressora_ad.sh --listar)."
		return d
	}
	for _, l := range linhas {
		switch {
		case l[0] == "ERRO" && len(l) >= 2:
			d.Erro = strings.Join(l[1:], "|")
		case l[0] == "SERVIDOR" && len(l) == 2:
			d.Servidor = l[1]
		case l[0] == "SEM_TICKET":
			d.SemTicket = true
		case l[0] == "FILA" && len(l) == 4:
			d.Filas = append(d.Filas, filaImpressora{Nome: l[1], Descricao: l[2], Instalada: l[3] == "1"})
		}
	}
	return d
}

func fimImpressoras(c *Central, acao string, filas []string) AoTerminar {
	return func(codigo int, _ map[string]string) ([][2]string, string) {
		if codigo != 0 {
			return nil, "Não foi possível concluir. Detalhes em " + logImpressoras + "."
		}
		var rel [][2]string
		sit := "removida"
		if acao == "adicionar" {
			sit = "instalada"
		}
		for _, f := range filas {
			rel = append(rel, [2]string{f, sit})
		}
		if acao != "adicionar" {
			return rel, "Impressora(s) removida(s)."
		}
		// Padrão só deste usuário: a primeira marcada, como no painel yad.
		ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancelar()
		_ = exec.CommandContext(ctx, filepath.Join(c.Scripts, "painel_impressora_ad.sh"), "--padrao", filas[0]).Run()
		return rel, "Impressora(s) instalada(s). A primeira ficou como padrão para você."
	}
}

var paginaImpressoras = &Pagina{
	Titulo: "Impressoras SEDUC",
	Modelo: "impressoras.html",
	Dados:  func(c *Central) any { return lerImpressoras(c) },
	Formularios: map[string]func(*Central, *http.Request) error{
		// Sem ticket Kerberos (login com senha em cache, fora da rede): pede a senha do AD.
		"kerberos": func(c *Central, r *http.Request) error {
			senha := r.PostFormValue("senha")
			if senha == "" || strings.ContainsAny(senha, "\r\n\x00") || len(senha) > 256 {
				return errFormulario("Digite a sua senha do AD.")
			}
			ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelar()
			cmd := exec.CommandContext(ctx, filepath.Join(c.Scripts, "painel_impressora_ad.sh"), "--kinit")
			cmd.Stdin = strings.NewReader(senha + "\n")
			if err := cmd.Run(); err != nil {
				return errFormulario("Senha recusada ou servidor do domínio fora de alcance.")
			}
			return nil
		},
	},
	Iniciar: func(c *Central, r *http.Request) (Comando, error) {
		d := lerImpressoras(c)
		if d.Erro != "" {
			return Comando{}, errFormulario(d.Erro)
		}
		if d.SemTicket {
			return Comando{}, errFormulario("Digite a sua senha do AD para acessar o servidor de impressão.")
		}
		acao := r.PostFormValue("acao")
		if acao != "adicionar" && acao != "remover" {
			return Comando{}, errFormulario("Ação inválida.")
		}
		var conhecidas, filas []string
		for _, f := range d.Filas {
			conhecidas = append(conhecidas, f.Nome)
		}
		for _, f := range r.PostForm["fila"] {
			if !slices.Contains(conhecidas, f) {
				return Comando{}, errFormulario("Impressora desconhecida: " + f)
			}
			if !slices.Contains(filas, f) {
				filas = append(filas, f)
			}
		}
		if len(filas) == 0 {
			return Comando{}, errFormulario("Marque pelo menos uma impressora.")
		}
		argv := append([]string{pkexec, filepath.Join(c.Scripts, "instalar_impressora_ad.sh"), acao}, filas...)
		return Comando{Argv: argv, Limite: 5 * time.Minute, Fim: fimImpressoras(c, acao, filas)}, nil
	},
}
