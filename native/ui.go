package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type DiscordUploaderWindow struct {
	app    fyne.App
	window fyne.Window

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
	cfg      UploadConfig

	// estado de execução
	cancelFn   context.CancelFunc
	executando bool
}

func setEnabled(o fyne.Disableable, enabled bool) {
	if enabled {
		o.Enable()
	} else {
		o.Disable()
	}
}

func newWindow(a fyne.App, w fyne.Window) {
	ui := &DiscordUploaderWindow{
		app:        a,
		window:     w,
		cfg:        DefaultConfig(),
		logLines:   nil,
		executando: false,
	}

	// 1. Webhook + carregar de arquivo
	ui.webhookEntry = widget.NewEntry()
	ui.webhookEntry.SetPlaceHolder("https://discord.com/api/webhooks/...")
	ui.webhookEntry.Password = true

	btnCarregarWebhook := widget.NewButton("Carregar de arquivo...", func() {
		// NewFileOpen devolve fyne.URIReadCloser
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
		// NewFolderOpen devolve ListableURI
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
	arquivos, err := EncontraArquivos(pasta, ui.recursivoChk.Checked, ui.ocultosChk.Checked)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Erro ao listar arquivos: %v", err), ui.window)
		return
	}
	ui.executar(true, pasta, "", arquivos)
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
	if !URLValida(webhook) {
		dialog.ShowError(fmt.Errorf("O formato da URL não corresponde a um webhook do Discord."), ui.window)
		return
	}
	arquivos, err := EncontraArquivos(pasta, ui.recursivoChk.Checked, ui.ocultosChk.Checked)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Erro ao listar arquivos: %v", err), ui.window)
		return
	}
	ui.cfg.Content = strings.TrimSpace(ui.contentEntry.Text)
	ui.executar(false, pasta, webhook, arquivos)
}

func (ui *DiscordUploaderWindow) executar(dryRun bool, pasta, webhook string, arquivos []string) {
	ui.executando = true
	ui.setEstadoExecucao(true)
	ui.limparLog()
	ui.progresso.Value = 0
	ui.progresso.Max = float64(len(arquivos))
	ui.lblProgresso.SetText(fmt.Sprintf("0/%d arquivos", len(arquivos)))
	ui.lblResumo.SetText("Enviados: 0 · Falhas: 0 · Incertos: 0")
	ui.btnSalvar.Disable()

	total := len(arquivos)
	var ok, falhas, incertos int64

	ctx, cancel := context.WithCancel(context.Background())
	ui.cancelFn = cancel

	go func() {
		defer func() {
			cancel() // Previne memory leak no context
			ui.cancelFn = nil
			ui.executando = false
			fyneDo(func() {
				ui.setEstadoExecucao(false)
				if ui.progresso.Max > 0 {
					ui.progresso.Value = ui.progresso.Max
					ui.progresso.Refresh()
				}
				ui.lblResumo.SetText(fmt.Sprintf("Enviados: %d · Falhas: %d · Incertos: %d", ok, falhas, incertos))
				ui.btnSalvar.Enable()
			})
		}()

		if dryRun {
			fyneDo(func() {
				ui.adicionarLog("=== Dry-run ===")
				for i, arq := range arquivos {
					nome := arq
					if rel, err := filepath.Rel(pasta, arq); err == nil {
						nome = rel
					}
					ui.adicionarLog(fmt.Sprintf("[%d/%d] %s", i+1, total, nome))
				}
				ui.adicionarLog(fmt.Sprintf("\nTotal: %d arquivo(s)\n", total))
			})
			return
		}

		for i, arq := range arquivos {
			// Verifica cancelamento antes de iniciar o próximo
			select {
			case <-ctx.Done():
				fyneDo(func() { ui.adicionarLog("Cancelado pelo usuário.") })
				return
			default:
			}

			nome := arq
			if rel, err := filepath.Rel(pasta, arq); err == nil {
				nome = rel
			}

			result := UploadFile(ctx, webhook, arq, ui.cfg)

			fyneDo(func() {
				atual := i + 1
				ui.progresso.Max = float64(total)
				ui.progresso.Value = float64(atual)
				ui.progresso.Refresh()
				ui.lblProgresso.SetText(fmt.Sprintf("%d/%d arquivos", atual, total))

				if result.Success {
					ok++
					ui.adicionarLog(fmt.Sprintf("✔ [%d/%d] %s — %s", atual, total, nome, result.Message))
				} else if result.Uncertain {
					incertos++
					ui.adicionarLog(fmt.Sprintf("⚠ [%d/%d] %s — %s", atual, total, nome, result.Message))
				} else {
					falhas++
					ui.adicionarLog(fmt.Sprintf("✘ [%d/%d] %s — %s", atual, total, nome, result.Message))
				}
				ui.lblResumo.SetText(fmt.Sprintf("Enviados: %d · Falhas: %d · Incertos: %d", ok, falhas, incertos))
			})
		}

		fyneDo(func() {
			switch {
			case incertos > 0 || falhas > 0:
				dialog.ShowInformation("Concluído com falhas", "Alguns arquivos não foram enviados. Veja o log.", ui.window)
			default:
				dialog.ShowInformation("Concluído", "Envio finalizado com sucesso.", ui.window)
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

func (ui *DiscordUploaderWindow) adicionarLog(linha string) {
	ui.logLines = append(ui.logLines, linha)
	ui.listaLog.SetText(strings.Join(ui.logLines, "\n"))
	ui.listaLog.CursorRow = len(ui.logLines)
}

func (ui *DiscordUploaderWindow) onCancelar() {
	if ui.cancelFn != nil {
		ui.cancelFn()
	}
}

func (ui *DiscordUploaderWindow) onSalvar() {
	dialog.ShowInformation("Salvar relatório", "Funcionalidade a ser preenchida com o caminho do relatório gerado.", ui.window)
}

func fyneDo(f func()) {
	f()
}