package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Tamanho da janela da Central (o conteúdo tem no máximo 980 px de largura)
const (
	janelaLargura = 1000
	janelaAltura  = 720
	janelaMargem  = 60 // folga para o painel do Xfce e as bordas da janela
)

type monitor struct{ X, Y, L, A int }

var reMonitor = regexp.MustCompile(` connected (primary )?(\d+)x(\d+)\+(\d+)\+(\d+)`)

// monitorPrincipal lê a saída do "xrandr --query": o monitor marcado como primary ou,
// sem ele, o primeiro conectado. Com vários monitores lado a lado, a tela inteira do X
// é a soma deles, e o Chrome sem posição abre a janela no canto esquerdo.
func monitorPrincipal(saida string) (monitor, bool) {
	var primeiro *monitor
	for _, linha := range strings.Split(saida, "\n") {
		m := reMonitor.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
		mon := monitor{X: n(4), Y: n(5), L: n(2), A: n(3)}
		if mon.L == 0 || mon.A == 0 {
			continue
		}
		if m[1] != "" {
			return mon, true
		}
		if primeiro == nil {
			primeiro = &mon
		}
	}
	if primeiro != nil {
		return *primeiro, true
	}
	return monitor{}, false
}

// geometria: tamanho da janela (cabe no monitor) e posição para ficar no centro dele
func geometria(mon monitor) (l, a, x, y int) {
	l = min(janelaLargura, mon.L-janelaMargem)
	a = min(janelaAltura, mon.A-janelaMargem)
	x = mon.X + (mon.L-l)/2
	y = mon.Y + (mon.A-a)/2
	return
}

// argumentosJanela: opções do Chrome para a janela de aplicativo da Central. Usa um perfil
// próprio (--user-data-dir): com o Chrome do usuário já aberto, a janela iria para o
// processo dele, que ignora --window-size e --window-position.
func argumentosJanela(url, perfil, xrandr string) []string {
	args := []string{
		"--app=" + url,
		"--class=central-suporte-seduc",
		"--user-data-dir=" + perfil,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if mon, ok := monitorPrincipal(xrandr); ok {
		l, a, x, y := geometria(mon)
		args = append(args,
			"--window-size="+strconv.Itoa(l)+","+strconv.Itoa(a),
			"--window-position="+strconv.Itoa(x)+","+strconv.Itoa(y))
	}
	return args
}

// pastaPerfil: perfil do Chrome só da Central, em ~/.cache (não é dado do usuário)
func pastaPerfil() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "seduc-central-suporte", "chrome")
}

func saidaXrandr() string {
	out, err := exec.Command("xrandr", "--query").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// Janela nativa (janela/janela.py, WebKitGTK), como o BigBashView: sem o Chrome no caminho.
// O endereço de entrada vai pela entrada padrão. Se faltar o WebKitGTK (saída 3), a
// entrada não foi usada e a Central abre o Chrome. Fechar a janela encerra a Central
// (depois das tarefas em andamento).
var visualizador = "/usr/lib/seduc-central-suporte/janela.py"

const semWebKit = 3

// navegador: reserva sem o WebKitGTK (trocada nos testes)
var navegador = abrirNavegador

func argumentosNativa(xrandr string) []string {
	mon, ok := monitorPrincipal(xrandr)
	if !ok {
		return nil
	}
	l, a, x, y := geometria(mon)
	return []string{"--largura", strconv.Itoa(l), "--altura", strconv.Itoa(a), "--x", strconv.Itoa(x), "--y", strconv.Itoa(y)}
}

func (c *Central) abrirJanela(url string, chrome bool) error {
	if chrome || !executavel(visualizador) {
		return navegador(url)
	}
	cmd := exec.Command(visualizador, argumentosNativa(saidaXrandr())...)
	cmd.Stdin = strings.NewReader(url + "\n")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		return navegador(url)
	}
	go func() {
		err := cmd.Wait()
		// Sem WebKitGTK (saída 3) ou janela que caiu antes de abrir a Central (falha de
		// segmentação, outro erro): a Central continua no navegador. Depois de entrar, o
		// endereço de uso único já foi gasto e a Central encerra. Fechada pelo usuário,
		// a saída é 0.
		var saiu *exec.ExitError
		if errors.As(err, &saiu) {
			c.mu.Lock()
			usada := c.usada
			c.mu.Unlock()
			if !usada {
				if saiu.ExitCode() != semWebKit {
					fmt.Fprintln(os.Stderr, "central-suporte: a janela nativa caiu ("+saiu.String()+"); abrindo no navegador")
				}
				if navegador(url) == nil {
					return
				}
			}
		}
		c.janelaFechada()
	}()
	return nil
}

// janelaFechada: a Central encerra no próximo giro do Vigiar, sem esperar o sinal.
func (c *Central) janelaFechada() {
	c.mu.Lock()
	c.fechada = true
	c.mu.Unlock()
}
