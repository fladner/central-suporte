package main

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Programas com página web própria (X-SEDUC-Mesma-Janela=true), como a tela do VDE-CE:
// a Central roda o Exec com --sem-navegador --voltar <endereço da Central>, lê na saída
// a linha <nome>=http://127.0.0.1:<porta>/... e leva a mesma janela para lá. O botão de
// volta do programa traz a janela de volta para a Central.

// esperaExterno: quanto a Central espera o programa dizer o endereço da página dele.
var esperaExterno = 20 * time.Second

// enderecoLocal aceita só uma página http em 127.0.0.1 com porta.
func enderecoLocal(s string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil {
		return "", false
	}
	if p, err := strconv.Atoi(u.Port()); err != nil || p < 1 || p > 65535 {
		return "", false
	}
	return u.String(), true
}

// lerEndereco procura na saída a primeira linha <nome>=<endereço local> e depois continua
// lendo (e descartando) a saída, para o programa nunca travar escrevendo.
func lerEndereco(saida io.Reader, achou chan<- string) {
	s := bufio.NewScanner(saida)
	enviado := false
	for s.Scan() {
		if enviado {
			continue
		}
		if _, valor, ok := strings.Cut(s.Text(), "="); ok {
			if e, ok := enderecoLocal(valor); ok {
				achou <- e
				enviado = true
			}
		}
	}
	io.Copy(io.Discard, saida)
	close(achou)
}

func (c *Central) externo(w http.ResponseWriter, r *http.Request, id string) {
	it, ok := c.item(id)
	if !ok || !it.Mesma || it.Exec == "" {
		http.NotFound(w, r)
		return
	}
	argv := argumentosExec(it.Exec)
	if len(argv) == 0 {
		http.NotFound(w, r)
		return
	}
	argv = append(argv, "--sem-navegador", "--voltar", "http://"+c.endereco+"/")
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	saida, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		c.erroInicio(w, "Não foi possível abrir "+it.Nome+".")
		return
	}
	// Enquanto o programa roda, a janela está nele: a Central não se encerra por falta do sinal.
	c.mu.Lock()
	c.externos++
	c.mu.Unlock()
	achou := make(chan string, 1)
	go lerEndereco(saida, achou)
	go func() {
		cmd.Wait()
		c.mu.Lock()
		c.externos--
		c.ultimo = time.Now()
		c.mu.Unlock()
	}()
	select {
	case e, ok := <-achou:
		if ok {
			http.Redirect(w, r, e, http.StatusSeeOther)
			return
		}
	case <-time.After(esperaExterno):
	}
	c.erroInicio(w, "Não foi possível abrir "+it.Nome+" nesta janela.")
}

func (c *Central) erroInicio(w http.ResponseWriter, msg string) {
	d := c.base("Central do Suporte SEDUC")
	d.Itens = listarItens(c.Itens, d.Admin)
	d.Erro = msg
	c.mostrar(w, "inicio.html", d)
}
