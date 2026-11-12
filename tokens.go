package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Instalador de Tokens: a lista de tokens e programas vem do próprio painel da imagem
// (painel_tokens.sh --listar), e a instalação é o mesmo instalar_tokens.sh root, pelo
// pkexec, como no painel yad. O script escreve o andamento no formato do yad --progress e
// deixa o resultado de cada item em instalar_tokens.relatorio.

var (
	relatorioTokens = "/usr/local/seduc/logs/instalar_tokens.relatorio"
	logTokens       = "/usr/local/seduc/logs/instalar_tokens.log"
	pkexec          = "pkexec"
)

type opcaoToken struct {
	Chave, Nome, Obs string
	Marcado          bool // conectado (token) ou marcado por padrão (programa)
}

type dadosTokens struct {
	Live      bool
	Detectado bool
	Tokens    []opcaoToken
	Extras    []opcaoToken
	Erro      string
}

func listarTokens(c *Central) dadosTokens {
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelar()
	saida, err := exec.CommandContext(ctx, filepath.Join(c.Scripts, "painel_tokens.sh"), "--listar").Output()
	d := dadosTokens{}
	if err != nil {
		d.Erro = "Não foi possível ler a lista de tokens (painel_tokens.sh --listar)."
		return d
	}
	s := bufio.NewScanner(strings.NewReader(string(saida)))
	for s.Scan() {
		campos := strings.Split(s.Text(), "|")
		switch {
		case campos[0] == "LIVE":
			d.Live = true
		case campos[0] == "TOKEN" && len(campos) == 4:
			o := opcaoToken{Chave: campos[1], Nome: campos[2], Marcado: campos[3] == "1"}
			d.Detectado = d.Detectado || o.Marcado
			d.Tokens = append(d.Tokens, o)
		case campos[0] == "EXTRA" && len(campos) == 5:
			d.Extras = append(d.Extras, opcaoToken{Chave: campos[1], Marcado: campos[2] == "TRUE", Nome: campos[3], Obs: campos[4]})
		}
	}
	if len(d.Tokens) == 0 && !d.Live {
		d.Erro = "O painel de tokens não informou nenhum token."
	}
	return d
}

func chaves(lista []opcaoToken) []string {
	var k []string
	for _, o := range lista {
		k = append(k, o.Chave)
	}
	return k
}

// fimTokens lê o relatório do script, como o mostrar_relatorio do painel yad.
func fimTokens(codigo int, _ map[string]string) ([][2]string, string) {
	switch codigo {
	case 126, 127:
		return nil, "A senha do administrador não foi confirmada. Nada foi instalado."
	}
	var rel [][2]string
	if b, err := os.ReadFile(relatorioTokens); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if item, sit, ok := strings.Cut(l, "|"); ok {
				rel = append(rel, [2]string{item, sit})
			}
		}
	}
	if codigo == 0 {
		return rel, "Instalação concluída. Se instalou extensões, feche e abra os navegadores."
	}
	return rel, "Algo não foi instalado. Tente de novo mais tarde; os detalhes ficam em " + logTokens + "."
}

var paginaTokens = &Pagina{
	Titulo: "Instalador de Tokens",
	Modelo: "tokens.html",
	Dados:  func(c *Central) any { return listarTokens(c) },
	Iniciar: func(c *Central, r *http.Request) (Comando, error) {
		d := listarTokens(c)
		if d.Erro != "" {
			return Comando{}, errFormulario(d.Erro)
		}
		if d.Live {
			return Comando{}, errFormulario("O instalador de tokens não funciona no modo de teste (Live). Instale o sistema primeiro.")
		}
		token := r.PostFormValue("token")
		if token != "nenhum" && !slices.Contains(chaves(d.Tokens), token) {
			return Comando{}, errFormulario("Escolha o modelo do token, ou \"Nenhum token\".")
		}
		var extras []string
		for _, e := range r.PostForm["extra"] {
			if !slices.Contains(chaves(d.Extras), e) {
				return Comando{}, errFormulario("Programa desconhecido: " + e)
			}
			if !slices.Contains(extras, e) {
				extras = append(extras, e)
			}
		}
		if token == "nenhum" && len(extras) == 0 {
			return Comando{}, errFormulario("Nenhuma opção selecionada.")
		}
		argv := append([]string{pkexec, filepath.Join(c.Scripts, "instalar_tokens.sh"), token}, extras...)
		return Comando{Argv: argv, Limite: 60 * time.Minute, Fim: fimTokens}, nil
	},
}
