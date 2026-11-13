package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Temas procurados, na ordem (o Xubuntu SEDUC usa o Papirus).
var temasIcone = []string{"Papirus", "elementary-xfce", "Adwaita", "hicolor"}

var raizIcones = "/usr/share/icons"

// Ícones já procurados (nome → arquivo): a página pede todos a cada abertura.
var cacheIcones sync.Map

// acharIcone devolve o arquivo de um ícone do tema (nome como no .desktop) ou um caminho
// absoluto existente. Prefere o tamanho 48 e depois o vetorial.
func acharIcone(nome string) string {
	if v, ok := cacheIcones.Load(nome); ok {
		return v.(string)
	}
	c := buscarIcone(nome)
	cacheIcones.Store(nome, c)
	return c
}

// aquecerIcones procura de antemão os ícones dos itens, enquanto a janela abre.
func aquecerIcones(pasta string) {
	for _, it := range listarItens(pasta, true) {
		acharIcone(it.Icone)
	}
}

func buscarIcone(nome string) string {
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
			achados = append(achados, candidatos(filepath.Join(raiz, tema), nome)...)
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

// candidatos: arquivos <tema>/<tamanho>/<categoria>/<nome>.svg|png e <tema>/<tamanho>/<nome>.svg|png
// que existem. Testa cada caminho direto: o filepath.Glob leria por inteiro as pastas do
// Papirus, com milhares de ícones cada, e levava meio segundo por ícone.
func candidatos(tema, nome string) []string {
	var tamanhos, categorias []string
	for _, t := range subpastas(tema) {
		tamanhos = append(tamanhos, t)
		categorias = append(categorias, subpastas(t)...)
	}
	var achados []string
	for _, grupo := range [][]string{categorias, tamanhos} {
		for _, ext := range []string{".svg", ".png"} {
			for _, d := range grupo {
				if c := filepath.Join(d, nome+ext); ehArquivo(c) {
					achados = append(achados, c)
				}
			}
		}
	}
	return achados
}

// subpastas (seguindo links, como 48x48@2x → 48x48), em ordem alfabética
func subpastas(dir string) []string {
	entradas, _ := os.ReadDir(dir)
	var r []string
	for _, e := range entradas {
		c := filepath.Join(dir, e.Name())
		if e.IsDir() || (e.Type()&os.ModeSymlink != 0 && ehPasta(c)) {
			r = append(r, c)
		}
	}
	return r
}

func ehPasta(c string) bool {
	st, err := os.Stat(c)
	return err == nil && st.IsDir()
}

func ehArquivo(c string) bool {
	st, err := os.Stat(c)
	return err == nil && !st.IsDir()
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
