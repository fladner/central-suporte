package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const scriptTeste = `case "$1" in
pagina) echo "<p>pagina $CENTRAL_PAGINA admin=$CENTRAL_ADMIN busca=$F_busca vazado=${F_vazado:-} url=$CENTRAL_URL</p>" ;;
salvar) echo "<p>nome=[$F_nome] lista=[$F_lista] estranho=[${F_X:-}]</p>" ;;
quieto) : ;;
quebra) echo "<b>detalhe</b>" >&2; exit 3 ;;
longa) echo "# Passo 1"; echo 50; echo "linha de saída"; echo "RESULTADO_AVISO=Tudo certo" ;;
*) exit 9 ;;
esac
`

func centralBash(t *testing.T) (*Central, string) {
	t.Helper()
	c := novaCentral(t)
	escrever(t, c.Itens, "65-teste.desktop", "[Desktop Entry]\nName=Página de teste\nX-SEDUC-Bash=teste\nX-SEDUC-Acesso=todos\n")
	pasta := filepath.Join(c.Itens, "paginas")
	if err := os.Mkdir(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(pasta, "teste.sh")
	if err := os.WriteFile(script, []byte(scriptTeste), 0o644); err != nil {
		t.Fatal(err)
	}
	return c, script
}

func TestPaginaBash(t *testing.T) {
	t.Setenv("F_vazado", "nao")
	c, script := centralBash(t)
	w := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil)
	ck := w.Result().Cookies()[0]

	if corpo := pedir(c, "GET", "/", c.endereco, "", ck).Body.String(); !strings.Contains(corpo, `href="/bash/teste"`) {
		t.Fatalf("item no início: %s", corpo)
	}
	corpo := pedir(c, "GET", "/bash/teste?busca=a%20b", c.endereco, "", ck).Body.String()
	if !strings.Contains(corpo, "<p>pagina teste admin=") || !strings.Contains(corpo, "busca=a b vazado= url=/bash/teste") || !strings.Contains(corpo, "<h1>Página de teste</h1>") {
		t.Errorf("página: %s", corpo)
	}

	// Formulário: campos em F_, nomes inválidos ignorados, vários valores numa linha cada
	f := url.Values{"nome": {"Ana"}, "lista": {"a", "b"}, "X": {"x"}}
	if w := postarForm(c, "/bash/teste/salvar", f, ck); w.Code != 200 || !strings.Contains(w.Body.String(), "nome=[Ana] lista=[a\nb] estranho=[]") {
		t.Errorf("salvar: %d %s", w.Code, w.Body.String())
	}
	// Sem saída: volta para a página
	if w := postarForm(c, "/bash/teste/quieto", nil, ck); w.Code != 303 || w.Header().Get("Location") != "/bash/teste" {
		t.Errorf("quieto: %d %s", w.Code, w.Header().Get("Location"))
	}
	// Erro: código e detalhe (escapado)
	if w := postarForm(c, "/bash/teste/quebra", nil, ck); !strings.Contains(w.Body.String(), "(código 3)") || !strings.Contains(w.Body.String(), "&lt;b&gt;detalhe&lt;/b&gt;") {
		t.Errorf("quebra: %s", w.Body.String())
	}
	// Ações inválidas, GET em ação, página que não existe
	for _, caminho := range []string{"/bash/teste/..%2Fx", "/bash/teste/Maiuscula", "/bash/outra", "/bash/teste/pagina"} {
		if w := postarForm(c, caminho, nil, ck); w.Code != 404 {
			t.Errorf("POST %s: %d", caminho, w.Code)
		}
	}
	if w := pedir(c, "GET", "/bash/teste/salvar", c.endereco, "", ck); w.Code != 404 {
		t.Errorf("GET ação: %d", w.Code)
	}
	// POST de outra origem
	if w := pedir(c, "POST", "/bash/teste/salvar", c.endereco, "http://evil.example", ck); w.Code != 403 {
		t.Errorf("origem: %d", w.Code)
	}

	// Tarefa: andamento, saída ao vivo e aviso do fim
	w = postarForm(c, "/bash/teste/tarefa/longa", nil, ck)
	destino := w.Header().Get("Location")
	if w.Code != 303 || !strings.HasPrefix(destino, "/tarefa/") {
		t.Fatalf("tarefa: %d %s", w.Code, destino)
	}
	tf := c.tarefas.Obter(strings.TrimPrefix(destino, "/tarefa/"))
	for i := 0; i < 100 && !tf.Estado().Fim; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if e := tf.Estado(); !e.OK || e.Aviso != "Tudo certo" || !strings.Contains(e.Saida, "linha de saída") {
		t.Errorf("estado: %+v", e)
	}
	if corpo := pedir(c, "GET", destino, c.endereco, "", ck).Body.String(); !strings.Contains(corpo, "data-tarefa=") || !strings.Contains(corpo, `href="/bash/teste"`) {
		t.Errorf("página da tarefa: %s", corpo)
	}
	if w := postarForm(c, destino+"/qualquer", nil, ck); w.Code != 404 {
		t.Errorf("ação na tarefa bash: %d", w.Code)
	}

	// Script gravável por outros: a página some
	if err := os.Chmod(script, 0o666); err != nil {
		t.Fatal(err)
	}
	if w := pedir(c, "GET", "/bash/teste", c.endereco, "", ck); w.Code != 404 {
		t.Errorf("inseguro: %d", w.Code)
	}
	if corpo := pedir(c, "GET", "/", c.endereco, "", ck).Body.String(); strings.Contains(corpo, "Página de teste") {
		t.Errorf("item inseguro no início")
	}
}

func TestScriptBash(t *testing.T) {
	d := t.TempDir()
	if _, ok := scriptBash(d, "x"); ok {
		t.Error("sem pasta")
	}
	os.Mkdir(filepath.Join(d, "paginas"), 0o755)
	os.WriteFile(filepath.Join(d, "paginas", "x.sh"), []byte("true"), 0o644)
	if _, ok := scriptBash(d, "x"); !ok {
		t.Error("válido")
	}
	for _, nome := range []string{"", "../x", "X", "a/b", strings.Repeat("a", 41)} {
		if _, ok := scriptBash(d, nome); ok {
			t.Errorf("nome %q", nome)
		}
	}
	// Link simbólico não vale
	os.Symlink(filepath.Join(d, "paginas", "x.sh"), filepath.Join(d, "paginas", "y.sh"))
	if _, ok := scriptBash(d, "y"); ok {
		t.Error("link")
	}
	os.Chmod(filepath.Join(d, "paginas"), 0o775)
	if _, ok := scriptBash(d, "x"); ok {
		t.Error("pasta gravável pelo grupo")
	}
}
