package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcharIcone(t *testing.T) {
	raiz := t.TempDir()
	antes := raizIcones
	raizIcones = raiz
	defer func() { raizIcones = antes }()
	criar := func(rel string) string {
		c := filepath.Join(raiz, rel)
		os.MkdirAll(filepath.Dir(c), 0o755)
		os.WriteFile(c, []byte("x"), 0o644)
		return c
	}
	criar("Papirus/16x16/apps/teste-central-a.svg")
	quer := criar("Papirus/48x48/apps/teste-central-a.svg")
	criar("Papirus/48x48/apps/teste-central-a-symbolic.svg")
	// Pasta-link do Papirus (48x48@2x → 48x48) e ícone só no hicolor, em png
	os.Symlink("48x48", filepath.Join(raiz, "Papirus/48x48@2x"))
	soHicolor := criar("hicolor/64x64/apps/teste-central-b.png")

	if c := acharIcone("teste-central-a"); c != quer {
		t.Errorf("Papirus 48: %q", c)
	}
	if c := acharIcone("teste-central-b"); c != soHicolor {
		t.Errorf("hicolor: %q", c)
	}
	if c := acharIcone("teste-central-nenhum"); c != "" {
		t.Errorf("inexistente: %q", c)
	}
	// Segunda busca vem do cache, mesmo com o arquivo removido
	os.Remove(quer)
	if c := acharIcone("teste-central-a"); c != quer {
		t.Errorf("cache: %q", c)
	}
	for _, n := range []string{"", "a*", "../x"} {
		if c := acharIcone(n); c != "" {
			t.Errorf("%q: %q", n, c)
		}
	}
}
