package main

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
)

const version = "1.0.0"

// guiAppID é o identificador da aplicação (.desktop / executável).
const guiAppID = "com.github.yan.discord-uploader"

// defaultMaxTentativas é o limite padrão de tentativas por arquivo.
const defaultMaxTentativas = 4

// defaultMaxRetryAfter é o teto máximo de espera por Retry-After (segundos).
const defaultMaxRetryAfter = 60

func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme()) // tema escuro fixo

	w := a.NewWindow("Discord Uploader")
	w.Resize(fyne.NewSize(700, 600))
	w.SetMaster()

	// Checagem de dependências do host (apenas aviso; o app nativo não precisa
	// de bash/curl, mas avisa se faltar algo essencial ao sistema).
	if faltantes := checkDependencies(); len(faltantes) > 0 {
		mensagem := "Ferramentas ausentes: " + strings.Join(faltantes, ", ") +
			"\n\nO app nativo não depende do Bash, mas algumas ferramentas do sistema podem ser úteis."
		dialog.ShowError(errors.New(mensagem), w)
		// continua mesmo assim
	}

	newWindow(a, w)
	w.ShowAndRun()
}

// checkDependencies verifica ferramentas úteis no host (apenas aviso).
func checkDependencies() []string {
	ferramentas := []string{}
	// O app nativo não requer bash/curl/etc. Retornamos vazio por padrão.
	// Mantemos a assinatura para compatibilidade com a UI.
	return ferramentas
}

// placeholder para evitar import não usado (version/guiAppID são exibidos futuramente).
var _ = version
var _ = guiAppID
