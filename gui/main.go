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

const cliName = "discord-uploader.sh"

func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme()) // tema escuro fixo

	w := a.NewWindow("Discord Uploader " + version)
	w.Resize(fyne.NewSize(700, 600))
	w.SetMaster()

	// Checagem de dependências do host (apenas aviso, não aborta).
	if faltantes := checkDependencies(); len(faltantes) > 0 {
		mensagem := "Ferramentas ausentes: " + strings.Join(faltantes, ", ") +
			"\n\nInstale com:\n  sudo apt install bash curl findutils coreutils gawk sed grep"
		dialog.ShowError(errors.New(mensagem), w)
	}

	// localizarCLI() está em runner.go.
	cliPath := localizarCLI()
	if cliPath == "" {
		dialog.ShowError(errors.New(
			"discord-uploader.sh não encontrado.\n\n"+
				"Coloque o arquivo ao lado deste binário ou instale-o no PATH."), w)
		return
	}

	newWindow(a, w, cliPath)
	w.ShowAndRun()
}	