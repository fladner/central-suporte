package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Boas-vindas: lista do welcome.sh --listar; criar_usuarios.sh recebe os logins na ordem
// da lista (o primeiro define o computador), não na ordem do formulário.
func TestPaginaBoasVindas(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "10-bv.desktop", "[Desktop Entry]\nName=Boas-vindas e usuários\nExec=/bin/true\nX-SEDUC-Web=boas-vindas\nX-SEDUC-Acesso=todos\n")
	d := t.TempDir()
	args := filepath.Join(d, "args")
	executavelTeste(t, filepath.Join(c.Scripts, "welcome.sh"), `if [ "$1" = --inep ]; then
  case "$2" in 23012345) echo "ESCOLA|EEM Teste";; 11111111) echo "SEMREDE";; *) echo "NAOENCONTRADA";; esac
  exit 0
fi
[ "$1" = --listar ] || exit 9
echo "SISTEMA|Xubuntu 26.04 LTS"
echo "AMBIENTE|escola|LMAT|Laboratório de Matemática"
echo "AMBIENTE|crede|INF|Informática"
echo "IDENT|escola|23012345||LMAT|EEM Teste|23012345-LMAT-1234"
echo "USUARIO|aluno|Aluno|Laboratório|0"
echo "USUARIO|direcao|Direção|Núcleo gestor|1"
echo "USUARIO|secretaria|Secretaria|Núcleo gestor|0"
`)
	executavelTeste(t, filepath.Join(c.Scripts, "criar_usuarios.sh"), `printf '%s\n' "$@" > `+args+`
echo "--- Configurando"
`)
	falso := filepath.Join(d, "pkexec")
	executavelTeste(t, falso, `exec "$@"`)
	pkexec = falso
	reiniciado := false
	reiniciar = func() error { reiniciado = true; return nil }

	c.Abrir = "boas-vindas"
	w := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil)
	if w.Header().Get("Location") != "/pagina/boas-vindas" {
		t.Errorf("--abrir: %q", w.Header().Get("Location"))
	}
	ck := w.Result().Cookies()[0]
	corpo := pedir(c, "GET", "/pagina/boas-vindas", c.endereco, "", ck).Body.String()
	if !strings.Contains(corpo, "<legend>Núcleo gestor</legend>") || !strings.Contains(corpo, "já existe") || !strings.Contains(corpo, "Xubuntu 26.04 LTS") {
		t.Fatalf("formulário: %s", corpo)
	}
	if !strings.Contains(corpo, `value="LMAT" selected`) || !strings.Contains(corpo, "23012345-LMAT-1234") || !strings.Contains(corpo, "CREDE 20") {
		t.Errorf("identificação no formulário: %s", corpo)
	}
	// Consulta do INEP (JSON)
	if w := pedir(c, "GET", "/pagina/boas-vindas/inep?inep=2301-2345", c.endereco, "", ck); !strings.Contains(w.Body.String(), `"escola":"EEM Teste"`) {
		t.Errorf("consulta inep: %d %s", w.Code, w.Body.String())
	}
	ok := url.Values{"usuario": {"aluno"}, "confirmo": {"sim"}}
	com := func(kv ...string) url.Values {
		f := url.Values{}
		for k, v := range ok {
			f[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			f.Set(kv[i], kv[i+1])
		}
		return f
	}
	for _, f := range []url.Values{
		{"confirmo": {"sim"}, "tipo": {"seduc"}},
		{"usuario": {"root"}, "confirmo": {"sim"}, "tipo": {"seduc"}},
		{"usuario": {"aluno"}, "tipo": {"seduc"}},
		ok,                                   // sem tipo
		com("tipo", "escola", "inep", "123"), // INEP curto
		com("tipo", "escola", "inep", "23012345", "ambiente_escola", "LMAT"),                               // sem consultar
		com("tipo", "escola", "inep", "23012345", "ambiente_escola", "INF", "inep_conferido", "23012345"),  // ambiente da CREDE
		com("tipo", "escola", "inep", "99999999", "ambiente_escola", "LMAT", "inep_conferido", "99999999"), // não encontrada
		com("tipo", "crede", "crede", "21", "ambiente_crede", "INF"),
		com("tipo", "crede", "crede", "05", "ambiente_crede", "LMAT"),
	} {
		if w := postarForm(c, "/pagina/boas-vindas/iniciar", f, ck); w.Code != 200 || !strings.Contains(w.Body.String(), `class="erro"`) {
			t.Errorf("%v aceito: %d", f, w.Code)
		}
	}
	// Escola sem internet na hora: segue só com o INEP
	w = postarForm(c, "/pagina/boas-vindas/iniciar", com("tipo", "escola", "inep", "11111111", "ambiente_escola", "LMAT", "inep_conferido", "11111111"), ck)
	if w.Code != 303 {
		t.Fatalf("escola sem rede: %d %s", w.Code, w.Body.String())
	}
	esperarTarefa(t, c, ck, w.Header().Get("Location"))
	if b, _ := os.ReadFile(args); string(b) != "--id\nescola\n11111111\nLMAT\n\naluno\n" {
		t.Errorf("escola sem rede: %q", b)
	}
	w = postarForm(c, "/pagina/boas-vindas/iniciar", com("tipo", "escola", "inep", "2301 2345", "ambiente_escola", "LMAT", "inep_conferido", "23012345"), ck)
	esperarTarefa(t, c, ck, w.Header().Get("Location"))
	if b, _ := os.ReadFile(args); string(b) != "--id\nescola\n23012345\nLMAT\nEEM Teste\naluno\n" {
		t.Errorf("escola: %q", b)
	}
	// Escola sem INEP: cria os usuários e mantém o nome do computador (ambiente ignorado)
	w = postarForm(c, "/pagina/boas-vindas/iniciar", com("tipo", "escola", "inep", " ", "ambiente_escola", "LMAT"), ck)
	esperarTarefa(t, c, ck, w.Header().Get("Location"))
	if b, _ := os.ReadFile(args); string(b) != "--manter-nome\naluno\n" {
		t.Errorf("escola sem INEP: %q", b)
	}
	w = postarForm(c, "/pagina/boas-vindas/iniciar", com("tipo", "crede", "crede", "5", "ambiente_crede", "INF"), ck)
	esperarTarefa(t, c, ck, w.Header().Get("Location"))
	if b, _ := os.ReadFile(args); string(b) != "--id\ncrede\n05\nINF\n\naluno\n" {
		t.Errorf("crede: %q", b)
	}
	w = postarForm(c, "/pagina/boas-vindas/iniciar", url.Values{"usuario": {"secretaria", "direcao"}, "confirmo": {"sim"}, "tipo": {"seduc"}}, ck)
	if w.Code != 303 {
		t.Fatalf("iniciar: %d %s", w.Code, w.Body.String())
	}
	local := w.Header().Get("Location")
	e := esperarTarefa(t, c, ck, local)
	if !e.OK || !strings.Contains(e.Aviso, "Reinicie") {
		t.Fatalf("estado: %+v", e)
	}
	if b, _ := os.ReadFile(args); string(b) != "direcao\nsecretaria\n" {
		t.Errorf("ordem: %q", b)
	}
	if w := postarForm(c, local+"/reiniciar", nil, ck); w.Code != 204 || !reiniciado {
		t.Errorf("reiniciar: %d %v", w.Code, reiniciado)
	}
}
