# Discord Uploader — GUI (Go + Fyne)

Interface gráfica (100% GUI, sem terminal) para o [Discord Uploader](
../discord-uploader.sh). A GUI **não reimplementa upload, retry, classificação de erro
ou sanitização** — tudo isso continua no script Bash, chamado via subprocesso.

## Estrutura

```
gui/
  go.mod              módulo Go + fyne.io/fyne/v2 v2.4.4
  main.go             app, tema escuro, checkDependencies, janela
  ui.go               widgets, layout, handlers
  runner.go           exec.Command, pipes stdout/stderr, parse de progresso
  deps.go             checagem de dependências do host
  FyneApp.toml        metadados do AppImage
  build.sh            gera o AppImage
  icon.png            ícone 256x256 (placeholder gerado se ausente)
  .gitignore          ignora cópia do CLI, AppImage, dist/
  README.md           este arquivo
```

## Pré-requisitos

- Go 1.21+: `sudo apt install golang`
- ImageMagick (apenas para gerar o ícone placeholder): `sudo apt install imagemagick`
- Ferramentas do host (necessárias pelo CLI Bash): `bash curl findutils coreutils gawk sed grep`

## Como buildar

```bash
cd gui
./build.sh
```

O script:
1. Copia `../discord-uploader.sh` para `gui/discord-uploader.sh` (bundled no AppImage)
2. Instala a CLI do Fyne (`fyne.io/tools/cmd/fyne`)
3. Roda `go mod tidy`
4. Gera um ícone placeholder se `icon.png` não existir
5. Empacota com `fyne package -os linux`
6. Move para `gui/dist/DiscordUploader-x86_64.AppImage`

## Como executar

```bash
chmod +x gui/dist/DiscordUploader-x86_64.AppImage
./gui/dist/DiscordUploader-x86_64.AppImage
```

### Requisito do host: FUSE2

AppImages clássicos precisam de FUSE2:

```bash
sudo apt install libfuse2   # Debian 12 / Ubuntu 22.04+
```

### Fallback sem FUSE

```bash
./gui/dist/DiscordUploader-x86_64.AppImage --appimage-extract
./squashfs-root/AppRun
```

## Notas importantes

- O AppImage **não bundleia** `bash`, `curl`, `coreutils` etc. — essas ferramentas devem
  estar instaladas no host onde o AppImage roda.
- O tema é **escuro fixo** (`theme.DarkTheme()`), por escolha do usuário. Não respeita
  o tema do sistema.
- A GUI chama o `discord-uploader.sh`. Se o CLI for atualizado, é necessário rebuildar
  o AppImage (o `build.sh` sempre copia a versão mais recente de `../discord-uploader.sh`).
- O webhook nunca viaja em argv visível via `ps`: a GUI grava em arquivo temp com `0600`
  e passa `-W ARQUIVO` ao CLI.

## Ícone

O `build.sh` gera um placeholder azul com "DU" se `icon.png` não existir. Substitua por
um ícone real 256x256 PNG antes de distribuir.

## Próximos passos (usuário)

1. Instalar Go: `sudo apt install golang`
2. Gerar/fornecer um `gui/icon.png` real (256x256)
3. Testar com um webhook descartável (não expor tokens reais)
4. Decidir se assina o AppImage com GPG (opcional)
5. Instalar `libfuse2` nos hosts-alvo (ou usar o fallback `--appimage-extract`)
