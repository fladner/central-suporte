package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHistorico(t *testing.T) {
	vs := historico("## 1.5-2 - 12/11/2026\n    - Primeiro item\n      continua aqui\n    - Segundo\n\n## 1.5-1 - 11/11/2026\n    - Antigo\n## 1.0-0\n")
	if len(vs) != 3 || vs[0].Versao != "1.5-2" || vs[0].Data != "12/11/2026" || len(vs[0].Itens) != 2 ||
		vs[0].Itens[0] != "Primeiro item continua aqui" || vs[1].Itens[0] != "Antigo" || vs[2].Versao != "1.0-0" || vs[2].Data != "" {
		t.Errorf("%+v", vs)
	}
	if vs := historico(""); vs != nil {
		t.Errorf("vazio: %+v", vs)
	}
}

func TestPaginaSobre(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "95-sobre.desktop", "[Desktop Entry]\nName=Sobre o Xubuntu SEDUC\nExec=/bin/true\nX-SEDUC-Web=sobre\nX-SEDUC-Acesso=todos\n")
	executavelTeste(t, filepath.Join(c.Scripts, "about_system.sh"), `[ "$1" = --listar ] || exit 9
echo "CAMPO|Este computador pertence|EEM <Teste> (INEP 23012345) - LEI"
echo "CAMPO|Versão|1.5-2"
echo "CONTATO|suporte@exemplo"
`)
	d := t.TempDir()
	changelogSistema = filepath.Join(d, "changelog")
	escrever(t, d, "changelog", "## 1.5-2 - 12/11/2026\n    - Sobre na web\n\n## 1.0-0 - 23/04/2026\n    - Versão inicial\n")
	w := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil)
	ck := w.Result().Cookies()[0]
	corpo := pedir(c, "GET", "/pagina/sobre", c.endereco, "", ck).Body.String()
	for _, s := range []string{"EEM &lt;Teste&gt; (INEP 23012345) - LEI", "<td>1.5-2</td>", `href="mailto:suporte@exemplo"`, "Histórico de versões", `<details class="versao" open>`, "Versão 1.5-2 <small>12/11/2026</small>", "<li>Sobre na web</li>", "<li>Versão inicial</li>"} {
		if !strings.Contains(corpo, s) {
			t.Errorf("falta %q em: %s", s, corpo)
		}
	}
	// Script com erro: a página avisa, sem quebrar
	executavelTeste(t, filepath.Join(c.Scripts, "about_system.sh"), "exit 1\n")
	if corpo := pedir(c, "GET", "/pagina/sobre", c.endereco, "", ck).Body.String(); !strings.Contains(corpo, "about_system.sh --listar") {
		t.Errorf("erro: %s", corpo)
	}
}
