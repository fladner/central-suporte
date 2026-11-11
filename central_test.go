package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func escrever(t *testing.T, pasta, nome, conteudo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(pasta, nome), []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListarItensRegrasDoCentralSuporte(t *testing.T) {
	d := t.TempDir()
	escrever(t, d, "20-admin.desktop", "[Desktop Entry]\nName=Admin\nExec=/bin/true\nX-SEDUC-Acesso=admin\n")
	escrever(t, d, "10-todos.desktop", "[Desktop Entry]\nName=Todos\nComment=c\nExec=/bin/true\nIcon=printer\nX-SEDUC-Acesso=todos\n")
	escrever(t, d, "30-falta.desktop", "[Desktop Entry]\nName=Falta\nExec=/nao/existe\nTryExec=/nao/existe\n")
	escrever(t, d, "40-web.desktop", "[Desktop Entry]\nName=Logs\nExec=/bin/true\nX-SEDUC-Web=coletar-logs\n")
	escrever(t, d, "50-web-desconhecida.desktop", "[Desktop Entry]\nName=Nova\nExec=/bin/true\nX-SEDUC-Web=nao-existe\n")
	escrever(t, d, "60-so-web-desconhecida.desktop", "[Desktop Entry]\nName=Some\nX-SEDUC-Web=nao-existe\n")
	escrever(t, d, "LEIAME", "não é item")

	nomes := func(its []Item) (n []string) {
		for _, i := range its {
			n = append(n, i.Nome)
		}
		return
	}
	if got := nomes(listarItens(d, false)); !reflect.DeepEqual(got, []string{"Todos", "Logs", "Nova"}) {
		t.Errorf("usuário comum: %v", got)
	}
	its := listarItens(d, true)
	if got := nomes(its); !reflect.DeepEqual(got, []string{"Todos", "Admin", "Logs", "Nova"}) {
		t.Errorf("administrador: %v", got)
	}
	if its[2].Web != "coletar-logs" || its[3].Web != "" {
		t.Errorf("páginas web: %q %q", its[2].Web, its[3].Web)
	}
}

func TestArgumentosExec(t *testing.T) {
	casos := map[string][]string{
		"/usr/local/seduc/scripts/welcome.sh --central": {"/usr/local/seduc/scripts/welcome.sh", "--central"},
		`prog "com espaço" %U`:                          {"prog", "com espaço"},
		"  a   b ":                                      {"a", "b"},
	}
	for linha, quer := range casos {
		if got := argumentosExec(linha); !reflect.DeepEqual(got, quer) {
			t.Errorf("%q: %v", linha, got)
		}
	}
}

func TestTarefaProtocoloDoYad(t *testing.T) {
	var ts Tarefas
	tf := ts.Iniciar("coletar-logs", []string{"sh", "-c", `echo "# Copiando"; echo 20; echo ruido; echo "RESULTADO_ARQUIVO=/tmp/x.tar.gz"; echo 100`}, 10*time.Second)
	for i := 0; i < 100 && !tf.Estado().Fim; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	e := tf.Estado()
	if !e.Fim || !e.OK || e.Percent != 100 || e.Mensagem != "Copiando" || e.Resultado["arquivo"] != "/tmp/x.tar.gz" {
		t.Fatalf("estado: %+v", e)
	}
	tf = ts.Iniciar("coletar-logs", []string{"sh", "-c", `echo "deu errado"; exit 3`}, 10*time.Second)
	for i := 0; i < 100 && !tf.Estado().Fim; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if e := tf.Estado(); e.OK || !strings.Contains(e.Registro, "deu errado") {
		t.Fatalf("falha: %+v", e)
	}
}

func novaCentral(t *testing.T) *Central {
	t.Helper()
	d := t.TempDir()
	escrever(t, d, "90-logs.desktop", "[Desktop Entry]\nName=Coletar logs\nExec=/bin/true\nX-SEDUC-Web=coletar-logs\nX-SEDUC-Acesso=todos\n")
	c := &Central{Itens: d, Scripts: t.TempDir(), AdConf: "/nao/existe", Versao: "teste"}
	l, err := c.Ouvir(0)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	return c
}

func pedir(c *Central, metodo, caminho, host, origem string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, caminho, nil)
	r.Host = host
	if origem != "" {
		r.Header.Set("Origin", origem)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.ServeHTTP(w, r)
	return w
}

func TestSegurancaDaEntrada(t *testing.T) {
	c := novaCentral(t)
	h := c.endereco
	if w := pedir(c, "GET", "/", h, "", nil); w.Code != http.StatusForbidden {
		t.Errorf("sem cookie: %d", w.Code)
	}
	if w := pedir(c, "GET", "/entrar/"+c.chave, "evil.example:80", "", nil); w.Code != http.StatusForbidden {
		t.Errorf("Host errado: %d", w.Code)
	}
	if w := pedir(c, "GET", "/entrar/errada", h, "", nil); w.Code != http.StatusForbidden {
		t.Errorf("chave errada: %d", w.Code)
	}
	w := pedir(c, "GET", "/entrar/"+c.chave, h, "", nil)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("entrada: %d", w.Code)
	}
	ck := w.Result().Cookies()[0]
	if !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie: %+v", ck)
	}
	if w := pedir(c, "GET", "/entrar/"+c.chave, h, "", nil); w.Code != http.StatusForbidden {
		t.Errorf("chave usada duas vezes: %d", w.Code)
	}
	w = pedir(c, "GET", "/", h, "", ck)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Coletar logs") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("início: %d %s", w.Code, w.Body.String())
	}
	if w := pedir(c, "POST", "/pagina/coletar-logs/iniciar", h, "http://evil.example", ck); w.Code != http.StatusForbidden {
		t.Errorf("POST de outra origem: %d", w.Code)
	}
	if w := pedir(c, "POST", "/pagina/coletar-logs/iniciar", h, "", ck); w.Code != http.StatusForbidden {
		t.Errorf("POST sem Origin: %d", w.Code)
	}
	if w := pedir(c, "POST", "/abrir/../../etc", h, "http://"+h, ck); w.Code != http.StatusNotFound {
		t.Errorf("item fora da pasta: %d", w.Code)
	}
	if w := pedir(c, "GET", "/pagina/nao-existe", h, "", ck); w.Code != http.StatusNotFound {
		t.Errorf("página desconhecida: %d", w.Code)
	}
	if w := pedir(c, "GET", "/estatico/central.css", h, "", ck); w.Code != http.StatusOK {
		t.Errorf("css: %d", w.Code)
	}
	if w := pedir(c, "GET", "/estatico/inicio.html", h, "", ck); w.Code != http.StatusNotFound {
		t.Errorf("modelo exposto: %d", w.Code)
	}
	// Coletar logs: a tarefa roda o script com --sem-janela.
	escrever(t, c.Scripts, "coletar_logs.sh", "#!/bin/sh\n[ \"$1\" = --sem-janela ] || exit 9\necho '# ok'\necho RESULTADO_ARQUIVO=/tmp/a.tar.gz\n")
	os.Chmod(filepath.Join(c.Scripts, "coletar_logs.sh"), 0o755)
	w = pedir(c, "POST", "/pagina/coletar-logs/iniciar", h, "http://"+h, ck)
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/tarefa/") {
		t.Fatalf("iniciar: %d %v", w.Code, w.Header())
	}
	loc := w.Header().Get("Location")
	var corpo string
	for i := 0; i < 100; i++ {
		corpo = pedir(c, "GET", loc+"/estado", h, "", ck).Body.String()
		if strings.Contains(corpo, `"fim":true`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(corpo, `"ok":true`) || !strings.Contains(corpo, "/tmp/a.tar.gz") {
		t.Errorf("tarefa: %s", corpo)
	}
}
