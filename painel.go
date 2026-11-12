package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// listarPainel roda "<painel> --listar" da imagem e devolve as linhas já separadas por "|".
// A lista de opções fica no painel (um lugar só para o yad e para a web).
func listarPainel(c *Central, script string) ([][]string, error) {
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	saida, err := exec.CommandContext(ctx, filepath.Join(c.Scripts, script), "--listar").Output()
	if err != nil {
		return nil, err
	}
	var linhas [][]string
	for _, l := range strings.Split(strings.TrimSpace(string(saida)), "\n") {
		if l != "" {
			linhas = append(linhas, strings.Split(l, "|"))
		}
	}
	return linhas, nil
}

// lerRelatorio lê um relatório "item|situação" que o script root deixa em logs.
func lerRelatorio(caminho string) [][2]string {
	var rel [][2]string
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if item, sit, ok := strings.Cut(l, "|"); ok {
			rel = append(rel, [2]string{item, sit})
		}
	}
	return rel
}

// senhaRecusada: o pkexec devolve 126/127 quando a senha do administrador não é confirmada.
func senhaRecusada(codigo int) bool { return codigo == 126 || codigo == 127 }

// semQuebra recusa valores com quebra de linha ou que comecem com "-" (viraria opção).
func semQuebra(s string) bool {
	return !strings.ContainsAny(s, "\r\n\x00") && !strings.HasPrefix(s, "-")
}
