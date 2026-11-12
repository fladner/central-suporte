// Central do Suporte SEDUC: sinal de vida (fechar a janela encerra a Central),
// abrir itens ainda em yad e acompanhar tarefas.
"use strict";

function postar(url) {
  return fetch(url, { method: "POST", credentials: "same-origin" });
}

setInterval(function () { postar("/sinal").catch(function () {}); }, 20000);

document.addEventListener("DOMContentLoaded", function () {
  var nota = document.getElementById("nota");

  document.querySelectorAll("[data-abrir]").forEach(function (b) {
    b.addEventListener("click", function () {
      var nome = b.querySelector(".nome").textContent;
      b.disabled = true;
      postar("/abrir/" + encodeURIComponent(b.dataset.abrir)).then(function (r) {
        if (nota) nota.textContent = r.ok ? "Abrindo " + nome + "..." : "Não foi possível abrir " + nome + ".";
      }).catch(function () {
        if (nota) nota.textContent = "Não foi possível abrir " + nome + ".";
      }).finally(function () {
        setTimeout(function () { b.disabled = false; }, 2500);
      });
    });
  });

  var caixa = document.querySelector("[data-tarefa]");
  if (!caixa) return;
  var id = caixa.dataset.tarefa;
  var msg = caixa.querySelector(".mensagem");
  var barra = caixa.querySelector(".barra");

  caixa.querySelectorAll("[data-acao]").forEach(function (b) {
    b.addEventListener("click", function () {
      b.disabled = true;
      postar("/tarefa/" + id + "/" + b.dataset.acao).finally(function () {
        setTimeout(function () { b.disabled = false; }, 2000);
      });
    });
  });

  function atualizar() {
    fetch("/tarefa/" + id + "/estado", { credentials: "same-origin" }).then(function (r) {
      return r.json();
    }).then(function (e) {
      msg.textContent = e.mensagem || "";
      barra.setAttribute("aria-valuenow", e.percentual);
      barra.firstElementChild.style.width = e.percentual + "%";
      if (!e.fim) { setTimeout(atualizar, 500); return; }
      barra.hidden = true;
      caixa.querySelectorAll("[data-aviso]").forEach(function (a) {
        a.textContent = e.aviso || "";
        a.hidden = !e.aviso;
      });
      caixa.querySelectorAll(".relatorio").forEach(function (tab) {
        var corpo = tab.querySelector("tbody");
        corpo.textContent = "";
        (e.relatorio || []).forEach(function (l) {
          var tr = document.createElement("tr");
          l.forEach(function (v) {
            var td = document.createElement("td");
            td.textContent = v;
            tr.appendChild(td);
          });
          if (/FALHA/.test(l[1])) tr.className = "falha";
          corpo.appendChild(tr);
        });
        tab.hidden = !(e.relatorio && e.relatorio.length);
      });
      if (e.ok) {
        msg.hidden = true;
        caixa.querySelectorAll("[data-campo]").forEach(function (c) {
          c.textContent = (e.resultado || {})[c.dataset.campo] || "";
        });
        caixa.querySelector(".concluido").hidden = false;
      } else {
        caixa.querySelector(".registro").textContent = e.registro || "";
        caixa.querySelector(".falhou").hidden = false;
      }
    }).catch(function () { setTimeout(atualizar, 1500); });
  }
  atualizar();
});
