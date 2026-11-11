package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tarefa roda um script de trabalho e acompanha o andamento pelo mesmo protocolo do
// "yad --progress": linha "# texto" troca a mensagem, linha só com número é o percentual.
// Linhas "RESULTADO=valor" guardam o resultado (por exemplo o arquivo gerado).
// As demais linhas vão para o registro, que aparece se a tarefa falhar.
type Tarefa struct {
	ID     string
	Pagina string

	mu        sync.Mutex
	Percent   int
	Mensagem  string
	Registro  []string
	Resultado map[string]string
	Fim       bool
	OK        bool
}

type Estado struct {
	Percent   int               `json:"percentual"`
	Mensagem  string            `json:"mensagem"`
	Fim       bool              `json:"fim"`
	OK        bool              `json:"ok"`
	Registro  string            `json:"registro,omitempty"`
	Resultado map[string]string `json:"resultado,omitempty"`
}

func (t *Tarefa) Estado() Estado {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := Estado{Percent: t.Percent, Mensagem: t.Mensagem, Fim: t.Fim, OK: t.OK, Resultado: t.Resultado}
	if t.Fim && !t.OK {
		e.Registro = strings.Join(t.Registro, "\n")
	}
	return e
}

func (t *Tarefa) linha(l string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	l = strings.TrimRight(l, "\r")
	switch {
	case strings.HasPrefix(l, "#"):
		t.Mensagem = strings.TrimSpace(strings.TrimPrefix(l, "#"))
	case strings.HasPrefix(l, "RESULTADO_") && strings.Contains(l, "="):
		k, v, _ := strings.Cut(strings.TrimPrefix(l, "RESULTADO_"), "=")
		t.Resultado[strings.ToLower(k)] = v
	default:
		if n, err := strconv.Atoi(strings.TrimSpace(l)); err == nil && n >= 0 && n <= 100 {
			t.Percent = n
			return
		}
		if strings.TrimSpace(l) != "" {
			t.Registro = append(t.Registro, l)
			if len(t.Registro) > 200 {
				t.Registro = t.Registro[len(t.Registro)-200:]
			}
		}
	}
}

type Tarefas struct {
	mu    sync.Mutex
	todas map[string]*Tarefa
}

func novoID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (ts *Tarefas) Obter(id string) *Tarefa {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.todas[id]
}

// Iniciar roda argv em segundo plano (stdout e stderr juntos) com limite de tempo.
func (ts *Tarefas) Iniciar(pagina string, argv []string, limite time.Duration) *Tarefa {
	t := &Tarefa{ID: novoID(), Pagina: pagina, Mensagem: "Iniciando...", Resultado: map[string]string{}}
	ts.mu.Lock()
	if ts.todas == nil {
		ts.todas = map[string]*Tarefa{}
	}
	ts.todas[t.ID] = t
	ts.mu.Unlock()
	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), limite)
		defer cancelar()
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		saida, err := cmd.StdoutPipe()
		if err == nil {
			cmd.Stderr = cmd.Stdout
			err = cmd.Start()
		}
		if err == nil {
			s := bufio.NewScanner(saida)
			s.Buffer(make([]byte, 64*1024), 1024*1024)
			for s.Scan() {
				t.linha(s.Text())
			}
			err = cmd.Wait()
		}
		t.mu.Lock()
		t.Fim = true
		t.OK = err == nil
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				t.Registro = append(t.Registro, "tempo esgotado")
			} else {
				t.Registro = append(t.Registro, err.Error())
			}
		} else {
			t.Percent = 100
		}
		t.mu.Unlock()
	}()
	return t
}
