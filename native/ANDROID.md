# Build Android (APK) — Discord Uploader Nativo

Esta pasta contém a versão nativa em Go + Fyne do Discord Uploader, com suporte
ao Android. O aplicativo é um único APK que não depende do Bash nem de qualquer
processo externo.

## Pré-requisitos

Antes de construir o APK, instale:

1. **Go 1.21+**:
   ```bash
   sudo apt install golang
   ```

2. **Android Studio** (para obter o SDK e o NDK):
   - Baixe em: https://developer.android.com/studio
   - Abra o Android Studio > SDK Manager > SDK Tools
   - Instale: **NDK (side by side)** e **CMake**

3. **Variáveis de ambiente** (adicione ao seu `~/.bashrc` ou `~/.zshrc`):
   ```bash
   export ANDROID_HOME=$HOME/Android/Sdk
   export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/<versao-do-ndk>
   export PATH=$PATH:$ANDROID_HOME/platform-tools
   ```

4. **CLI do Fyne** (instalado automaticamente pelo script, se necessário):
   ```bash
   go install fyne.io/tools/cmd/fyne@latest
   export PATH=$HOME/go/bin:$PATH
   ```

## Como Construir o APK

Na pasta `native/`:

```bash
cd native
./build-android.sh
```

O script:
1. Verifica se `ANDROID_HOME` e `ANDROID_NDK_HOME` estão configurados.
2. Resolve dependências Go (`go mod tidy`).
3. Executa `fyne package -os android` com o `app-id`
   `com.github.sevencomplex.discorduploader` e o ícone `icon.png`.
4. Gera o APK na pasta `native/`.

## Permissões Android

O `FyneApp.toml` declara as seguintes permissões:

```toml
[Android]
Permissions = ["INTERNET", "READ_EXTERNAL_STORAGE"]
```

- `INTERNET`: necessário para enviar os arquivos ao webhook do Discord.
- `READ_EXTERNAL_STORAGE`: necessário para ler arquivos selecionados pelo usuário
  via seletor de arquivos do Android (URIs `content://`).

## Como o Android é Tratado

No Android, o usuário não navega por caminhos de sistema de arquivos (`/sdcard/...`).
Em vez disso, ele usa o seletor nativo de arquivos (`dialog.NewFileOpenMultiple`),
que retorna URIs do tipo `content://...`.

O fluxo é:
1. O usuário seleciona um ou mais arquivos.
2. O app lê cada URI e copia os bytes para um arquivo temporário local.
3. O upload é feito usando a mesma lógica de retry/429/5xx do Desktop.
4. O arquivo temporário é apagado ao final.

## Limitações

- O tema AMOLED é aplicado em todas as plataformas (Desktop + Android).
- No Android, não há suporte a pastas recursivas nem checkboxes — apenas seleção
  de arquivos individuais.
- "Salvar relatório…" ainda é um placeholder.
