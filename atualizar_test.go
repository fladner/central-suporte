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

// Atualização automática: sem o timer (antes da 1.7-0) não aparece; com ele, mostra o
// último resultado do atualizar_pacotes.sh.
func TestStatusAutomatico(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "20-at.desktop", "[Desktop Entry]\nName=Atualizar o sistema\nExec=/bin/true\nX-SEDUC-Web=atualizar\nX-SEDUC-Acesso=todos\n")
	d := t.TempDir()
	cmdlineKernel = filepath.Join(d, "cmdline")
	escrever(t, d, "cmdline", "quiet splash\n")
	timerAuto = filepath.Join(d, "seduc-atualizar.timer")
	statusAuto = filepath.Join(d, "status")
	ck := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil).Result().Cookies()[0]
	pagina := func() string { return pedir(c, "GET", "/pagina/atualizar", c.endereco, "", ck).Body.String() }

	if p := pagina(); strings.Contains(p, "Atualização automática") {
		t.Errorf("sem o timer, mostrou a atualização automática")
	}
	escrever(t, d, "seduc-atualizar.timer", "")
	if p := pagina(); !strings.Contains(p, "Ainda não rodou") {
		t.Errorf("sem status: %s", p)
	}
	escrever(t, d, "status", "DATA_TEXTO=22/11/2026 10:15\nORIGEM=automatica\nRESULTADO=parcial\nMENSAGEM=2 pacote(s) ficaram para a próxima vez\nBLOQUEADOS=firefox google-chrome-stable\nREINICIAR=1\nULTIMO_OK_DATA=21/11/2026 09:40\n")
	p := pagina()
	for _, s := range []string{"Atualização incompleta", "22/11/2026 10:15", "automática", "firefox google-chrome-stable", "21/11/2026 09:40", "Reinicie"} {
		if !strings.Contains(p, s) {
			t.Errorf("faltou %q", s)
		}
	}
	escrever(t, d, "status", "DATA_TEXTO=22/11/2026 11:00\nORIGEM=manual\nRESULTADO=ok\nMENSAGEM=Todos os pacotes atualizados\nREINICIAR=0\nULTIMO_OK_DATA=22/11/2026 11:00\n")
	p = pagina()
	if !strings.Contains(p, "Tudo atualizado") || !strings.Contains(p, "pelo Atualizar o sistema") || strings.Contains(p, "Última atualização completa") || strings.Contains(p, "Reinicie") {
		t.Errorf("ok: %s", p)
	}
}
