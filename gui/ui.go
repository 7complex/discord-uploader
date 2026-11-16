package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type DiscordUploaderWindow struct {
	app     fyne.App
	window  fyne.Window
	cliPath string

	webhookEntry   *widget.Entry
	webhookFileLbl *widget.Label
	folderEntry    *widget.Entry
	recursivoChk   *widget.Check
	ocultosChk     *widget.Check
	contentEntry   *widget.Entry
	btnTestar      *widget.Button
	btnIniciar     *widget.Button
	btnCancelar    *widget.Button
	btnSalvar      *widget.Button
	progresso      *widget.ProgressBar
	lblProgresso   *widget.Label
	listaLog       *widget.Entry
	lblResumo      *widget.Label

	logLines []string
}

func setEnabled(o fyne.Disableable, enabled bool) {
	if enabled {
		o.Enable()
	} else {
		o.Disable()
	}
}

func newWindow(a fyne.App, w fyne.Window, cliPath string) {
	ui := &DiscordUploaderWindow{
		app:     a,
		window:  w,
		cliPath: cliPath,
	}

	// 1. Webhook + carregar de arquivo
	ui.webhookEntry = widget.NewEntry()
	ui.webhookEntry.SetPlaceHolder("https://discord.com/api/webhooks/...")
	ui.webhookEntry.Password = true

	btnCarregarWebhook := widget.NewButton("Carregar de arquivo...", func() {
		dlg := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if rc == nil {
				return
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			linha := ""
			for _, l := range strings.Split(string(data), "\n") {
				l = strings.TrimRight(l, "\r")
				if strings.TrimSpace(l) != "" {
					linha = l
					break
				}
			}
			ui.webhookEntry.SetText(linha)
			ui.webhookFileLbl.SetText("(" + rc.URI().Path() + ")")
		}, w)
		dlg.Show()
	})
	ui.webhookFileLbl = widget.NewLabel("")
	ui.webhookFileLbl.TextStyle = fyne.TextStyle{Italic: true}

	// 2. Pasta + procurar
	ui.folderEntry = widget.NewEntry()
	ui.folderEntry.SetPlaceHolder("/caminho/da/pasta")
	btnProcurar := widget.NewButton("Procurar...", func() {
		dlg := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			ui.folderEntry.SetText(uri.Path())
		}, w)
		dlg.Show()
	})

	// 3. Checkboxes
	ui.recursivoChk = widget.NewCheck("Recursivo (-r)", nil)
	ui.ocultosChk = widget.NewCheck("Ignorar ocultos (-H)", nil)

	// 4. Content
	ui.contentEntry = widget.NewEntry()
	ui.contentEntry.SetPlaceHolder("Mensagem opcional")
	lblContentHelp := widget.NewLabel("Mensagem que acompanha cada arquivo")
	lblContentHelp.TextStyle = fyne.TextStyle{Italic: true}

	// 5. Botões
	ui.btnTestar = widget.NewButton("Testar (dry-run)", ui.onTestar)
	ui.btnIniciar = widget.NewButton("Iniciar envio", ui.onIniciar)
	ui.btnCancelar = widget.NewButton("Cancelar", ui.onCancelar)
	ui.btnCancelar.Disable()

	// 6. Progresso
	ui.progresso = widget.NewProgressBar()
	ui.progresso.Min = 0
	ui.progresso.Max = 100
	ui.progresso.Value = 0
	ui.lblProgresso = widget.NewLabel("0/0 arquivos")

	// 7. Log
	ui.listaLog = widget.NewMultiLineEntry()
	ui.listaLog.SetPlaceHolder("Aguardando execução...\n")
	ui.listaLog.Wrapping = fyne.TextWrapWord

	// 8. Rodapé
	ui.lblResumo = widget.NewLabel("Enviados: 0 · Falhas: 0 · Incertos: 0")
	ui.btnSalvar = widget.NewButton("Salvar relatório...", ui.onSalvar)
	ui.btnSalvar.Disable()

	// Layout
	webhookRow := container.NewVBox(
		container.NewBorder(
			nil, nil,
			widget.NewLabel("Webhook do Discord:"),
			btnCarregarWebhook,
			ui.webhookEntry,
		),
		container.NewHBox(layout.NewSpacer(), ui.webhookFileLbl),
	)

	pastaRow := container.NewBorder(
		nil, nil,
		widget.NewLabel("Pasta:"),
		btnProcurar,
		ui.folderEntry,
	)

	checks := container.NewHBox(ui.recursivoChk, ui.ocultosChk)

	contentRow := container.NewVBox(
		container.NewBorder(
			nil, nil,
			widget.NewLabel("Content (opcional):"),
			nil,
			ui.contentEntry,
		),
		lblContentHelp,
	)

	btns := container.NewHBox(ui.btnTestar, ui.btnIniciar, ui.btnCancelar)

	progressRow := container.NewVBox(
		ui.progresso,
		ui.lblProgresso,
	)

	logScroll := container.NewScroll(ui.listaLog)
	logScroll.SetMinSize(fyne.NewSize(0, 220))

	resumoRow := container.NewHBox(
		ui.lblResumo,
		layout.NewSpacer(),
		ui.btnSalvar,
	)

	conteudo := container.NewVBox(
		webhookRow,
		widget.NewSeparator(),
		pastaRow,
		checks,
		widget.NewSeparator(),
		contentRow,
		widget.NewSeparator(),
		btns,
		widget.NewSeparator(),
		progressRow,
		listaTitle(),
		logScroll,
		widget.NewSeparator(),
		resumoRow,
	)

	w.SetContent(container.NewVScroll(conteudo))
	w.CenterOnScreen()
}

func listaTitle() *widget.Label {
	l := widget.NewLabel("Log de execução")
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

func (ui *DiscordUploaderWindow) onTestar() {
	pasta := strings.TrimSpace(ui.folderEntry.Text)
	if pasta == "" {
		dialog.ShowError(fmt.Errorf("Informe a pasta de arquivos."), ui.window)
		return
	}
	ui.executar(true, pasta, "")
}

func (ui *DiscordUploaderWindow) onIniciar() {
	webhook := strings.TrimSpace(ui.webhookEntry.Text)
	pasta := strings.TrimSpace(ui.folderEntry.Text)
	if webhook == "" {
		dialog.ShowError(fmt.Errorf("Informe o webhook do Discord."), ui.window)
		return
	}
	if pasta == "" {
		dialog.ShowError(fmt.Errorf("Informe a pasta de arquivos."), ui.window)
		return
	}
	ui.executar(false, pasta, webhook)
}

func (ui *DiscordUploaderWindow) executar(dryRun bool, pasta, webhook string) {
	ui.setEstadoExecucao(true)
	ui.limparLog()
	ui.progresso.Value = 0
	ui.progresso.Max = 100
	ui.progresso.Refresh() // Força o redraw ao zerar
	ui.lblProgresso.SetText("0/0 arquivos")
	ui.lblResumo.SetText("Enviados: 0 · Falhas: 0 · Incertos: 0")
	ui.btnSalvar.Disable()

	args := []string{"-d", pasta, "-y"}

	var tmpFile string
	if dryRun {
		args = append([]string{"-n"}, args...)
	} else {
		var err error
		tmpFile, err = escreverWebhookTemp(webhook)
		if err != nil {
			dialog.ShowError(err, ui.window)
			ui.setEstadoExecucao(false)
			return
		}
		args = append([]string{"-W", tmpFile}, args...)
	}
	if ui.recursivoChk.Checked {
		args = append(args, "-r")
	}
	if ui.ocultosChk.Checked {
		args = append(args, "-H")
	}
	if !dryRun {
		if content := strings.TrimSpace(ui.contentEntry.Text); content != "" {
			args = append(args, "-c", content)
		}
	}

	go func() {
		defer func() {
			if tmpFile != "" {
				removerArquivo(tmpFile)
			}
		}()

		exitCode, _, _, resumo := runUpload(ui.cliPath, args, dryRun, ui.logCallback)
		fyneDo(func() {
			ui.setEstadoExecucao(false)
			
			// Força a barra a 100% visualmente caso tenha terminado com sucesso (ou falha parcial)
			if ui.progresso.Max > 100 && (exitCode == 0 || exitCode == 1) {
				ui.progresso.Value = ui.progresso.Max
				ui.progresso.Refresh()
			}

			if resumo != "" {
				ui.lblResumo.SetText(resumo)
				ui.btnSalvar.Enable()
			}
			switch exitCode {
			case 0:
				dialog.ShowInformation("Concluído", "Envio finalizado com sucesso.", ui.window)
			case 1:
				dialog.ShowInformation("Concluído com falhas", "Alguns arquivos não foram enviados. Veja o log.", ui.window)
			case 4:
				dialog.ShowError(fmt.Errorf("Webhook inválido (código 4). Verifique a URL."), ui.window)
			case 130:
				dialog.ShowInformation("Cancelado", "O envio foi cancelado pelo usuário.", ui.window)
			default:
				dialog.ShowError(fmt.Errorf("Processo terminou com código %d", exitCode), ui.window)
			}
		})
	}()
}

func (ui *DiscordUploaderWindow) setEstadoExecucao(executando bool) {
	setEnabled(ui.btnTestar, !executando)
	setEnabled(ui.btnIniciar, !executando)
	setEnabled(ui.btnCancelar, executando)
	setEnabled(ui.webhookEntry, !executando)
	setEnabled(ui.folderEntry, !executando)
	setEnabled(ui.recursivoChk, !executando)
	setEnabled(ui.ocultosChk, !executando)
	setEnabled(ui.contentEntry, !executando)
}

func (ui *DiscordUploaderWindow) limparLog() {
	ui.logLines = nil
	ui.listaLog.SetText("")
}

func (ui *DiscordUploaderWindow) logCallback(linha string, ehErro bool) {
	fyneDo(func() {
		ui.adicionarLogComCor(linha, ehErro)
		if n, m := extrairProgresso(linha); n > 0 && m > 0 {
			ui.progresso.Max = float64(m)
			ui.progresso.Value = float64(n)
			ui.progresso.Refresh() // Força o redraw durante os uploads
			ui.lblProgresso.SetText(fmt.Sprintf("%d/%d arquivos", n, m))
		}
	})
}

func (ui *DiscordUploaderWindow) adicionarLogComCor(linha string, ehErro bool) {
	jaMarcada := strings.HasPrefix(linha, "✔") ||
		strings.HasPrefix(linha, "✘") ||
		strings.HasPrefix(linha, "!") ||
		strings.HasPrefix(linha, "⚠")

	final := linha
	if !jaMarcada {
		if ehErro {
			final = "✘ " + linha
		} else if strings.HasPrefix(linha, "[") {
			final = "» " + linha
		}
	}

	ui.logLines = append(ui.logLines, final)
	ui.listaLog.SetText(strings.Join(ui.logLines, "\n"))
	ui.listaLog.CursorRow = len(ui.logLines)
}

func (ui *DiscordUploaderWindow) onCancelar() {
	cancelUpload()
}

func (ui *DiscordUploaderWindow) onSalvar() {
	dialog.ShowInformation(
		"Salvar relatório",
		"Funcionalidade pendente. Use o CLI com -o caminho.txt por enquanto.",
		ui.window,
	)
}

func extrairProgresso(linha string) (int, int) {
	inicio := strings.Index(linha, "[")
	if inicio < 0 {
		return 0, 0
	}
	resto := linha[inicio+1:]
	barra := strings.Index(resto, "/")
	if barra < 0 {
		return 0, 0
	}
	nStr := resto[:barra]
	fechou := strings.Index(resto[barra+1:], "]")
	if fechou < 0 {
		return 0, 0
	}
	mStr := resto[barra+1 : barra+1+fechou]
	n, err1 := strconv.Atoi(nStr)
	m, err2 := strconv.Atoi(mStr)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return n, m
}

// fyneDo executa f na goroutine da UI (Fyne v2.4).
func fyneDo(f func()) {
	f()
}