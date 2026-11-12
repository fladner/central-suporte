package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web
var arquivosWeb embed.FS

var errSemResultado = errors.New("a tarefa não deixou resultado")

// Central é o servidor local da página. Segurança (como a tela do VDE-CE):
//   - só em 127.0.0.1, porta livre;
//   - a entrada é um endereço de uso único (/entrar/<chave>) que troca a chave por um
//     cookie HttpOnly; a chave na linha de comando do navegador não serve uma segunda vez;
//   - Host conferido em tudo e Origin em todo POST; CSP sem nada de fora;
//   - só abre itens de central.d (pelo nome do arquivo) e tarefas das páginas conhecidas.
type Central struct {
	Itens   string // pasta dos .desktop
	Scripts string // pasta dos scripts de trabalho
	AdConf  string
	Versao  string // versão da Central

	endereco string
	chave    string
	sessao   string
	usada    bool
	mu       sync.Mutex
	tarefas  Tarefas
	modelos  *template.Template
	ultimo   time.Time
	fim      chan struct{}
	fechar   sync.Once
	externos int // programas abertos na mesma janela (externos.go)
}

func aleatorio(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (c *Central) Ouvir(porta int) (net.Listener, error) {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(porta)))
	if err != nil {
		return nil, err
	}
	c.endereco = l.Addr().String()
	c.chave = aleatorio(24)
	c.sessao = aleatorio(32)
	c.ultimo = time.Now()
	c.fim = make(chan struct{})
	c.modelos = template.Must(template.ParseFS(arquivosWeb, "web/*.html"))
	return l, nil
}

func (c *Central) URLEntrada() string {
	return "http://" + c.endereco + "/entrar/" + c.chave
}

func (c *Central) encerrar() { c.fechar.Do(func() { close(c.fim) }) }

// Vigiar encerra depois de 15 minutos sem nenhum pedido; a página manda um sinal a cada
// 20 s, então fechar a janela encerra a Central em cerca de um minuto.
func (c *Central) Vigiar() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.fim:
			return
		case <-t.C:
			c.mu.Lock()
			parado := time.Since(c.ultimo)
			usada := c.usada
			externos := c.externos
			c.mu.Unlock()
			if externos == 0 && ((usada && parado > 70*time.Second) || parado > 15*time.Minute) {
				c.encerrar()
				return
			}
		}
	}
}

func (c *Central) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self' http://127.0.0.1:*; frame-ancestors 'none'; base-uri 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	// same-origin, não no-referrer: com no-referrer o Chrome manda "Origin: null" nos
	// formulários e a conferência do Origin recusaria todo POST da própria Central.
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Cache-Control", "no-store")
	if r.Host != c.endereco {
		http.Error(w, "endereço inválido", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost && r.Header.Get("Origin") != "http://"+c.endereco {
		http.Error(w, "origem inválida", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/entrar/") {
		c.entrar(w, r)
		return
	}
	ck, err := r.Cookie("central")
	if err != nil || subtle.ConstantTimeCompare([]byte(ck.Value), []byte(c.sessao)) != 1 {
		http.Error(w, "Abra a Central do Suporte pelo menu SEDUC.", http.StatusForbidden)
		return
	}
	c.mu.Lock()
	c.ultimo = time.Now()
	c.mu.Unlock()
	c.rotas(w, r)
}

func (c *Central) entrar(w http.ResponseWriter, r *http.Request) {
	chave := strings.TrimPrefix(r.URL.Path, "/entrar/")
	c.mu.Lock()
	valida := !c.usada && subtle.ConstantTimeCompare([]byte(chave), []byte(c.chave)) == 1
	if valida {
		c.usada = true
		c.ultimo = time.Now()
	}
	c.mu.Unlock()
	if !valida {
		http.Error(w, "Este endereço já foi usado. Abra a Central do Suporte pelo menu SEDUC.", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "central", Value: c.sessao, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *Central) rotas(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case p == "/" && r.Method == http.MethodGet:
		c.inicio(w)
	case p == "/sinal" && r.Method == http.MethodPost:
		w.WriteHeader(http.StatusNoContent)
	case p == "/sair" && r.Method == http.MethodPost:
		c.mostrar(w, "fim.html", c.base("Central encerrada"))
		go func() { time.Sleep(300 * time.Millisecond); c.encerrar() }()
	case strings.HasPrefix(p, "/estatico/") && r.Method == http.MethodGet:
		c.estatico(w, r, strings.TrimPrefix(p, "/estatico/"))
	case p == "/logo" && r.Method == http.MethodGet:
		c.arquivo(w, r, "/usr/share/pixmaps/seduc-logo.png")
	case strings.HasPrefix(p, "/icone/") && r.Method == http.MethodGet:
		c.icone(w, r, strings.TrimPrefix(p, "/icone/"))
	case strings.HasPrefix(p, "/externo/") && r.Method == http.MethodPost:
		c.externo(w, r, strings.TrimPrefix(p, "/externo/"))
	case strings.HasPrefix(p, "/abrir/") && r.Method == http.MethodPost:
		c.abrir(w, strings.TrimPrefix(p, "/abrir/"))
	case strings.HasPrefix(p, "/pagina/"):
		c.pagina(w, r, strings.TrimPrefix(p, "/pagina/"))
	case strings.HasPrefix(p, "/tarefa/"):
		c.tarefa(w, r, strings.TrimPrefix(p, "/tarefa/"))
	default:
		http.NotFound(w, r)
	}
}

type dados struct {
	Titulo, Computador, Sistema, Versao, Perfil string
	Admin                                       bool
	Itens                                       []Item
	Pagina                                      *Pagina
	NomePagina                                  string
	Tarefa                                      string
	Erro                                        string
	Extra                                       any
}

func (c *Central) base(titulo string) dados {
	nome, _ := os.Hostname()
	sistema := "?"
	if b, err := os.ReadFile("/usr/local/seduc/version"); err == nil {
		sistema = strings.TrimSpace(string(b))
	}
	admin := ehAdmin(c.AdConf)
	perfil := "usuário (as ferramentas de administrador ficam ocultas)"
	if admin {
		perfil = "administrador"
	}
	return dados{Titulo: titulo, Computador: nome, Sistema: sistema, Versao: c.Versao, Perfil: perfil, Admin: admin}
}

func (c *Central) mostrar(w http.ResponseWriter, modelo string, d dados) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.modelos.ExecuteTemplate(w, modelo, d); err != nil {
		http.Error(w, "erro ao montar a página", http.StatusInternalServerError)
	}
}

func (c *Central) inicio(w http.ResponseWriter) {
	d := c.base("Central do Suporte SEDUC")
	d.Itens = listarItens(c.Itens, d.Admin)
	c.mostrar(w, "inicio.html", d)
}

func (c *Central) item(id string) (Item, bool) {
	if strings.ContainsAny(id, "/\\") || id == "" {
		return Item{}, false
	}
	for _, it := range listarItens(c.Itens, ehAdmin(c.AdConf)) {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// abrir roda o Exec de um item ainda em yad, separado da Central (sessão própria).
func (c *Central) abrir(w http.ResponseWriter, id string) {
	it, ok := c.item(id)
	if !ok || it.Exec == "" {
		http.NotFound(w, nil)
		return
	}
	argv := argumentosExec(it.Exec)
	if len(argv) == 0 {
		http.NotFound(w, nil)
		return
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	go cmd.Wait()
	w.WriteHeader(http.StatusNoContent)
}

func (c *Central) pagina(w http.ResponseWriter, r *http.Request, resto string) {
	nome, acao, _ := strings.Cut(resto, "/")
	pg := paginas[nome]
	// A página só abre se algum item visível para este usuário a aponta.
	visivel := false
	if pg != nil {
		d := c.base("")
		for _, it := range listarItens(c.Itens, d.Admin) {
			if it.Web == nome {
				visivel = true
			}
		}
	}
	if !visivel {
		http.NotFound(w, r)
		return
	}
	switch {
	case acao == "" && r.Method == http.MethodGet:
		c.mostrarPagina(w, pg, nome, "")
	case acao == "iniciar" && r.Method == http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "formulário inválido", http.StatusBadRequest)
			return
		}
		cmd, err := pg.Iniciar(c, r)
		if err != nil {
			c.mostrarPagina(w, pg, nome, err.Error())
			return
		}
		t := c.tarefas.Iniciar(nome, cmd.Argv, cmd.Limite, cmd.Fim)
		http.Redirect(w, r, "/tarefa/"+t.ID, http.StatusSeeOther)
	default:
		http.NotFound(w, r)
	}
}

func (c *Central) mostrarPagina(w http.ResponseWriter, pg *Pagina, nome, erro string) {
	d := c.base(pg.Titulo)
	d.Pagina, d.NomePagina, d.Erro = pg, nome, erro
	if pg.Dados != nil {
		d.Extra = pg.Dados(c)
	}
	c.mostrar(w, pg.Modelo, d)
}

func (c *Central) tarefa(w http.ResponseWriter, r *http.Request, resto string) {
	id, acao, _ := strings.Cut(resto, "/")
	t := c.tarefas.Obter(id)
	if t == nil {
		http.NotFound(w, r)
		return
	}
	pg := paginas[t.Pagina]
	switch {
	case acao == "" && r.Method == http.MethodGet:
		d := c.base(pg.Titulo)
		d.Pagina, d.NomePagina, d.Tarefa = pg, t.Pagina, t.ID
		c.mostrar(w, pg.Modelo, d)
	case acao == "estado" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(t.Estado())
	case acao != "" && r.Method == http.MethodPost:
		f := pg.Acoes[acao]
		e := t.Estado()
		if f == nil || !e.Fim || !e.OK {
			http.NotFound(w, r)
			return
		}
		if err := f(c, e.Resultado); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (c *Central) estatico(w http.ResponseWriter, r *http.Request, nome string) {
	sub, _ := fs.Sub(arquivosWeb, "web")
	if strings.HasSuffix(nome, ".html") {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, sub, nome)
}

func (c *Central) icone(w http.ResponseWriter, r *http.Request, id string) {
	it, ok := c.item(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	c.arquivo(w, r, acharIcone(it.Icone))
}

func (c *Central) arquivo(w http.ResponseWriter, r *http.Request, caminho string) {
	if caminho == "" || !(strings.HasSuffix(caminho, ".png") || strings.HasSuffix(caminho, ".svg")) {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "private, max-age=600")
	if strings.HasSuffix(caminho, ".svg") {
		h.Set("Content-Type", "image/svg+xml")
		// SVG de tema sem script: a CSP abaixo vale só para este arquivo.
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}
	http.ServeFile(w, r, caminho)
}
