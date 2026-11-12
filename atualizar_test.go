package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Atualizar o sistema: etapa "==> " vira a mensagem, a saída aparece enquanto roda e o
// botão de reiniciar só vale quando o sistema pede reinício.
func TestPaginaAtualizar(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "20-at.desktop", "[Desktop Entry]\nName=Atualizar o sistema\nExec=/bin/true\nX-SEDUC-Web=atualizar\nX-SEDUC-Acesso=todos\n")
	d := t.TempDir()
	liberar := filepath.Join(d, "liberar")
	cmdlineKernel = filepath.Join(d, "cmdline")
	escrever(t, d, "cmdline", "quiet splash\n")
	reinicioPendente = filepath.Join(d, "reboot-required")
	executavelTeste(t, filepath.Join(c.Scripts, "atualizar_sistema_root.sh"), `echo; echo "==> Buscando atualizações"; echo "Obter:1 http://br.archive.ubuntu.com"
while [ ! -f `+liberar+` ]; do sleep 0.1; done
echo "==> Instalando atualizações"
`)
	falso := filepath.Join(d, "pkexec")
	executavelTeste(t, falso, `exec "$@"`)
	pkexec = falso
	reiniciado := false
	reiniciar = func() error { reiniciado = true; return nil }

	ck := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil).Result().Cookies()[0]
	w := postarForm(c, "/pagina/atualizar/iniciar", url.Values{}, ck)
	if w.Code != 303 {
		t.Fatalf("iniciar: %d %s", w.Code, w.Body.String())
	}
	local := w.Header().Get("Location")
	tf := c.tarefas.Obter(strings.TrimPrefix(local, "/tarefa/"))
	var e Estado
	for i := 0; i < 50 && !strings.Contains(e.Saida, "Obter:1"); i++ {
		time.Sleep(100 * time.Millisecond)
		e = tf.Estado()
	}
	if e.Fim || e.Mensagem != "Buscando atualizações" || !strings.Contains(e.Saida, "Obter:1") || c.tarefas.Ativas() != 1 {
		t.Fatalf("enquanto roda: %+v (ativas %d)", e, c.tarefas.Ativas())
	}
	if w := postarForm(c, local+"/reiniciar", nil, ck); w.Code != 404 {
		t.Errorf("reiniciar antes do fim: %d", w.Code)
	}
	escrever(t, d, "reboot-required", "")
	escrever(t, d, "liberar", "")
	e = esperarTarefa(t, c, ck, local)
	if !e.OK || e.Resultado["reiniciar"] != "1" || !strings.Contains(e.Aviso, "Reinicie") || c.tarefas.Ativas() != 0 {
		t.Fatalf("fim: %+v", e)
	}
	if w := postarForm(c, local+"/reiniciar", nil, ck); w.Code != 204 || !reiniciado {
		t.Errorf("reiniciar: %d %v", w.Code, reiniciado)
	}
	// Sem reinício pendente, o botão não serve.
	os.Remove(reinicioPendente)
	if _, aviso := fimAtualizar(0, map[string]string{}); aviso != "Sistema atualizado." {
		t.Errorf("aviso: %q", aviso)
	}
	escrever(t, d, "cmdline", "boot=casper\n")
	if w := postarForm(c, "/pagina/atualizar/iniciar", url.Values{}, ck); !strings.Contains(w.Body.String(), "modo live") {
		t.Errorf("live aceito: %d", w.Code)
	}
}
