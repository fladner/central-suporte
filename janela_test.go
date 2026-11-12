package main

import (
	"reflect"
	"testing"
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
