package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const xrandrTres = `Screen 0: minimum 320 x 200, current 5760 x 1080, maximum 16384 x 16384
HDMI-1 connected 1920x1080+0+0 (normal left inverted right x axis y axis) 527mm x 296mm
   1920x1080     60.00*+
eDP-1 connected primary 1920x1080+1920+0 (normal left inverted right x axis y axis) 344mm x 194mm
   1920x1080     60.01*+
DP-1 connected 1920x1080+3840+0 (normal left inverted right x axis y axis) 527mm x 296mm
DP-2 disconnected (normal left inverted right x axis y axis)
`

func TestMonitorPrincipal(t *testing.T) {
	if m, ok := monitorPrincipal(xrandrTres); !ok || m != (monitor{X: 1920, Y: 0, L: 1920, A: 1080}) {
		t.Errorf("primary: %+v %v", m, ok)
	}
	semPrimary := "VGA-1 connected 1366x768+0+0 (normal) 0mm x 0mm\nHDMI-1 connected 1920x1080+1366+0 (normal)\n"
	if m, ok := monitorPrincipal(semPrimary); !ok || m != (monitor{L: 1366, A: 768}) {
		t.Errorf("sem primary: %+v %v", m, ok)
	}
	// Monitor conectado mas desligado (sem geometria) e saída vazia
	if _, ok := monitorPrincipal("HDMI-1 connected (normal left inverted right)\n"); ok {
		t.Error("sem geometria não devia achar monitor")
	}
	if _, ok := monitorPrincipal(""); ok {
		t.Error("saída vazia não devia achar monitor")
	}
}

func TestGeometria(t *testing.T) {
	// Monitor do meio de três: janela 1000x720 no centro dele
	if l, a, x, y := geometria(monitor{X: 1920, L: 1920, A: 1080}); l != 1000 || a != 720 || x != 2380 || y != 180 {
		t.Errorf("1920x1080: %d %d %d %d", l, a, x, y)
	}
	// Tela pequena: a janela cabe nela
	if l, a, x, y := geometria(monitor{L: 1024, A: 600}); l != 964 || a != 540 || x != 30 || y != 30 {
		t.Errorf("1024x600: %d %d %d %d", l, a, x, y)
	}
}

func TestArgumentosJanela(t *testing.T) {
	got := argumentosJanela("http://127.0.0.1:1/entrar/x", "/p", xrandrTres)
	want := []string{"--app=http://127.0.0.1:1/entrar/x", "--class=central-suporte-seduc", "--user-data-dir=/p",
		"--no-first-run", "--no-default-browser-check", "--window-size=1000,720", "--window-position=2380,180"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args:\n%v\n%v", got, want)
	}
	// Sem xrandr: Chrome escolhe tamanho e posição
	if got := argumentosJanela("u", "/p", ""); len(got) != 5 {
		t.Errorf("sem xrandr: %v", got)
	}
}

func TestArgumentosNativa(t *testing.T) {
	got := argumentosNativa(xrandrTres)
	want := []string{"--largura", "1000", "--altura", "720", "--x", "2380", "--y", "180"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%v", got)
	}
	if got := argumentosNativa(""); got != nil {
		t.Errorf("sem xrandr: %v", got)
	}
}

// Janela nativa: o endereço vai pela entrada padrão (não no argv); fechar a janela marca a
// Central para encerrar; sem WebKitGTK (saída 3) abre o navegador com o mesmo endereço.
func TestAbrirJanela(t *testing.T) {
	d := t.TempDir()
	recebido := filepath.Join(d, "entrada")
	argv := filepath.Join(d, "argv")
	antes, antesNav := visualizador, navegador
	t.Cleanup(func() { visualizador, navegador = antes, antesNav })
	var noNavegador []string
	navegador = func(u string) error { noNavegador = append(noNavegador, u); return nil }

	esperar := func(c *Central, cond func() bool) {
		t.Helper()
		for i := 0; i < 100 && !cond(); i++ {
			time.Sleep(20 * time.Millisecond)
		}
	}
	visualizador = filepath.Join(d, "janela.py")
	executavelTeste(t, visualizador, `cat > `+recebido+`; echo "$@" > `+argv+"\n")
	c := &Central{}
	if err := c.abrirJanela("http://127.0.0.1:1/entrar/chave", false); err != nil {
		t.Fatal(err)
	}
	esperar(c, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.fechada })
	if b, _ := os.ReadFile(recebido); string(b) != "http://127.0.0.1:1/entrar/chave\n" {
		t.Errorf("entrada: %q", b)
	}
	if b, _ := os.ReadFile(argv); strings.Contains(string(b), "chave") {
		t.Errorf("endereço no argv: %q", b)
	}
	if !c.fechada || len(noNavegador) != 0 {
		t.Errorf("fechada=%v navegador=%v", c.fechada, noNavegador)
	}

	executavelTeste(t, visualizador, "exit 3\n")
	c = &Central{}
	_ = c.abrirJanela("http://127.0.0.1:1/entrar/outra", false)
	esperar(c, func() bool { return len(noNavegador) > 0 })
	if len(noNavegador) != 1 || noNavegador[0] != "http://127.0.0.1:1/entrar/outra" || c.fechada {
		t.Errorf("sem WebKit: navegador=%v fechada=%v", noNavegador, c.fechada)
	}

	// --chrome, ou sem o visualizador instalado: direto no navegador
	_ = c.abrirJanela("u1", true)
	visualizador = filepath.Join(d, "nao-existe")
	_ = c.abrirJanela("u2", false)
	if len(noNavegador) != 3 {
		t.Errorf("navegador: %v", noNavegador)
	}
}
