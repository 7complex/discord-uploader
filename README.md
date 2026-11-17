# Discord Uploader

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![ShellCheck](https://img.shields.io/badge/ShellCheck-pass-brightgreen)](https://github.com/7complex/discord-uploader/actions)
[![Go](https://img.shields.io/badge/Go-1.21%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Fyne](https://img.shields.io/badge/Fyne-2.4.4-009688?logo=go&logoColor=white)](https://fyne.io/)
[![Android](https://img.shields.io/badge/Android-APK-green?logo=android&logoColor=white)](https://github.com/7complex/discord-uploader/releases)

Envia arquivos para um canal do Discord via webhook. Disponível em quatro frentes:
um **CLI em Bash** (Linux/macOS), uma **GUI em Go + Fyne** (wrapper do CLI, AppImage para Linux),
uma **GUI nativa em Go** (sem dependência de Bash, .exe para Windows, binário Linux e .apk para Android).

Versão: **1.1.0**

O idioma da interface é português, mas a documentação está em inglês para alcance global.

---

## 📥 Download

Todos os binários são publicados na aba **Releases** do GitHub:

👉 [https://github.com/7complex/discord-uploader/releases](https://github.com/7complex/discord-uploader/releases)

> **Windows:** pegue o arquivo `DiscordUploader.exe` na aba Releases.<br>
> **Linux GUI:** pegue o arquivo `DiscordUploader-x86_64.AppImage` na aba Releases.<br>
> **Linux CLI:** pegue o arquivo `discord-uploader.sh` na aba Releases.<br>
> **Android:** pegue o arquivo `.apk` na aba Releases.

---

## ✨ Features

- 📎 **Envio de arquivos** para webhooks do Discord (campo `files[0]`).
- 🔁 **Retry automático** com backoff exponencial (até 4 tentativas por padrão).
- ⏱️ **Rate limit (HTTP 429)**: respeita o header `Retry-After` e o campo JSON `retry_after`.
- 🛡️ **Classificação de resposta**: diferencia sucesso, falha permanente e resultado incerto (5xx/200 sem ID).
- 🖥️ **Três interfaces**: CLI em Bash, GUI em Go/Fyne (wrapper) e GUI nativa em Go.
- 🔒 **Segurança do webhook**: a URL nunca viaja em argv visível via `ps`.
- 🌙 **Tema AMOLED (Preto Puro)** nas GUIs: fundo #000000, superfícies #121212, destaque Discord #5865F2.
- 🇧🇷 **Interface em português**.

---

## 🚀 Installation & Usage

### Windows

1. Acesse a aba [Releases](https://github.com/7complex/discord-uploader/releases).
2. Baixe o arquivo `DiscordUploader.exe`.
3. Dê um duplo-clique para executar (não precisa instalar nada).
4. Informe o webhook do Discord, escolha a pasta e clique em **Iniciar envio**.

### Linux — GUI (AppImage)

1. Acesse a aba [Releases](https://github.com/7complex/discord-uploader/releases).
2. Baixe o arquivo `DiscordUploader-x86_64.AppImage`.
3. Torná-lo executável e rodar:

```bash
chmod +x DiscordUploader-x86_64.AppImage
./DiscordUploader-x86_64.AppImage
```

> O AppImage requer **FUSE2** no host (`sudo apt install libfuse2`). Caso não tenha FUSE, use:
> `./DiscordUploader-x86_64.AppImage --appimage-extract` seguido de `./squashfs-root/AppRun`.
>
> O AppImage é **nativo (Go + Fyne)** e não requer Bash, `curl` ou `coreutils` no host — apenas o próprio executável.

### Linux — CLI (Bash)

1. Acesse a aba [Releases](https://github.com/7complex/discord-uploader/releases).
2. Baixe o arquivo `discord-uploader.sh`.
3. Torná-lo executável:

```bash
chmod +x discord-uploader.sh
```

4. Executar sem argumentos abre o modo interativo (prompts em português):

```bash
./discord-uploader.sh
```

5. Ou usar como CLI com flags:

```bash
./discord-uploader.sh --dry-run -d /caminho/da/pasta -y
./discord-uploader.sh -w https://discord.com/api/webhooks/... -d /caminho/da/pasta -y
```

> ⚠️ **Segurança:** não passe o webhook por `--webhook URL` em scripts compartilhados, pois a URL ficaria visível em `ps`. Prefira `--webhook-file`, `--webhook-stdin` ou a variável de ambiente `DISCORD_WEBHOOK`.

### Android

1. Acesse a aba [Releases](https://github.com/7complex/discord-uploader/releases).
2. Baixe o arquivo `.apk` na aba Releases.
3. No seu dispositivo Android, permita a instalação de fontes desconhecidas:
   **Ajustes > Apps > Acesso especial a apps > Instalar apps desconhecidos > Seu navegador > Permitir nesta fonte**.
4. Abra o APK baixado e siga as instruções de instalação.
5. Informe o webhook do Discord, selecione os arquivos e clique em **Iniciar Envio**.

---

## 🏗️ Building from Source

### GUI (Fyne wrapper — AppImage para Linux)

Localizada em `gui/`. Requer Go 1.21+, a CLI do Fyne e `appimagetool`.

```bash
cd gui
./build.sh
# gera: gui/dist/DiscordUploader-x86_64.AppImage
```

O `build.sh` copia o `discord-uploader.sh` da raiz para o bundle, gera um ícone placeholder caso `icon.png` não exista, resolve dependências com `go mod tidy` e empacota com `fyne package`.

### GUI nativa (Go — sem Bash)

Localizada em `native/`. Requer Go 1.21+.

**Linux:**

```bash
cd native
./build-linux.sh
# gera: native/DiscordUploader
```

**Windows (cross-compile a partir do Linux):**

Requer MinGW (`sudo apt install gcc-mingw-w64-x86-64`).

```bash
cd native
./build-win.sh
# gera: native/DiscordUploader.exe
```

O executável Windows usa o flag `-H windowsgui` para não abrir o console preto do CMD ao clicar.

**Android (APK):**

Requer Android Studio, `ANDROID_HOME` e `ANDROID_NDK_HOME` configurados.

```bash
cd native
./build-android.sh
# gera: native/DiscordUploader.apk (ou similar)
```

O `build-android.sh` instala a CLI do Fyne (se necessário), valida as variáveis de ambiente do Android e executa `fyne package -os android`.

---

## 📋 CLI Flags

Todas as flags são documentadas em `--help`. Copia abaixo para referência:

| Flag | Descrição |
|------|-----------|
| `-w, --webhook URL` | URL do webhook (visível em `ps`; prefira as alternativas abaixo) |
| `-W, --webhook-file FILE` | Lê a 1ª linha de `FILE` como webhook |
| `--webhook-stdin` | Lê a 1ª linha de stdin como webhook |
| `-d, --dir PATH` | Pasta dos arquivos (ou argumento posicional) |
| `-f, --file PATH` | Arquivo específico (repeatable) |
| `--files-from FILE` | Lista de arquivos, um por linha (`-` = stdin) |
| `-r, --recursive` | Inclui subpastas |
| `-H, --ignore-hidden` | Ignora nomes iniciados por ponto |
| `-i, --include GLOB` | Inclui apenas nomes que casarem (repeatable) |
| `-x, --exclude GLOB` | Exclui nomes que casarem (repeatable) |
| `--min-size BYTES` | Pula arquivos menores que isso |
| `-m, --max-bytes N` | Tamanho máximo por arquivo (default `DISCORD_MAX_BYTES`) |
| `-c, --content STRING` | Mensagem enviada junto do upload |
| `-t, --thread-id ID` | Envia em thread existente |
| `--thread-name NAME` | Cria thread a partir da mensagem |
| `-y, --yes` | Não pede confirmação; assume "sim" |
| `-n, --dry-run` | Lista o que faria, não envia; exit 0 |
| `-o, --report PATH` | Salva relatório ao final (pula prompt) |
| `--log FILE` | Log por linha TSV (ts, status, bytes, arquivo) |
| `-j, --jobs N` | Aceito, mas roda com 1 (paralelismo futuro) |
| `-q, --quiet` | Suprime info/success; mantém warning/failure |
| `-v, --verbose` | Mostra detalhes extras |
| `--no-color` | Desativa cores |
| `-h, --help` | Ajuda completa com códigos de saída |
| `-V, --version` | Imprime a versão |

**Precedência do webhook:** `--webhook-file` > `--webhook-stdin` > `--webhook` > env `DISCORD_WEBHOOK` > prompt (tty).

**Códigos de saída:**

| Código | Significado |
|--------|-------------|
| 0 | OK |
| 1 | Falhas / incertos / incompleto / relatório falhou |
| 2 | Erro de uso/config (inclui falta de webhook em não-tty) |
| 4 | Webhook inválido (formato ou preflight 401/403/404 com `--yes`) |
| 130 | Interrompido (SIGINT/SIGTERM/SIGHUP) |

---

## 🔒 Segurança do Webhook

O webhook do Discord é um segredo: quem possui a URL pode enviar mensagens para o canal.

- **Nunca** passe o webhook como argumento visível (`--webhook URL` aparece em `ps`).
- Prefira `--webhook-file`, `--webhook-stdin` ou a variável de ambiente `DISCORD_WEBHOOK`.
- No modo GUI, o webhook é digitado em campo de senha mascarado e **não** é gravado em disco.
- **Não comite** arquivos contendo webhooks reais no repositório.

---

## ⚠️ Limitações Conhecidas

- O AppImage requer FUSE2 no host (ou uso do fallback `--appimage-extract`).
- O AppImage e o .exe não bundleiam `bash`, `curl`, `coreutils` etc. — essas ferramentas devem estar instaladas no host (só necessário para o CLI Bash e para a GUI wrapper).
- A GUI nativa (`native/`) não depende de nenhuma ferramenta externa além do próprio executável.
- "Salvar relatório…" nas GUIs ainda é uma funcionalidade a ser preenchida.
- O tema das GUIs é escuro fixo (não respeita o tema do sistema).

---

## 🤝 Como Contribuir

Veja o [CONTRIBUTING.md](CONTRIBUTING.md) para diretrizes de como reportar bugs, padrões de código e como compilar as GUIs.

---

## 📄 Licença

Este projeto está licenciado sob a [Licença MIT](LICENSE).

© 2026 Yan (7complex).

---

> Feito com ❤️ e [Claude Code](https://claude.com/claude-code).
