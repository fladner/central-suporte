#!/usr/bin/python3
# Janela nativa da Central do Suporte SEDUC (WebKitGTK), no lugar da janela de aplicativo
# do Chrome, como o BigBashView do BigLinux faz. Mostra só as páginas da Central e dos
# programas locais que abrem na mesma janela (http://127.0.0.1:<porta>); os demais
# endereços (e-mail, sites) abrem no programa padrão do sistema.
#
# O endereço de entrada (de uso único) chega pela entrada padrão, não pela linha de
# comando, que outros usuários do computador podem ver no "ps".
# Uso: echo URL | janela.py [--largura L --altura A --x X --y Y]
# Saída 3: falta o WebKitGTK ou o GTK (a Central usa o Chrome no lugar).

import argparse
import sys
from urllib.parse import urlsplit

SEM_BIBLIOTECA = 3

try:
    import gi

    gi.require_version("Gtk", "3.0")
    gi.require_version("Gdk", "3.0")
    gi.require_version("WebKit2", "4.1")
    from gi.repository import Gdk, Gio, GLib, Gtk, WebKit2
except (ImportError, ValueError) as erro:
    print("central-janela: sem WebKitGTK:", erro, file=sys.stderr)
    sys.exit(SEM_BIBLIOTECA)

TITULO = "Central do Suporte SEDUC"
ICONE = "seduc-logo"


def endereco_local(endereco):
    """Página da Central ou de um programa local: http://127.0.0.1:<porta>/..."""
    try:
        u = urlsplit(endereco)
        return u.scheme == "http" and u.hostname == "127.0.0.1" and u.port is not None and not u.username
    except ValueError:
        return False


def abrir_fora(endereco):
    """E-mail (mailto:) e sites abrem no programa padrão, fora da Central."""
    if endereco.startswith(("http://", "https://", "mailto:")):
        try:
            Gio.AppInfo.launch_default_for_uri(endereco, None)
        except GLib.Error as erro:
            print("central-janela: não abriu", endereco, erro, file=sys.stderr)


def decidir(_vista, decisao, tipo):
    if tipo == WebKit2.PolicyDecisionType.RESPONSE:
        return False
    endereco = decisao.get_navigation_action().get_request().get_uri()
    if tipo == WebKit2.PolicyDecisionType.NAVIGATION_ACTION and endereco_local(endereco):
        return False
    # Link para fora, ou pedido de nova janela (target=_blank): programa padrão.
    decisao.ignore()
    abrir_fora(endereco)
    return True


def geometria(args):
    """Tamanho pedido pela Central; sem ele, 1000x720 no centro do monitor principal."""
    if args.largura and args.altura:
        return args.largura, args.altura, args.x, args.y
    largura, altura = 1000, 720
    tela = Gdk.Display.get_default()
    monitor = tela.get_primary_monitor() or tela.get_monitor(0) if tela else None
    if monitor is None:
        return largura, altura, None, None
    area = monitor.get_workarea()
    largura, altura = min(largura, area.width - 40), min(altura, area.height - 40)
    return largura, altura, area.x + (area.width - largura) // 2, area.y + (area.height - altura) // 2


def main():
    p = argparse.ArgumentParser(description=TITULO)
    for nome in ("largura", "altura", "x", "y"):
        p.add_argument("--" + nome, type=int)
    args = p.parse_args()

    entrada = sys.stdin.readline().strip()
    if not endereco_local(entrada):
        print("central-janela: endereço inválido na entrada", file=sys.stderr)
        return 2

    # Classe da janela (WM_CLASS) igual à da janela do Chrome: o painel agrupa as duas.
    GLib.set_prgname("central-suporte-seduc")
    Gdk.set_program_class("Central-suporte-seduc")
    GLib.set_application_name(TITULO)

    # Sem nada gravado em disco: cookie da sessão e cache só na memória.
    dados = WebKit2.WebsiteDataManager.new_ephemeral()
    contexto = WebKit2.WebContext.new_with_website_data_manager(dados)
    contexto.set_cache_model(WebKit2.CacheModel.DOCUMENT_VIEWER)
    vista = WebKit2.WebView.new_with_context(contexto)
    cfg = vista.get_settings()
    cfg.set_enable_developer_extras(False)
    cfg.set_enable_write_console_messages_to_stdout(False)
    cfg.set_javascript_can_open_windows_automatically(False)
    cfg.set_enable_back_forward_navigation_gestures(False)
    vista.connect("decide-policy", decidir)
    # Sem menu de contexto (recarregar, inspecionar, abrir em outra janela).
    vista.connect("context-menu", lambda *_: True)

    janela = Gtk.Window(title=TITULO)
    janela.set_icon_name(ICONE)
    largura, altura, x, y = geometria(args)
    janela.set_default_size(largura, altura)
    if x is not None and y is not None:
        janela.move(x, y)
    else:
        janela.set_position(Gtk.WindowPosition.CENTER)
    vista.connect("notify::title", lambda v, _: janela.set_title(v.get_title() or TITULO))
    janela.add(vista)
    janela.connect("destroy", Gtk.main_quit)

    # Ctrl+R recarrega a página (como no Chrome); F5 também.
    def tecla(_j, ev):
        ctrl = ev.state & Gdk.ModifierType.CONTROL_MASK
        if ev.keyval == Gdk.KEY_F5 or (ctrl and ev.keyval in (Gdk.KEY_r, Gdk.KEY_R)):
            vista.reload()
            return True
        return False

    janela.connect("key-press-event", tecla)

    vista.load_uri(entrada)
    janela.show_all()
    Gtk.main()
    return 0


if __name__ == "__main__":
    sys.exit(main())
