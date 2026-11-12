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
	executavelTeste(t, filepath.Join(c.Scripts, "welcome.sh"), `[ "$1" = --listar ] || exit 9
echo "SISTEMA|Xubuntu 26.04 LTS"
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
	for _, f := range []url.Values{
		{"confirmo": {"sim"}},
		{"usuario": {"root"}, "confirmo": {"sim"}},
		{"usuario": {"aluno"}},
	} {
		if w := postarForm(c, "/pagina/boas-vindas/iniciar", f, ck); w.Code != 200 || !strings.Contains(w.Body.String(), `class="erro"`) {
			t.Errorf("%v aceito: %d", f, w.Code)
		}
	}
	w = postarForm(c, "/pagina/boas-vindas/iniciar", url.Values{"usuario": {"secretaria", "direcao"}, "confirmo": {"sim"}}, ck)
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
