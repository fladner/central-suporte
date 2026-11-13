package main

import (
	"os"
	"strings"
)

// Sobre o Xubuntu SEDUC: os campos vêm do about_system.sh --listar (a data da
// compilação é gravada nele pelo build.sh) e o histórico de versões, do changelog do
// sistema inteiro (a versão atual primeiro).

var changelogSistema = "/usr/local/seduc/changelog"

type versaoSistema struct {
	Versao, Data string
	Itens        []string // um item por mudança, já juntado em uma linha
}

type dadosSobre struct {
	Campos  [][2]string
	Contato string
	Versoes []versaoSistema // changelog inteiro, da mais nova para a mais antiga
	Erro    string
}

// historico lê todas as seções "## <versão> - <data>" do changelog; cada item começa com
// "- " e pode continuar nas linhas seguintes.
func historico(texto string) []versaoSistema {
	var vs []versaoSistema
	for _, l := range strings.Split(texto, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "## "):
			cab := strings.TrimSpace(strings.TrimPrefix(t, "## "))
			versao, data, _ := strings.Cut(cab, " - ")
			vs = append(vs, versaoSistema{Versao: strings.TrimSpace(versao), Data: strings.TrimSpace(data)})
		case len(vs) == 0 || t == "":
		case strings.HasPrefix(t, "- "):
			v := &vs[len(vs)-1]
			v.Itens = append(v.Itens, strings.TrimPrefix(t, "- "))
		default:
			if v := &vs[len(vs)-1]; len(v.Itens) > 0 {
				v.Itens[len(v.Itens)-1] += " " + t
			}
		}
	}
	return vs
}

var paginaSobre = &Pagina{
	Titulo: "Sobre o Xubuntu SEDUC",
	Modelo: "sobre.html",
	Dados: func(c *Central) any {
		var d dadosSobre
		linhas, err := listarPainel(c, "about_system.sh")
		if err != nil {
			d.Erro = "Não foi possível ler as informações do sistema (about_system.sh --listar)."
		}
		for _, l := range linhas {
			switch {
			case l[0] == "CAMPO" && len(l) >= 3:
				d.Campos = append(d.Campos, [2]string{l[1], strings.Join(l[2:], "|")})
			case l[0] == "CONTATO" && len(l) == 2 && strings.Contains(l[1], "@"):
				d.Contato = l[1]
			}
		}
		if b, err := os.ReadFile(changelogSistema); err == nil {
			d.Versoes = historico(string(b))
		}
		return d
	},
}
