# Discord Uploader — Native (Go + Fyne)

Reescrita 100% nativa em Go do [Discord Uploader](
../discord-uploader.sh). Não depende de Bash, `curl`, `find`, `stat` nem de
nenhum processo externo. Tudo é feito em HTTP puro via `net/http` +
`mime/multipart`.

## Estrutura

```
native/
  go.mod             módulo Go + fyne.io/fyne/v2 v2.4.4
  main.go            inicialização da UI e tema escuro fixo
  ui.go              layout da janela (adaptado de gui/ui.go)
  uploader.go        miolo: HTTP multipart, retry, 429, 413, 5xx, incerto
  FyneApp.toml       metadados do executável
  build-linux.sh     build para Linux
  build-win.sh       cross-compile para Windows (MinGW)
  icon.png           ícone 256x256 (copiado de gui/icon.png)
  .gitignore         ignora binários gerados
  README.md          este arquivo
```

## Pré-requisitos

- Go 1.21+
- Para cross-compile Windows no Linux: `sudo apt install gcc-mingw-w64-x86-64`

## Como buildar (Linux)

```bash
cd native
./build-linux.sh
# gera: native/DiscordUploader
```

## Como cross-compilar para Windows

```bash
cd native
./build-win.sh
# gera: native/DiscordUploader.exe
```

O executável Windows usa `-H windowsgui` para não abrir o console preto do CMD.

## Lógica portada do Bash

A função `UploadFile` em `uploader.go` replica fielmente o comportamento do
script Bash:

| Caso | Comportamento |
|------|---------------|
| **204 No Content** | Sucesso (webhook sem retorno). |
| **200 com `"id"` no JSON** | Sucesso (confirmação de upload). |
| **429 Rate Limit** | Lê `Retry-After` (header) ou `retry_after` (JSON). Se > 60s, aborta permanente. Senão, dorme e tenta novamente (máx. 4 tentativas). |
| **413 Payload Too Large** | Falha permanente. |
| **5xx** | Estado **incerto** — o servidor pode ter processado. Não reenviar cegamente. |
| **200 sem `"id"`** | Estado incerto. |
| **Falha de rede** (DNS/TLS/timeout) | Retry com backoff exponencial (2s, 4s, 8s… até 30s). Se persistir, estado incerto. |
| **Cancelamento** | `context.Context` aborta imediatamente (botão Cancelar / SIGTERM). |

## Segurança

- O webhook viaja **na URL da requisição HTTP**, nunca em arquivo temporário nem em argv visível via `ps`.
- Não há criação de arquivos temporários para credenciais.

## Tema

- Tema **escuro fixo** (`theme.DarkTheme()`), por escolha do usuário. Não respeita o tema do sistema.

## Próximos passos (usuário)

1. Instalar Go: `sudo apt install golang`
2. Para build Windows: `sudo apt install gcc-mingw-w64-x86-64`
3. Substituir `native/icon.png` por um ícone real 256x256 (placeholder atual)
4. Testar com um webhook descartável (não expor tokens reais)
5. Decidir se assina o executável com GPG/Authenticode

## Limitações

- O log de progresso conta apenas o que o próprio app imprime (não há TSV externo).
- "Salvar relatório…" ainda é placeholder (funcionalidade a ser preenchida).
- O upload usa `http.DefaultClient`; não há configuração de proxy personalizada.
