package main

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Active Directory SEDUC: a situação vem do painel da imagem (painel_ad.sh --listar) e o
// ingresso é o mesmo ingressar_ad.sh root pelo pkexec, com a senha do AD na entrada padrão
// (nunca na linha de comando), como no painel yad.

var (
	relatorioAD = "/usr/local/seduc/logs/ingressar_ad.relatorio"
	logAD       = "/usr/local/seduc/logs/ingressar_ad.log"

	reComputador = regexp.MustCompile(`^[A-Z0-9-]{1,15}$`)
	reUsuarioAD  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@\\-]{0,63}$`)
	reOU         = regexp.MustCompile(`^[\p{L}0-9 ._/=,-]{0,200}$`)
)

type dadosAD struct {
	Live       bool
	Dominio    string
	NoDominio  string // domínio atual, se já ingressado
	Computador string
	Erro       string
}

func lerAD(c *Central) dadosAD {
	linhas, err := listarPainel(c, "painel_ad.sh")
	d := dadosAD{}
	if err != nil {
		d.Erro = "Não foi possível ler a situação do domínio (painel_ad.sh --listar)."
		return d
	}
	for _, l := range linhas {
		switch {
		case l[0] == "LIVE":
			d.Live = true
		case l[0] == "DOMINIO" && len(l) == 2:
			d.Dominio = l[1]
		case l[0] == "NO_DOMINIO" && len(l) == 2:
			d.NoDominio = l[1]
		case l[0] == "COMPUTADOR" && len(l) == 2:
			d.Computador = l[1]
		}
	}
	return d
}

func fimAD(acao string) AoTerminar {
	return func(codigo int, _ map[string]string) ([][2]string, string) {
		if senhaRecusada(codigo) {
			return nil, "A senha do administrador local não foi confirmada. Nada foi alterado."
		}
		rel := lerRelatorio(relatorioAD)
		switch {
		case codigo == 0 && acao == "sair":
			return rel, "Computador fora do domínio. Reinicie para voltar só com os usuários locais."
		case codigo == 0:
			return rel, "Pronto. Reinicie o computador para usar o login do domínio. Depois de entrar, o usuário instala as impressoras em Impressoras SEDUC."
		}
		return rel, "Algo deu errado. Detalhes em " + logAD + "."
	}
}

var paginaAD = &Pagina{
	Titulo: "Active Directory SEDUC",
	Modelo: "ad.html",
	Dados:  func(c *Central) any { return lerAD(c) },
	Iniciar: func(c *Central, r *http.Request) (Comando, error) {
		d := lerAD(c)
		if d.Erro != "" {
			return Comando{}, errFormulario(d.Erro)
		}
		if d.Live {
			return Comando{}, errFormulario("O ingresso no domínio não funciona no modo de teste (Live). Instale o sistema primeiro.")
		}
		acao := r.PostFormValue("acao")
		usuario := strings.TrimSpace(r.PostFormValue("usuario"))
		senha := r.PostFormValue("senha")
		if usuario == "" || senha == "" {
			return Comando{}, errFormulario("Informe o usuário e a senha do administrador do AD.")
		}
		if !reUsuarioAD.MatchString(usuario) {
			return Comando{}, errFormulario("Usuário do AD inválido.")
		}
		// A senha vai pela entrada padrão (1ª linha): só não pode ter quebra de linha.
		if strings.ContainsAny(senha, "\r\n\x00") || len(senha) > 256 {
			return Comando{}, errFormulario("Senha inválida.")
		}
		script := filepath.Join(c.Scripts, "ingressar_ad.sh")
		var argv []string
		switch acao {
		case "sair":
			if d.NoDominio == "" {
				return Comando{}, errFormulario("Este computador não está no domínio.")
			}
			if r.PostFormValue("confirmo") != "sim" {
				return Comando{}, errFormulario("Marque a confirmação para remover o computador do domínio.")
			}
			argv = []string{pkexec, script, "sair", usuario}
		case "entrar":
			computador := strings.ToUpper(strings.TrimSpace(r.PostFormValue("computador")))
			ou := strings.TrimSpace(r.PostFormValue("ou"))
			if !reComputador.MatchString(computador) {
				return Comando{}, errFormulario("Nome do computador inválido. Use até 15 letras, números ou hífen.")
			}
			if !reOU.MatchString(ou) || !semQuebra(ou) {
				return Comando{}, errFormulario("OU inválida. Exemplo: Escolas/Computadores.")
			}
			argv = []string{pkexec, script, "entrar", usuario, computador, ou}
		default:
			return Comando{}, errFormulario("Ação inválida.")
		}
		return Comando{Argv: argv, Entrada: senha + "\n", Limite: 15 * time.Minute, Fim: fimAD(acao)}, nil
	},
	Acoes: map[string]func(*Central, map[string]string) error{
		"reiniciar": func(c *Central, _ map[string]string) error {
			return reiniciar()
		},
	},
}
