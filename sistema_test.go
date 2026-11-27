package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Cabeçalho: versão do Xubuntu SEDUC ou, sem ela (.deb do AD em outro sabor), o
// PRETTY_NAME do os-release.
func TestLerSistema(t *testing.T) {
	d := t.TempDir()
	antesV, antesOS := arquivoVersaoSEDUC, arquivoOSRelease
	t.Cleanup(func() { arquivoVersaoSEDUC, arquivoOSRelease = antesV, antesOS })
	arquivoVersaoSEDUC = filepath.Join(d, "version")
	arquivoOSRelease = filepath.Join(d, "os-release")

	if err := os.WriteFile(arquivoOSRelease, []byte("NAME=\"Ubuntu\"\nPRETTY_NAME=\"Kubuntu 26.04 LTS\"\nID=ubuntu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, seduc := lerSistema(); s != "Kubuntu 26.04 LTS" || seduc {
		t.Errorf("outro sabor: %q %v", s, seduc)
	}
	if err := os.WriteFile(arquivoVersaoSEDUC, []byte("1.7-6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, seduc := lerSistema(); s != "1.7-6" || !seduc {
		t.Errorf("Xubuntu SEDUC: %q %v", s, seduc)
	}
	os.Remove(arquivoVersaoSEDUC)
	os.Remove(arquivoOSRelease)
	if s, seduc := lerSistema(); s != "?" || seduc {
		t.Errorf("sem os dois: %q %v", s, seduc)
	}
}
