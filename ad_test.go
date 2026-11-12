package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func esperarTarefa(t *testing.T, c *Central, ck *http.Cookie, local string) Estado {
	t.Helper()
	id := strings.TrimPrefix(local, "/tarefa/")
	var e Estado
	for i := 0; i < 50 && !e.Fim; i++ {
		time.Sleep(100 * time.Millisecond)
		json.NewDecoder(pedir(c, "GET", "/tarefa/"+id+"/estado", c.endereco, "", ck).Body).Decode(&e)
	}
	return e
}

// Active Directory: a senha do AD vai pela entrada padrão (nunca no argv) e não volta
// para o formulário depois de um erro.
func TestPaginaAD(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "30-ad.desktop", "[Desktop Entry]\nName=Active Directory SEDUC\nExec=/bin/true\nX-SEDUC-Web=ad\nX-SEDUC-Acesso=todos\n")
	d := t.TempDir()
	args, entrada := filepath.Join(d, "args"), filepath.Join(d, "entrada")
	relatorioAD = filepath.Join(d, "relatorio")
	executavelTeste(t, filepath.Join(c.Scripts, "painel_ad.sh"), `[ "$1" = --listar ] || exit 9
echo "DOMINIO|seduc.ce.gov.br"
echo "COMPUTADOR|HOMOLOG-01"
`)
	executavelTeste(t, filepath.Join(c.Scripts, "ingressar_ad.sh"), `printf '%s\n' "$@" > `+args+`
cat > `+entrada+`
echo 5; echo "# Procurando os controladores"
echo "Rede|FALHA: o DNS não encontra o domínio" > `+relatorioAD+`
exit 2
`)
	falso := filepath.Join(d, "pkexec")
	executavelTeste(t, falso, `exec "$@"`)
	pkexec = falso

	ck := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil).Result().Cookies()[0]
	if w := pedir(c, "GET", "/pagina/ad", c.endereco, "", ck); !strings.Contains(w.Body.String(), `value="HOMOLOG-01"`) || strings.Contains(w.Body.String(), "Sair do domínio") {
		t.Fatalf("formulário: %s", w.Body.String())
	}
	for _, f := range []url.Values{
		{"acao": {"entrar"}, "usuario": {"adm"}, "senha": {""}, "computador": {"PC1"}},
		{"acao": {"entrar"}, "usuario": {"-x"}, "senha": {"s3nh@"}, "computador": {"PC1"}},
		{"acao": {"entrar"}, "usuario": {"adm"}, "senha": {"s3nh@"}, "computador": {"NOME-COMPRIDO-DEMAIS"}},
		{"acao": {"entrar"}, "usuario": {"adm"}, "senha": {"a\nb"}, "computador": {"PC1"}},
		{"acao": {"sair"}, "usuario": {"adm"}, "senha": {"s3nh@"}},
	} {
		w := postarForm(c, "/pagina/ad/iniciar", f, ck)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `class="erro"`) {
			t.Errorf("%v aceito: %d", f, w.Code)
		}
		if strings.Contains(w.Body.String(), "s3nh@") {
			t.Errorf("a senha voltou para a página")
		}
	}
	w := postarForm(c, "/pagina/ad/iniciar", url.Values{"acao": {"entrar"}, "usuario": {"adm.ti"}, "senha": {"-s3nh@ com espaço"}, "computador": {"pc-lab1"}, "ou": {"Escolas/Computadores"}}, ck)
	if w.Code != 303 {
		t.Fatalf("iniciar: %d %s", w.Code, w.Body.String())
	}
	e := esperarTarefa(t, c, ck, w.Header().Get("Location"))
	if !e.Fim || e.OK || len(e.Relatorio) != 1 || !strings.Contains(e.Aviso, "Algo deu errado") {
		t.Fatalf("estado: %+v", e)
	}
	b, _ := os.ReadFile(args)
	if got := string(b); got != "entrar\nadm.ti\nPC-LAB1\nEscolas/Computadores\n" || strings.Contains(got, "s3nh") {
		t.Errorf("argumentos: %q", got)
	}
	if b, _ := os.ReadFile(entrada); string(b) != "-s3nh@ com espaço\n" {
		t.Errorf("entrada: %q", b)
	}
	if _, aviso := fimAD("entrar")(126, nil); !strings.Contains(aviso, "Nada foi alterado") {
		t.Errorf("senha local recusada: %q", aviso)
	}
}

// Impressoras: lista do painel, senha do Kerberos pela entrada padrão, só filas da lista.
func TestPaginaImpressoras(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "40-imp.desktop", "[Desktop Entry]\nName=Impressoras SEDUC\nExec=/bin/true\nX-SEDUC-Web=impressoras\nX-SEDUC-Acesso=todos\n")
	d := t.TempDir()
	ticket, args, padrao := filepath.Join(d, "ticket"), filepath.Join(d, "args"), filepath.Join(d, "padrao")
	executavelTeste(t, filepath.Join(c.Scripts, "painel_impressora_ad.sh"), `case "$1" in
--listar) echo "SERVIDOR|srv.seduc"; [ -f `+ticket+` ] || { echo SEM_TICKET; exit 0; }
  echo "FILA|LAB 1|Laser do lab|0"; echo "FILA|SECRETARIA|Jato|1" ;;
--kinit) read -r S; [ "$S" = "certa" ] && touch `+ticket+` ;;
--padrao) echo "$2" > `+padrao+` ;;
esac
`)
	executavelTeste(t, filepath.Join(c.Scripts, "instalar_impressora_ad.sh"), `printf '%s\n' "$@" > `+args+`
`)
	falso := filepath.Join(d, "pkexec")
	executavelTeste(t, falso, `exec "$@"`)
	pkexec = falso

	ck := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil).Result().Cookies()[0]
	if w := pedir(c, "GET", "/pagina/impressoras", c.endereco, "", ck); !strings.Contains(w.Body.String(), `action="/pagina/impressoras/kerberos"`) {
		t.Fatalf("sem pedir a senha: %s", w.Body.String())
	}
	if w := postarForm(c, "/pagina/impressoras/kerberos", url.Values{"senha": {"errada"}}, ck); !strings.Contains(w.Body.String(), "Senha recusada") {
		t.Errorf("senha errada aceita: %d", w.Code)
	}
	if w := postarForm(c, "/pagina/impressoras/kerberos", url.Values{"senha": {"certa"}}, ck); w.Code != 303 {
		t.Fatalf("kerberos: %d", w.Code)
	}
	if w := pedir(c, "GET", "/pagina/impressoras", c.endereco, "", ck); !strings.Contains(w.Body.String(), `value="LAB 1"`) {
		t.Fatalf("lista: %s", w.Body.String())
	}
	if w := postarForm(c, "/pagina/impressoras/iniciar", url.Values{"acao": {"adicionar"}, "fila": {"OUTRA"}}, ck); !strings.Contains(w.Body.String(), "desconhecida") {
		t.Errorf("fila de fora aceita: %d", w.Code)
	}
	w := postarForm(c, "/pagina/impressoras/iniciar", url.Values{"acao": {"adicionar"}, "fila": {"LAB 1", "SECRETARIA"}}, ck)
	if w.Code != 303 {
		t.Fatalf("iniciar: %d %s", w.Code, w.Body.String())
	}
	e := esperarTarefa(t, c, ck, w.Header().Get("Location"))
	if !e.OK || len(e.Relatorio) != 2 || !strings.Contains(e.Aviso, "padrão") {
		t.Fatalf("estado: %+v", e)
	}
	if b, _ := os.ReadFile(args); string(b) != "adicionar\nLAB 1\nSECRETARIA\n" {
		t.Errorf("argumentos: %q", b)
	}
	if b, _ := os.ReadFile(padrao); strings.TrimSpace(string(b)) != "LAB 1" {
		t.Errorf("padrão: %q", b)
	}
}

// O aviso do painel (ERRO|...) aparece na página.
func TestImpressorasMostraOAvisoDoPainel(t *testing.T) {
	c := novaCentral(t)
	escrever(t, c.Itens, "40-imp.desktop", "[Desktop Entry]\nName=Impressoras SEDUC\nExec=/bin/true\nX-SEDUC-Web=impressoras\nX-SEDUC-Acesso=todos\n")
	executavelTeste(t, filepath.Join(c.Scripts, "painel_impressora_ad.sh"), "echo 'ERRO|Só para usuários do domínio.'\n")
	ck := pedir(c, "GET", "/entrar/"+c.chave, c.endereco, "", nil).Result().Cookies()[0]
	if w := pedir(c, "GET", "/pagina/impressoras", c.endereco, "", ck); !strings.Contains(w.Body.String(), "Só para usuários do domínio.") {
		t.Fatalf("sem o aviso: %s", w.Body.String())
	}
}
