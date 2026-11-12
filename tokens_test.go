package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func executavelTeste(t *testing.T, caminho, corpo string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte("#!/bin/sh\n"+corpo), 0o755); err != nil {
		t.Fatal(err)
	}
}

func postarForm(c *Central, caminho string, form url.Values, ck *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", caminho, strings.NewReader(form.Encode()))
	r.Host = c.endereco
	r.Header.Set("Origin", "http://"+c.endereco)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(ck)
	w := httptest.NewRecorder()
	c.ServeHTTP(w, r)
	return w
}

// Fluxo do Instalador de Tokens com um painel e um instalador de mentira: a lista vem do
// painel --listar, o formulário é conferido e o instalador roda pelo "pkexec".
func TestPaginaTokens(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "50-tokens.desktop", "[Desktop Entry]\nName=Instalador de Tokens\nExec=/bin/true\nX-SEDUC-Web=tokens\nX-SEDUC-Acesso=todos\n")
	d := t.TempDir()
	args := filepath.Join(d, "args")
	relatorioTokens = filepath.Join(d, "relatorio")
	executavelTeste(t, filepath.Join(c.Scripts, "painel_tokens.sh"), `[ "$1" = --listar ] || exit 9
echo "TOKEN|safenet|Safenet eToken 5100|0"
echo "TOKEN|cryptoid|mToken CryptoID|1"
echo "EXTRA|logus|TRUE|Assinador Logus|Siafe"
echo "EXTRA|java8|FALSE|Oracle Java 8|precisa de internet"
`)
	executavelTeste(t, filepath.Join(c.Scripts, "instalar_tokens.sh"), `echo "$@" > `+args+`
echo 30; echo "# Instalando o driver"
printf 'mToken CryptoID|instalado\nAssinador Logus|FALHA no download\n' > `+relatorioTokens+`
exit 2
`)
	falso := filepath.Join(d, "pkexec")
	executavelTeste(t, falso, `exec "$@"`)
	pkexec = falso

	ck := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil).Result().Cookies()[0]
	w := pedir(c, "GET", "/pagina/tokens", c.endereco, "", ck)
	corpo := w.Body.String()
	if w.Code != 200 || !strings.Contains(corpo, `value="cryptoid" checked`) || !strings.Contains(corpo, `value="logus" checked`) || strings.Contains(corpo, `value="java8" checked`) {
		t.Fatalf("formulário: %d %s", w.Code, corpo)
	}
	for _, f := range []url.Values{
		{"token": {"outro"}},
		{"token": {"nenhum"}},
		{"token": {"safenet"}, "extra": {"rm -rf"}},
	} {
		if w := postarForm(c, "/pagina/tokens/iniciar", f, ck); w.Code != 200 || !strings.Contains(w.Body.String(), `class="erro"`) {
			t.Errorf("%v aceito: %d", f, w.Code)
		}
	}
	w = postarForm(c, "/pagina/tokens/iniciar", url.Values{"token": {"cryptoid"}, "extra": {"logus", "java8", "logus"}}, ck)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("iniciar: %d %s", w.Code, w.Body.String())
	}
	id := strings.TrimPrefix(w.Header().Get("Location"), "/tarefa/")
	var e Estado
	for i := 0; i < 50 && !e.Fim; i++ {
		time.Sleep(100 * time.Millisecond)
		json.NewDecoder(pedir(c, "GET", "/tarefa/"+id+"/estado", c.endereco, "", ck).Body).Decode(&e)
	}
	if !e.Fim || e.OK || e.Codigo != 2 || len(e.Relatorio) != 2 || e.Relatorio[1][1] != "FALHA no download" || !strings.Contains(e.Aviso, "Algo não foi instalado") {
		t.Fatalf("estado: %+v", e)
	}
	if b, _ := os.ReadFile(args); strings.TrimSpace(string(b)) != "cryptoid logus java8" {
		t.Errorf("argumentos: %q", b)
	}
}

func TestFimTokensSenhaRecusada(t *testing.T) {
	if rel, aviso := fimTokens(126, nil); rel != nil || !strings.Contains(aviso, "senha") {
		t.Errorf("%v %q", rel, aviso)
	}
}
