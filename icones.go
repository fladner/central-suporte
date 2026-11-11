package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Temas procurados, na ordem (o Xubuntu SEDUC usa o Papirus).
var temasIcone = []string{"Papirus", "elementary-xfce", "Adwaita", "hicolor"}

var raizIcones = "/usr/share/icons"

// acharIcone devolve o arquivo de um ícone do tema (nome como no .desktop) ou um caminho
// absoluto existente. Prefere o tamanho 48 e depois o vetorial.
func acharIcone(nome string) string {
	if nome == "" || strings.ContainsAny(nome, "*?[") {
		return ""
	}
	if filepath.IsAbs(nome) {
		if st, err := os.Stat(nome); err == nil && !st.IsDir() {
			return nome
		}
		return ""
	}
	if strings.Contains(nome, "/") {
		return ""
	}
	locais := []string{"/usr/local/share/icons", raizIcones}
	for _, tema := range temasIcone {
		var achados []string
		for _, raiz := range locais {
			for _, padrao := range []string{"*/*/%s.svg", "*/*/%s.png", "*/%s.svg", "*/%s.png"} {
				m, _ := filepath.Glob(filepath.Join(raiz, tema, strings.Replace(padrao, "%s", nome, 1)))
				achados = append(achados, m...)
			}
		}
		if c := melhorTamanho(achados); c != "" {
			return c
		}
	}
	for _, ext := range []string{".svg", ".png"} {
		if c := filepath.Join("/usr/share/pixmaps", nome+ext); existe(c) {
			return c
		}
	}
	return ""
}

func melhorTamanho(achados []string) string {
	for _, pref := range []string{"/48x48/", "/48/", "/scalable/", "/64x64/", "/64/"} {
		for _, a := range achados {
			if strings.Contains(a, pref) && !strings.Contains(a, "symbolic") {
				return a
			}
		}
	}
	for _, a := range achados {
		if !strings.Contains(a, "symbolic") {
			return a
		}
	}
	return ""
}

func existe(c string) bool {
	_, err := os.Stat(c)
	return err == nil
}
