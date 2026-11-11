// Central do Suporte SEDUC em web: a mesma central do central_suporte.sh (itens de
// /usr/local/seduc/central.d), numa janela de aplicativo do navegador, como o BigLinux faz
// com o BigBashView. Roda como o próprio usuário; o que precisa de root continua nos
// scripts da imagem, com o pkexec deles.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var Versao = "0.1.0-dev"

func main() {
	porta := flag.Int("porta", 0, "porta local (0 = livre)")
	semJanela := flag.Bool("sem-janela", false, "não abre a janela; só mostra o endereço de entrada")
	itens := flag.String("itens", "/usr/local/seduc/central.d", "pasta dos itens (.desktop)")
	scripts := flag.String("scripts", "/usr/local/seduc/scripts", "pasta dos scripts de trabalho")
	adConf := flag.String("ad-conf", "/usr/local/seduc/ad/ad.conf", "configuração do AD (GRUPOS_ADMIN)")
	versao := flag.Bool("version", false, "mostra a versão")
	flag.Parse()
	if *versao {
		fmt.Println("central-suporte", Versao)
		return
	}
	c := &Central{Itens: *itens, Scripts: *scripts, AdConf: *adConf, Versao: Versao}
	l, err := c.Ouvir(*porta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "central-suporte: não foi possível abrir a porta local:", err)
		os.Exit(1)
	}
	srv := &http.Server{Handler: c, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(l)
	go c.Vigiar()
	fmt.Println("central=" + c.URLEntrada())
	if !*semJanela {
		if err := abrirJanela(c.URLEntrada()); err != nil {
			fmt.Fprintln(os.Stderr, "central-suporte: não foi possível abrir a janela:", err)
		}
	}
	<-c.fim
	_ = srv.Close()
}

// abrirJanela abre como janela de aplicativo (sem barra de endereço) no Chrome ou Chromium
// da imagem; sem eles, no navegador padrão.
func abrirJanela(url string) error {
	for _, nav := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if caminho, err := exec.LookPath(nav); err == nil {
			cmd := exec.Command(caminho, "--app="+url, "--class=central-suporte-seduc", "--no-first-run", "--no-default-browser-check")
			if err := cmd.Start(); err == nil {
				go cmd.Wait()
				return nil
			}
		}
	}
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
