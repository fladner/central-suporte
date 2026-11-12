package main

import (
	"net/http"
	"path/filepath"
	"slices"
	"time"
)

// Boas-vindas e usuários: a lista de usuários da escola vem do próprio welcome.sh
// (--listar), e a criação é o mesmo criar_usuarios.sh root pelo pkexec, como no yad.
// O primeiro marcado (na ordem da lista) define o nome do computador e o tema.

var logUsuarios = "/usr/local/seduc/logs/criar_usuarios.log"

type usuarioEscola struct {
	Login, Nome, Grupo string
	Existe             bool
}

type grupoUsuarios struct {
	Nome     string
	Usuarios []usuarioEscola
}

type dadosBoasVindas struct {
	Live    bool
	Grupos  []grupoUsuarios
	Logins  []string
	Erro    string
	Sistema string
}

func lerBoasVindas(c *Central) dadosBoasVindas {
	linhas, err := listarPainel(c, "welcome.sh")
	d := dadosBoasVindas{}
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
		}
	}
	return d
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
		argv := append([]string{pkexec, filepath.Join(c.Scripts, "criar_usuarios.sh")}, logins...)
		return Comando{Argv: argv, Limite: 30 * time.Minute, Fim: fimBoasVindas,
			Mensagem: "Criando usuários e configurando o computador..."}, nil
	},
	Acoes: map[string]func(*Central, map[string]string) error{
		"reiniciar": func(c *Central, _ map[string]string) error { return reiniciar() },
	},
}
