# Speed News — visão geral do projeto

> Retrato do projeto em 25/11/2026. Para a ordem em que as coisas acontecem, veja [fluxos.md](fluxos.md). Explica o que existe, como as peças se encaixam, as regras da casa e por que cada decisão foi tomada.

---

## 1. O que é

Um **widget de bandeja** (não uma janela comum) que reúne notícias de Fórmula 1 e mostra a programação do GP da semana. Ele roda **inteiro no computador de quem usa**: não tem login, servidor próprio nem telemetria.

**Funcionalidades:**

- **Notícias** de 8 fontes em português e inglês, com contador de não lidas no ícone da barra, opção de marcar como lida ou não lida, Anterior/Próxima e link para a matéria original.
- **Card do GP** com quatro abas:
  - Programação: sessões no fuso do usuário, agrupadas por dia, com contagem regressiva;
  - Último GP: pódio, pole e volta mais rápida;
  - Pista: ficha do circuito e histórico de vencedores;
  - Clima: previsão por sessão.
- **Configurações**: idioma do app, idiomas das notícias, fuso horário (agrupado por país), "fechar ao clicar fora" e a seção Sobre.
- **Atualização automática**, assinada, a partir do GitHub Releases.

---

## 2. Arquitetura

```
┌──────────────────────── Computador do usuário ─────────────────────────┐
│                                                                        │
│  Barra do GNOME            App Tauri (Rust) ──────────────────────┐    │
│  ┌──────────────┐  D-Bus   │ tray/ícone · janela popover          │    │
│  │ extensão JS  │◄────────►│ ponte D-Bus · atualizador            │    │
│  │ ▚▞ 12        │          │ inicia/encerra o sidecar ────────┐   │    │
│  └──────────────┘          │                                  │   │    │
│                            │  WebView: React + TypeScript     │   │    │
│                            │  (interface)                     │   │    │
│                            └──────────┬───────────────────────┼───┘    │
│                    HTTP + SSE em      │                       │        │
│                    127.0.0.1:<porta   ▼                       ▼        │
│                    aleatória>   ┌───────────────────────────────┐      │
│                    + token      │ Sidecar Go (news-api)         │      │
│                                 │ agendador · serviços · cache  │      │
│                                 │ SQLite (dados locais)         │      │
│                                 └──────────────┬────────────────┘      │
└────────────────────────────────────────────────┼───────────────────────┘
                                                 │ HTTPS (direto, sem servidor nosso)
           ┌──────────────┬──────────────────────┼──────────────┬─────────────┐
           ▼              ▼                      ▼              ▼             ▼
      Feeds RSS     Jolpica F1           f1-circuits      Open-Meteo    páginas do
      (8 fontes)    (calendário,         (traçados)       (clima)       Formula1.com
                    resultados)                                         (og:image)
```

**Divisão de responsabilidades:**

| Camada | Faz | Não faz |
|---|---|---|
| **Go** (`apps/api`) | Toda a regra de negócio: busca, normalização, armazenamento, cache, agendamento e a API HTTP | Nada de interface |
| **React** (`apps/desktop/src`) | Telas, formatação no fuso e idioma do usuário, animações | Não acessa a internet diretamente; tudo passa pelo Go |
| **Rust** (`apps/desktop/src-tauri`) | Só a "cola" nativa: tray, janela, sidecar, D-Bus, atualizador | Nenhuma regra de negócio |
| **Extensão GNOME** | Botão na barra: clique simples abre, contador em texto | Nada além do botão |

**Como as peças conversam:**

1. O Tauri inicia o sidecar Go numa **porta aleatória**, passando um **token aleatório** por variável de ambiente.
2. O Go imprime `SPEED_NEWS_API_READY port=NNNN`; o Rust lê essa linha e entrega porta e token à interface.
3. O Go também imprime `SPEED_NEWS_TRAY {json}` sempre que o contador ou uma configuração da bandeja muda; o Rust atualiza o ícone, o menu e o serviço D-Bus lido pela extensão do GNOME. A bandeja **não depende da interface**.
4. A interface (só existe com o popover aberto, mais 60 s) chama a API HTTP com `Authorization: Bearer <token>` e recebe o contador por **SSE** (`/events`).
5. Ao fechar o app, o Rust encerra o sidecar. Se o app cair, o Go percebe o stdin fechado e encerra sozinho.

---

## 3. Linguagens, frameworks e bibliotecas

| Parte | Linguagem | Principais tecnologias (versão) |
|---|---|---|
| Backend | **Go 1.27** (módulo declara 1.26.1) | biblioteca padrão (`net/http`, `log/slog`), `gofeed` 1.5 (RSS), `modernc.org/sqlite` 1.63 (SQLite em Go puro, sem CGO), `yaml.v3` |
| Interface | **TypeScript 5.8** + **React 19** | Vite 6 (build/dev server), CSS puro com variáveis (sem framework de CSS), `Intl` nativo para datas, fusos e idiomas |
| Casca nativa | **Rust 1.100** | **Tauri 2.12**, `tauri-plugin-updater` 2.14, `-shell`, `-opener`, `tray-icon` 0.25 com backend **`ksni`** 0.3, `zbus` 5.19 (D-Bus) |
| Extensão GNOME | **JavaScript (GJS, módulos ES)** | GNOME Shell 45–50 (`St`, `Clutter`, `Gio`, `PanelMenu`) |
| Ferramentas | Go / Node | `tools/icon-generator` (gera os ícones), scripts Node (sidecar, mapeamento fuso→país) |

**Testes e qualidade:** Go `testing` + `-race`, **Vitest 4** + **Testing Library** + jsdom, **Playwright 1.56**, `cargo test`, **golangci-lint 2.15** (com `varnamelen`), **ESLint 9** (com `id-length`), `rustfmt`/`clippy`.

**Tamanho do código** (sem arquivos gerados nem locks): ~6.200 linhas de Go, ~4.400 de TS/TSX, ~1.500 de CSS, ~870 de Rust.

---

## 4. Estrutura do monorepo

```
speed-news/
├── apps/
│   ├── api/                         # Backend Go (sidecar)
│   │   ├── cmd/news-api/            #   main: lê flags e chama internal/app
│   │   ├── cmd/fixture-server/      #   servidor falso das fontes (para testes/E2E)
│   │   └── internal/
│   │       ├── app/                 #   monta tudo (URLs das fontes configuráveis)
│   │       ├── domain/              #   entidades: Article, RaceEvent, UserSettings…
│   │       ├── sources/             #   lista de fontes (default_sources.yaml)
│   │       ├── providers/           #   tudo o que acessa a internet
│   │       │   ├── rss/             #     feeds (+ imagem fora dos campos)
│   │       │   ├── ogimage/         #     og:image para fontes sem imagem no feed
│   │       │   ├── jolpica/         #     calendário e resultados
│   │       │   ├── f1circuits/      #     traçados GeoJSON → SVG, ficha do circuito
│   │       │   └── openmeteo/       #     previsão do tempo por sessão
│   │       ├── news/                #   serviço de notícias + normalização dos feeds
│   │       ├── raceweekend/         #   "qual é o GP da semana", modo fim de semana
│   │       ├── raceinsights/        #   abas Último GP, Pista e Clima (com cache)
│   │       ├── jobs/                #   agendador (notícias e calendário separados)
│   │       ├── notifier/            #   transmite o contador (SSE e bandeja)
│   │       ├── shellbridge/         #   estado da bandeja no stdout para o Tauri
│   │       ├── httpapi/             #   rotas por assunto, token, CORS, SSE
│   │       ├── storage/sqlite/      #   banco por agregado, migrações, manutenção
│   │       └── fixtures/            #   o "mundo falso" usado pelos testes
│   └── desktop/
│       ├── src/                     # Interface React
│       │   ├── features/            #   news, race-weekend, settings, updates
│       │   ├── shared/              #   api (cliente), i18n, time (fusos), ui, platform (ponte Tauri)
│       │   └── test/                #   cliente de API falso e helpers de teste
│       ├── src-tauri/               # Rust: lib.rs, popover.rs, placement.rs, tray.rs, tray_texts.rs, sidecar.rs, panel_bridge.rs, updates.rs
│       ├── gnome-shell-extension/   # botão da barra do GNOME
│       ├── e2e/                     # testes Playwright
│       └── scripts/                 # build do sidecar, geração fuso→país
├── tools/icon-generator/            # fonte única dos ícones: app, bandeja (claro/escuro) e botão do GNOME
├── scripts/                         # setup-linux.sh, check-go-coverage.sh, measure-memory.sh/.ps1
├── .devcontainer/  .github/workflows/  docs/
├── Makefile  go.work  package.json  rust-toolchain.toml  .mise.toml  .golangci.yml
└── CLAUDE.md  README.md  LICENSE (AGPL-3.0)
```

---

## 5. Backend Go em detalhe

### API HTTP (só em `127.0.0.1`, todas as rotas exceto `/health` exigem o token)

| Rota | Para quê |
|---|---|
| `GET /health` | verificação de vida |
| `GET /articles?status=unread\|all&limit=` | lista de notícias (filtrada pelos idiomas escolhidos) |
| `GET /articles/unread-count` | contador |
| `POST /articles/{id}/read` · `/unread` · `POST /articles/read-all` | estado de leitura |
| `POST /articles/refresh` | atualização manual (botão ↻) |
| `GET /sources` | fontes configuradas (a interface deriva os idiomas daqui) |
| `GET /race-weekend/current` | GP atual/próximo, sessões e traçado |
| `GET /race-weekend/last-race` · `/circuit` · `/forecast` | abas do card |
| `GET /settings` · `PUT /settings` | preferências |
| `GET /events` | stream SSE do contador |

### Banco (SQLite, um arquivo local)

| Tabela | Conteúdo |
|---|---|
| `articles` | notícias normalizadas, `read_at`, `image_lookup_done` |
| `race_events`, `race_sessions` | calendário da temporada |
| `circuit_layouts` | traçado já convertido em SVG (cache permanente) |
| `api_cache` | respostas das APIs externas com data de busca |
| `settings` | preferências (JSON) |

**Migrações:** tabelas criadas com `CREATE TABLE IF NOT EXISTS`; colunas novas entram pela lista `addedColumns`, que adiciona o que falta em bancos antigos. Isso permite atualizar o app sem perder dados.

### Ritmos e prazos

| O quê | Valor |
|---|---|
| Busca de notícias | a cada 30 min, **5 min na semana de corrida** (até 4 dias antes da 1ª sessão e 6 h depois da corrida) |
| Sincronização do calendário | ao iniciar e a cada 12 h (temporada atual e a seguinte) |
| Notícia vista pela 1ª vez com mais de 48 h | já entra como lida (evita 200 não lidas na 1ª execução) |
| Retenção de notícias | 30 dias |
| Cache: Último GP / Pista / Clima | 3 h / 24 h / 1 h; previsão só a partir de 7 dias antes da corrida |
| Busca de `og:image` | só fontes marcadas; no máximo 12 páginas por atualização; cada página uma única vez |
| Após cada busca de notícias | checkpoint do WAL, fecha conexões ociosas, devolve memória ao sistema |
| Manutenção diária do banco | apaga `api_cache` com mais de 30 dias, `incremental_vacuum`, `PRAGMA optimize` |
| Webview do popover | destruída 60 s depois de fechada; recriada no próximo clique |

### Resiliência (decisões que já evitaram bugs reais)

- **Notícias e calendário rodam em ciclos independentes.** Antes, a Jolpica fora do ar segurava as notícias por até 60 s.
- **Fonte fora do ar → dado antigo do cache**, nunca tela de erro.
- **Resultado parcial é exibido, mas não fica no cache.**
- **Buscas iguais e simultâneas viram uma só** (trava por chave de cache, que se apaga quando ninguém mais espera).
- **Uma fonte quebrada não derruba as outras** (busca em paralelo, erro isolado).

---

## 6. Casca nativa e interface

### Rust (`src-tauri/src`)

| Arquivo | Responsabilidade |
|---|---|
| `lib.rs` | registra plugins, comandos e eventos da janela; impede que destruir a janela encerre o app |
| `sidecar.rs` | inicia o Go com porta e token aleatórios, entrega a conexão à interface e lê o estado da bandeja do stdout |
| `popover.rs` | cria a janela sob demanda, mostra/esconde, destrói após 60 s escondida, proteção "fecha e reabre" |
| `placement.rs` | onde o popover abre: monitor da âncora, área útil, acima/abaixo, escala por monitor (funções puras) |
| `tray.rs` | ícone (claro/escuro, ponto vermelho), tooltip e menu a partir do estado vindo do Go |
| `tray_texts.rs` | textos da bandeja em PT/EN (espelho de `messages.ts`, com teste de paridade) e idioma do sistema |
| `panel_bridge.rs` | serviço D-Bus `dev.speednews.Desktop1` para a extensão do GNOME (Linux) |
| `updates.rs` | atualização automática (silenciosa ou "pergunta antes", conforme o formato) |

### Interface (`apps/desktop/src`)

- **Telas:** lista de notícias, detalhe da notícia, configurações (com o seletor de fuso) e o card do GP com as abas.
- **Padrões:**
  - todo acesso ao Tauri passa por `shared/platform/tauriBridge.ts`, por isso a interface também roda num navegador comum;
  - datas chegam sempre em UTC e são formatadas só na exibição (`shared/time`);
  - textos em PT e EN ficam em `shared/i18n/messages.ts`.
- **Experiência:**
  - skeletons de carregamento, que só aparecem se a espera passar de 150 ms;
  - transições suaves, incluindo a altura animada do card;
  - respeito ao "reduzir movimento" do sistema e temas claro/escuro automáticos.

### Por que existe a extensão do GNOME

No Ubuntu, a extensão AppIndicator abre o **menu** no clique simples de qualquer ícone de bandeja, e só o duplo clique chega ao app. Também não mostra texto ao lado do ícone com o backend `ksni`. O botão próprio resolve as duas coisas: um clique abre, e o contador aparece em texto. Ao abrir, a extensão também **dá foco** à janela, porque o GNOME não dá foco a janelas abertas via D-Bus, e sem foco o "fechar ao clicar fora" não funcionaria.

---

## 7. Fontes de dados externas

| Fonte | Uso | Licença / regra |
|---|---|---|
| 8 feeds RSS (Formula1.com, Motorsport, Autosport, The Race, Motorsport Brasil, F1Mania, Autoracing, GPTotal) | notícias | só título, resumo, imagem e link **que o próprio feed publica**; a matéria abre no site original |
| Páginas do Formula1.com | imagem de compartilhamento (`og:image`) | só o `<head>`; só páginas do próprio site da fonte |
| Jolpica F1 API | calendário, sessões, resultados | API aberta e comunitária (sucessora da Ergast) |
| bacinger/f1-circuits | traçados e ficha dos circuitos | MIT, com atribuição exibida |
| Open-Meteo | previsão do tempo | CC BY 4.0, com atribuição exibida |
| tz database | mapeamento fuso → país | domínio público (gerado em `timeZoneCountries.generated.ts`) |

Adicionar uma fonte de notícias é **só uma entrada em `default_sources.yaml`**, sem código novo. Quem tem feed sem imagem pode receber `imageFromArticlePage: true`.

---

## 8. Regras da casa (de `CLAUDE.md`)

1. **Nomes sempre semânticos.** Nada de `a`, `x`, `i`, `e`, `t`, `w`, `r`, em qualquer linguagem; só `err`, `ctx` e `id` são aceitos. O ESLint (`id-length`) e o golangci-lint (`varnamelen`) fiscalizam isso na CI.
2. **Horários em UTC** no backend e na API; a conversão para o fuso do usuário acontece só na exibição.
3. **Notícias: só o que o feed expõe.** Nunca copiar a matéria inteira.
4. **Nova fonte = nova entrada no YAML**, e também nas fixtures dos testes.
5. **Escopo atual: só F1**, mas as entidades guardam `SeriesSlug` para o WEC entrar depois.
6. **Testes nunca acessam fontes reais**: usam o servidor de fixtures.
7. **Os pisos de cobertura só sobem** (Go 80%, frontend em `vite.config.ts`).
8. **A interface D-Bus `dev.speednews.Desktop1` não pode quebrar compatibilidade**, porque a extensão do GNOME é atualizada separadamente do app.
9. **Os dados precisam sobreviver às atualizações**: mudanças no banco passam por `addedColumns`, e campos novos de configuração precisam de valor padrão.
10. **A versão do app mora só em `apps/desktop/package.json`.**
11. **A chave de assinatura das atualizações nunca vai para o repositório.**

---

## 9. Segurança e privacidade

- **Nenhum dado sai para nós:** sem conta, sem telemetria. Banco em `~/.local/share/dev.speednews.desktop/` (Linux) ou o equivalente em cada sistema.
- **API local protegida:**
  - escuta só em `127.0.0.1`, numa porta aleatória;
  - exige um token aleatório gerado a cada execução;
  - aceita CORS apenas das origens da própria interface.
- **CSP da webview:** só conecta ao `127.0.0.1` e carrega imagens via `https`.
- **`og:image`:** o app recusa (sem nem acessar) qualquer endereço fora do domínio da fonte, inclusive a rede local.
- **Atualizações assinadas:** o app só instala pacotes assinados com a chave do projeto.
- **User-Agent genérico** (`SpeedNews/0.1`), sem identificar o usuário.
- **Commits** usam o e-mail `noreply` do GitHub.

---

## 10. Qualidade: testes e CI

| Camada | Quantidade | O que cobre |
|---|---|---|
| Go | 82 funções de teste (+ subtestes), **cobertura 82%** | serviços, clientes HTTP, banco/migrações/manutenção, contrato de stdout da bandeja, ícones gerados, o **backend inteiro** contra as fixtures e um teste de estabilidade (dezenas de ciclos sem crescer) |
| Frontend | **117 testes**, **cobertura 85%** das linhas | formatação, fusos, telas (lista, notícia, configurações, card, avisos de atualização), paridade dos textos da bandeja com o Rust |
| Rust | 42 | posição da janela, "fecha e reabre", ciclo de vida da janela (destruição cancelada ao reabrir, sair só explicitamente), leitura do stdout do Go, textos/idioma da bandeja, variantes do ícone, regra de atualização |
| E2E (Playwright) | 9, em **Chromium e WebKit** | app real no navegador: contador, leitura, abas, idiomas, notícia nova, fonte fora do ar, regressões visuais |

**O servidor de fixtures** (`internal/fixtures`) imita todas as fontes externas:
- tem um "GP de Singapura" **sempre 2 dias à frente**, então a suíte não vence quando a temporada acaba;
- tem rotas de controle para simular notícia nova e queda de fonte.

**CI** (`.github/workflows/ci.yml`), a cada push:
- lint do Go e cobertura mínima;
- lint, tipos e cobertura do frontend;
- E2E em dois motores;
- `rustfmt`, `clippy` com avisos tratados como erro, e testes do Rust **em Linux, Windows e macOS** (o código da bandeja tem partes por plataforma).

**Release** (`release.yml`): roda ao criar uma tag `v*`.

---

## 11. Ambiente de desenvolvimento

**Versões fixadas** (mude todas juntas):

| Ferramenta | Versão | Onde |
|---|---|---|
| Go | 1.27.4 | `.mise.toml`, Dockerfile, CI |
| Node | 22.23.5 | `.mise.toml`, Dockerfile, CI |
| Rust | 1.100.0 | `rust-toolchain.toml` |
| golangci-lint | 2.15.0 | `.mise.toml`, Dockerfile, CI |

**Comandos do dia a dia:**

| Comando | Faz |
|---|---|
| `make dev` | app desktop completo (Tauri + sidecar) |
| `make backend-dev` + `make web-dev` | interface no navegador, sem Rust |
| `make test` | testes rápidos (Go, frontend, Rust) |
| `make test-e2e` | E2E no navegador |
| `make check` | tudo o que a CI roda |
| `make measure-memory` | memória do app em execução por grupo (casca, webview, sidecar) |
| `make icons` | regera todos os ícones a partir de `tools/icon-generator` |
| `make install-gnome-extension` | instala o botão da barra (depois: Alt+F2 → `r`) |
| `make setup-linux` | prepara uma máquina Debian/Ubuntu do zero |
| `make docker-check` | lint e testes dentro do contêiner, sem instalar nada |

**Opções de ambiente:**
- **Dev Container** (VS Code → "Reopen in Container") para Go e React;
- **mise** para rodar o app nativo com as versões fixadas.

---

## 12. Distribuição e atualizações

- **Código público:** github.com/Breno-Andrade/speed-news, licença **AGPL-3.0-or-later**, **© 2026 Breno Andrade**. Quem distribuir uma versão (modificada ou não) precisa manter o aviso de copyright, a licença AGPL e o código aberto; quem modificar e oferecer o software pela rede (por exemplo, hospedar o backend como serviço) também precisa liberar o código aos usuários. Por isso o copyleft foi mantido em vez de MIT: com MIT, alguém poderia transformar o app num produto fechado.
- **Instaladores**, de graça no GitHub Releases:
  - **AppImage** (recomendado no Linux), `.deb` e `.rpm`;
  - `.exe` e `.msi` no Windows;
  - `.dmg` no macOS.
- **Nenhum servidor é necessário**, nem agora nem com milhares de usuários: cada instalação busca as fontes diretamente.

**Fluxo de release:**
1. Subir a versão em `apps/desktop/package.json`.
2. `git tag vX.Y.Z && git push --tags`.
3. A CI compila, **assina** e cria um rascunho de release com o `latest.json`.
4. Publicar o rascunho.

**Atualização automática** (verifica 1 min após abrir e a cada 6 h):

| Instalado como | Comportamento |
|---|---|
| AppImage, Windows, macOS | baixa, espera a janela estar fechada, instala e reinicia; mostra "Atualizado para a versão X" |
| `.deb` / `.rpm` | baixa e oferece "Atualizar agora" (o sistema pede a senha de administrador) |
| `make dev` | não verifica |

⚠️ **Chave de assinatura:** fica nos *secrets* do repositório e na sua pasta `~/.tauri/`. **Se for perdida, os apps instalados nunca mais conseguem se atualizar.** Mantenha uma cópia num cofre de senhas.

---

## 13. Decisões tomadas e por quê

| Decisão | Motivo |
|---|---|
| **Tauri** em vez de Electron | ~30–60 MB de RAM contra 150–250 MB; importante para um widget sempre ligado |
| **Go como sidecar** | toda a lógica numa linguagem com testes simples; o Rust fica só como cola |
| Pasta **`apps/api`** (como no etapy) | o Go é empacotado dentro do app, não implantado à parte; padrão igual ao seu outro projeto |
| **SQLite em Go puro** | sem CGO: compila para qualquer sistema sem toolchain C |
| Backend de tray **`ksni`** | o padrão do Tauri (AppIndicator) nunca entrega o clique esquerdo ao app |
| **Extensão própria do GNOME** | o AppIndicator do Ubuntu abre o menu no clique simples e não tem opção para mudar |
| **Bolinha vermelha** no ícone (Linux/Windows) | o `ksni` não exibe texto; o número fica no botão do GNOME, no cabeçalho e no tooltip |
| **AppImage como download principal no Linux** | é o único formato do Linux que se atualiza sem senha |
| **Repositório público + AGPL-3.0** | CI ilimitada e assinatura gratuita no Windows (SignPath); derivados continuam livres, inclusive se alguém hospedar o backend como serviço |
| **Fixtures com datas relativas** | testes determinísticos que não vencem com a temporada |
| **Fusos agrupados por país** | as pessoas conhecem o país, não o identificador IANA nem o GMT |

---

## 14. Limitações conhecidas e próximos passos

**Ainda não verificado:**
- O app **nunca rodou no Windows nem no macOS**. A CI compila, mas falta um teste manual (tray, posição da janela, sidecar).
- **Uma atualização real de versão** só pode ser testada com duas releases publicadas (v0.1.0 e depois v0.1.1).

**Limitações atuais:**
- A **GPTotal** não publica imagem confiável, então suas notícias usam a capa colorida.
- O macOS **não é notarizado** (custa US$ 99/ano): na primeira abertura é preciso liberar o app nas configurações do sistema.
- No Wayland, a extensão do GNOME exige **sair e entrar na sessão** para carregar; no X11, basta Alt+F2 → `r`.
- A Jolpica às vezes fica lenta ou fora do ar; o cache cobre esses momentos.

**Pendências para a v0.1.0:**
- **2 commits locais ainda não enviados** ao GitHub (configurações e fusos por país).
- Primeira execução no GNOME: detectar a falta da extensão e oferecer a instalação.
- Teste manual no Windows e no macOS.
- Opcional: doações (PIX com chave aleatória, GitHub Sponsors ou Ko-fi), com um link discreto em Configurações → Sobre.

---

## 15. Glossário rápido

| Termo | Significado aqui |
|---|---|
| **Tray / bandeja** | área de ícones no canto da barra do sistema |
| **Popover** | a janela pequena que abre sob o ícone e fecha ao clicar fora |
| **Sidecar** | programa auxiliar (o Go) que o app inicia e encerra junto com ele |
| **SSE** | Server-Sent Events: o backend "empurra" o contador para a interface em tempo real |
| **D-Bus** | canal de comunicação entre programas no Linux (app ↔ extensão do GNOME) |
| **IANA / tz database** | base oficial de fusos (`America/Sao_Paulo`) |
| **Fixture** | dado ou servidor falso e controlado, usado nos testes |
| **E2E** | teste de ponta a ponta: o app inteiro rodando num navegador real |
| **og:image** | imagem que um site publica para aparecer em prévias de link |
| **AppImage** | formato de app Linux num arquivo único, que se atualiza sozinho |
