package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Boas-vindas e usuários: a lista de usuários da escola vem do próprio welcome.sh
// (--listar), e a criação é o mesmo criar_usuarios.sh root pelo pkexec, como no yad.
// O primeiro marcado (na ordem da lista) define o tema. O nome do computador vem da
// identificação: escola (INEP, consultado na API do NIC.br pelo welcome.sh --inep) ou
// CREDE, mais o ambiente; o identificar_computador.sh acrescenta o serial.

var logUsuarios = "/usr/local/seduc/logs/criar_usuarios.log"

type usuarioEscola struct {
	Login, Nome, Grupo string
	Existe             bool
}

type grupoUsuarios struct {
	Nome     string
	Usuarios []usuarioEscola
}

// Ambiente do computador (ambientes.conf): Tipo é "escola" ou "crede".
type ambiente struct{ Tipo, Sigla, Nome string }

// Identificação já gravada (identificacao.conf), para preencher o formulário.
type identificacao struct{ Tipo, INEP, CREDE, Ambiente, Escola, Computador string }

type dadosBoasVindas struct {
	Live      bool
	Grupos    []grupoUsuarios
	Logins    []string
	Erro      string
	Sistema   string
	Ambientes []ambiente
	Credes    []string
	Ident     identificacao
}

// AmbientesDe: ambientes de um tipo, para a lista do formulário.
func (d dadosBoasVindas) AmbientesDe(tipo string) []ambiente {
	var r []ambiente
	for _, a := range d.Ambientes {
		if a.Tipo == tipo {
			r = append(r, a)
		}
	}
	return r
}

func (d dadosBoasVindas) ambienteValido(tipo, sigla string) bool {
	return slices.ContainsFunc(d.Ambientes, func(a ambiente) bool { return a.Tipo == tipo && a.Sigla == sigla })
}

func lerBoasVindas(c *Central) dadosBoasVindas {
	linhas, err := listarPainel(c, "welcome.sh")
	d := dadosBoasVindas{}
	for i := 1; i <= 20; i++ {
		d.Credes = append(d.Credes, fmt.Sprintf("%02d", i))
	}
	if err != nil {
		d.Erro = "Não foi possível ler a lista de usuários (welcome.sh --listar)."
		return d
	}
	for _, l := range linhas {
		switch {
		case l[0] == "LIVE":
			d.Live = true
		case l[0] == "ERRO" && len(l) >= 2:
			d.Erro = l[1]
		case l[0] == "SISTEMA" && len(l) == 2:
			d.Sistema = l[1]
		case l[0] == "USUARIO" && len(l) == 5:
			u := usuarioEscola{Login: l[1], Nome: l[2], Grupo: l[3], Existe: l[4] == "1"}
			d.Logins = append(d.Logins, u.Login)
			if n := len(d.Grupos); n > 0 && d.Grupos[n-1].Nome == u.Grupo {
				d.Grupos[n-1].Usuarios = append(d.Grupos[n-1].Usuarios, u)
			} else {
				d.Grupos = append(d.Grupos, grupoUsuarios{Nome: u.Grupo, Usuarios: []usuarioEscola{u}})
			}
		case l[0] == "AMBIENTE" && len(l) == 4:
			d.Ambientes = append(d.Ambientes, ambiente{Tipo: l[1], Sigla: l[2], Nome: l[3]})
		case l[0] == "IDENT" && len(l) == 7:
			d.Ident = identificacao{Tipo: l[1], INEP: l[2], CREDE: l[3], Ambiente: l[4], Escola: l[5], Computador: l[6]}
		}
	}
	return d
}

// consultarINEP: nome da escola pelo welcome.sh --inep. situacao: ESCOLA, NAOENCONTRADA,
// INVALIDO ou SEMREDE (sem internet ou API fora do ar).
func consultarINEP(c *Central, inep string) (situacao, escola string) {
	ctx, cancelar := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelar()
	saida, err := exec.CommandContext(ctx, filepath.Join(c.Scripts, "welcome.sh"), "--inep", inep).Output()
	if err != nil {
		return "SEMREDE", ""
	}
	linha, _, _ := strings.Cut(strings.TrimSpace(string(saida)), "\n")
	situacao, escola, _ = strings.Cut(linha, "|")
	switch situacao {
	case "ESCOLA", "NAOENCONTRADA", "INVALIDO", "SEMREDE":
		return situacao, escola
	}
	return "SEMREDE", ""
}

func soDigitos(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// argumentosIdentificacao confere a escola/CREDE e o ambiente do formulário e devolve
// os argumentos --id do criar_usuarios.sh (nenhum para SEDUC, sem identificação).
func argumentosIdentificacao(c *Central, d dadosBoasVindas, r *http.Request) ([]string, error) {
	switch r.PostFormValue("tipo") {
	case "seduc":
		return nil, nil
	case "escola":
		inep := soDigitos(r.PostFormValue("inep"))
		if inep == "" {
			// Sem INEP: cria os usuários e o computador fica com o nome que já tem
			return []string{"--manter-nome"}, nil
		}
		if len(inep) != 8 {
			return nil, errFormulario("Informe o INEP da escola (8 números).")
		}
		sigla := r.PostFormValue("ambiente_escola")
		if !d.ambienteValido("escola", sigla) {
			return nil, errFormulario("Escolha o ambiente do computador na escola.")
		}
		if r.PostFormValue("inep_conferido") != inep {
			return nil, errFormulario("Clique em Consultar escola e confira o nome antes de continuar.")
		}
		situacao, escola := consultarINEP(c, inep)
		switch situacao {
		case "NAOENCONTRADA":
			return nil, errFormulario("Não encontramos nenhuma escola com o INEP " + inep + ". Verifique o código.")
		case "INVALIDO":
			return nil, errFormulario("O INEP informado é inválido.")
		}
		// SEMREDE: segue só com o INEP; o nome da escola fica em branco
		return []string{"--id", "escola", inep, sigla, escola}, nil
	case "crede":
		n, err := strconv.Atoi(r.PostFormValue("crede"))
		if err != nil || n < 1 || n > 20 {
			return nil, errFormulario("Escolha a CREDE.")
		}
		sigla := r.PostFormValue("ambiente_crede")
		if !d.ambienteValido("crede", sigla) {
			return nil, errFormulario("Escolha o setor do computador na CREDE.")
		}
		return []string{"--id", "crede", fmt.Sprintf("%02d", n), sigla, ""}, nil
	}
	return nil, errFormulario("Escolha onde o computador fica: escola, CREDE ou SEDUC.")
}

func fimBoasVindas(codigo int, _ map[string]string) ([][2]string, string) {
	switch {
	case senhaRecusada(codigo):
		return nil, "A senha do administrador não foi confirmada. Nenhum usuário foi criado."
	case codigo == 0:
		return nil, "Configuração concluída com sucesso! Reinicie o computador para aplicar o nome do computador e o tema."
	}
	return nil, "Erro durante a configuração. Detalhes em " + logUsuarios + "."
}

var paginaBoasVindas = &Pagina{
	Titulo: "Boas-vindas e usuários",
	Modelo: "boas-vindas.html",
	Dados:  func(c *Central) any { return lerBoasVindas(c) },
	Iniciar: func(c *Central, r *http.Request) (Comando, error) {
		d := lerBoasVindas(c)
		if d.Erro != "" {
			return Comando{}, errFormulario(d.Erro)
		}
		if d.Live {
			return Comando{}, errFormulario("A criação de usuários não funciona no modo de teste (Live). Instale o sistema primeiro.")
		}
		// Na ordem da lista, não na ordem do formulário: o primeiro define o computador.
		marcados := r.PostForm["usuario"]
		for _, m := range marcados {
			if !slices.Contains(d.Logins, m) {
				return Comando{}, errFormulario("Usuário desconhecido: " + m)
			}
		}
		var logins []string
		for _, l := range d.Logins {
			if slices.Contains(marcados, l) {
				logins = append(logins, l)
			}
		}
		if len(logins) == 0 {
			return Comando{}, errFormulario("Marque pelo menos um usuário para continuar.")
		}
		if r.PostFormValue("confirmo") != "sim" {
			return Comando{}, errFormulario("Confirme que os usuários serão criados com a senha padrão.")
		}
		ident, err := argumentosIdentificacao(c, d, r)
		if err != nil {
			return Comando{}, err
		}
		argv := append([]string{pkexec, filepath.Join(c.Scripts, "criar_usuarios.sh")}, ident...)
		argv = append(argv, logins...)
		return Comando{Argv: argv, Limite: 30 * time.Minute, Fim: fimBoasVindas,
			Mensagem: "Criando usuários e configurando o computador..."}, nil
	},
	Acoes: map[string]func(*Central, map[string]string) error{
		"reiniciar": func(c *Central, _ map[string]string) error { return reiniciar() },
	},
	Consultas: map[string]func(*Central, *http.Request) any{
		"inep": func(c *Central, r *http.Request) any {
			inep := soDigitos(r.URL.Query().Get("inep"))
			if len(inep) != 8 {
				return map[string]string{"situacao": "INVALIDO"}
			}
			situacao, escola := consultarINEP(c, inep)
			return map[string]string{"situacao": situacao, "escola": escola, "inep": inep}
		},
	},
}
