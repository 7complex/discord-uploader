# Como Contribuir

Obrigado por se interessar em contribuir com o **Discord Uploader**! Este documento
descreve como reportar bugs, propor melhorias e preparar pull requests.

---

## 1. Reportando Bugs

Antes de abrir uma issue, verifique se o bug já foi reportado na aba
[Issues](https://github.com/7complex/discord-uploader/issues).

Ao reportar, inclua:

- Sistema operacional e versão (ex.: Ubuntu 24.04, Windows 11).
- Qual frente está afetada: CLI Bash (`discord-uploader.sh`), GUI wrapper
  (`gui/`, AppImage) ou GUI nativa (`native/`, .exe/binário).
- Passos para reproduzir o problema.
- Comportamento esperado vs. comportamento observado.
- Se possível, cole o log de execução (sem expor seu webhook do Discord — veja
  a seção de segurança abaixo).

---

## 2. Segurança

**Nunca** cole seu webhook real em issues, pull requests ou comentários. O
webhook é um segredo que permite enviar mensagens para seu canal. Se precisar
compartilhar logs, substitua a URL por `[REDACTED]`.

Vale também ler o [SECURITY.md](SECURITY.md).

---

## 3. Padrões de Código

### Bash (`discord-uploader.sh`)

- O script deve passar no **ShellCheck** no nível `warning`:
  ```bash
  shellcheck -S warning discord-uploader.sh
  ```
- Sintaxe válida para **Bash 4.4+**.
- Não adicione `set -e`; o script depende de retornos não-zero para retry.
- O webhook nunca deve aparecer em argv visível via `ps` (usa `--config -`).
- Mantenha os comentários em português (idioma do script).
- Não reescreva o "miolo": `upload_file`, `sanitizar`, `escapar_saida`,
  `escape_form_filename`, `escapar_url_curl_config`, `extrair_retry_after`,
  `formatar_bytes`, `validar_webhook`, `validar_config_inteiro`.

### Go (`gui/` e `native/`)

- Formato padrão do Go:
  ```bash
  gofmt -w .
  go vet ./...
  ```
- Versão alvo: **Go 1.21+**.
- Dependência da GUI: `fyne.io/fyne/v2 v2.4.4`.
- Não hardcode caminhos absolutos.
- Tema escuro fixo nas GUIs (não tente detectar o tema do sistema).
- O webhook nunca deve ser gravado em arquivo temporário (GUI nativa) nem
  passado via argv (usa `-W arquivo` com `0600` na GUI wrapper).

---

## 4. Compilando as GUIs

### GUI wrapper (Fyne — AppImage)

```bash
cd gui
./build.sh
# gera: gui/dist/DiscordUploader-x86_64.AppImage
```

Requer Go 1.21+, `fyne` CLI, `appimagetool` e (opcional) ImageMagick para o
ícone placeholder.

### GUI nativa (Go — Linux e Windows)

```bash
cd native
./build-linux.sh   # Linux
./build-win.sh     # Windows (requer MinGW: gcc-mingw-w64-x86-64)
```

---

## 5. Enviando um Pull Request

1. Faça um fork do repositório.
2. Crie uma branch descritiva: `git checkout -b fix/minha-correcao`.
3. Faça suas alterações, respeitando os padrões acima.
4. Verifique se o ShellCheck e o `go vet` passam.
5. Abra o pull request describindo o que mudou e por quê.
6. Os PRs devem incluir a linha de atribuição:
   ```
   🤖 Generated with [Claude Code](https://claude.com/claude-code)
   ```
   (quando gerados com auxílio de IA) e terminar com:
   ```
   Co-Authored-By: Claude Code <noreply@anthropic.com>
   ```

---

## 6. Código de Conduta

Seja respeitoso com outros contribuidores. Não aceite nem envie webhooks
reais em qualquer meio público.
