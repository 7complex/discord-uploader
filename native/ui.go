package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

const (
	isAndroid = runtime.GOOS == "android"
)

type DiscordUploaderWindow struct {
	app        fyne.App
	window     fyne.Window
	cfg        UploadConfig
	logLines   []string
	cancelFn   context.CancelFunc
	executando bool

	webhookEntry   *widget.Entry
	webhookFileLbl *widget.Label
	contentEntry   *widget.Entry
	btnIniciar     *widget.Button
	btnCancelar    *widget.Button
	btnSalvar      *widget.Button
	progresso      *widget.ProgressBar
	lblProgresso   *widget.Label
	listaLog       *widget.Entry
	lblResumo      *widget.Label

	folderEntry  *widget.Entry
	btnProcurar  *widget.Button
	recursivoChk *widget.Check
	ocultosChk   *widget.Check
	btnTestar    *widget.Button

	btnSelecionarArquivos *widget.Button
	lblArquivosSel        *widget.Label
	arquivosSelURI        []fyne.URI
}

func newWindow(a fyne.App, w fyne.Window) {
	ui := &DiscordUploaderWindow{
		app:        a,
		window:     w,
		cfg:        DefaultConfig(),
		logLines:   nil,
		executando: false,
	}

	ui.webhookEntry = widget.NewEntry()
	ui.webhookEntry.SetPlaceHolder("https://discord.com/api/webhooks/...")
	ui.webhookEntry.Password = true

	btnCarregarWebhook := widget.NewButton("Carregar de arquivo...", func() {
		dlg := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil { return }
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			linha := ""
			for _, l := range strings.Split(string(data), "\n") {
				l = strings.TrimRight(l, "\r")
				if strings.TrimSpace(l) != "" { linha = l; break }
			}
			ui.webhookEntry.SetText(linha)
			ui.webhookFileLbl.SetText("(" + rc.URI().Path() + ")")
		}, w)
		dlg.Show()
	})
	ui.webhookFileLbl = widget.NewLabel("")
	ui.webhookFileLbl.TextStyle = fyne.TextStyle{Italic: true}

	ui.contentEntry = widget.NewEntry()
	ui.contentEntry.SetPlaceHolder("Mensagem opcional")
	lblContentHelp := widget.NewLabel("Mensagem que acompanha cada arquivo")
	lblContentHelp.TextStyle = fyne.TextStyle{Italic: true}

	if isAndroid {
		ui.montarAndroid(w, btnCarregarWebhook, lblContentHelp)
	} else {
		ui.montarDesktop(w, btnCarregarWebhook, lblContentHelp)
	}
}

func (ui *DiscordUploaderWindow) montarDesktop(w fyne.Window, btnCarregarWebhook *widget.Button, lblContentHelp *widget.Label) {
	webhookRow := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Webhook do Discord:"), btnCarregarWebhook, ui.webhookEntry),
		container.NewHBox(layout.NewSpacer(), ui.webhookFileLbl),
	)
	contentRow := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Content (opcional):"), nil, ui.contentEntry),
		lblContentHelp,
	)

	ui.folderEntry = widget.NewEntry()
	ui.folderEntry.SetPlaceHolder("/caminho/da/pasta")
	btnProcurar := widget.NewButton("Procurar...", func() {
		dlg := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil { return }
			ui.folderEntry.SetText(uri.Path())
		}, w)
		dlg.Show()
	})
	ui.btnProcurar = btnProcurar

	pastaRow := container.NewBorder(nil, nil, widget.NewLabel("Pasta:"), btnProcurar, ui.folderEntry)

	ui.recursivoChk = widget.NewCheck("Recursivo (-r)", nil)
	ui.ocultosChk = widget.NewCheck("Ignorar ocultos (-H)", nil)
	checks := container.NewHBox(ui.recursivoChk, ui.ocultosChk)

	ui.btnTestar = widget.NewButton("Testar (dry-run)", ui.onTestar)
	ui.btnIniciar = widget.NewButton("Iniciar envio", ui.onIniciar)
	ui.btnCancelar = widget.NewButton("Cancelar", ui.onCancelar)
	ui.btnCancelar.Disable()
	btns := container.NewHBox(ui.btnTestar, ui.btnIniciar, ui.btnCancelar)

	ui.montarComum(w, webhookRow, pastaRow, checks, contentRow, btns, false)
}

func (ui *DiscordUploaderWindow) montarAndroid(w fyne.Window, btnCarregarWebhook *widget.Button, lblContentHelp *widget.Label) {
	webhookRow := container.NewVBox(
		widget.NewLabel("Webhook do Discord:"),
		ui.webhookEntry,
		btnCarregarWebhook,
		container.NewHBox(layout.NewSpacer(), ui.webhookFileLbl),
	)
	contentRow := container.NewVBox(
		widget.NewLabel("Content (opcional):"),
		ui.contentEntry,
		lblContentHelp,
	)

	ui.btnSelecionarArquivos = widget.NewButton("Selecionar Pasta de Arquivos...", ui.onSelecionarArquivosAndroid)
	ui.btnSelecionarArquivos.Importance = widget.MediumImportance
	ui.lblArquivosSel = widget.NewLabel("Nenhum arquivo selecionado")
	ui.lblArquivosSel.TextStyle = fyne.TextStyle{Italic: true}

	pastaRow := container.NewVBox(ui.btnSelecionarArquivos, ui.lblArquivosSel)

	ui.btnIniciar = widget.NewButton("Iniciar Envio", ui.onIniciarAndroid)
	ui.btnIniciar.Importance = widget.HighImportance
	ui.btnCancelar = widget.NewButton("Cancelar", ui.onCancelar)
	ui.btnCancelar.Disable()
	btns := container.NewGridWithColumns(2, ui.btnIniciar, ui.btnCancelar)

	ui.montarComum(w, webhookRow, pastaRow, nil, contentRow, btns, true)
}

func (ui *DiscordUploaderWindow) montarComum(w fyne.Window, webhookRow, pastaRow, checks, contentRow, btns *fyne.Container, mobile bool) {
	ui.progresso = widget.NewProgressBar()
	ui.progresso.Min = 0
	ui.progresso.Max = 100
	ui.progresso.Value = 0
	ui.lblProgresso = widget.NewLabel("0/0 arquivos")

	ui.listaLog = widget.NewMultiLineEntry()
	ui.listaLog.SetPlaceHolder("Aguardando execução...\n")
	ui.listaLog.Wrapping = fyne.TextWrapWord

	ui.lblResumo = widget.NewLabel("Enviados: 0 · Falhas: 0 · Incertos: 0")
	ui.btnSalvar = widget.NewButton("Salvar relatório...", ui.onSalvar)
	ui.btnSalvar.Disable()

	progressRow := container.NewVBox(ui.progresso, ui.lblProgresso)
	logHeight := float32(220)
	if mobile { logHeight = 150 }
	logScroll := container.NewScroll(ui.listaLog)
	logScroll.SetMinSize(fyne.NewSize(0, logHeight))

	resumoRow := container.NewHBox(ui.lblResumo, layout.NewSpacer(), ui.btnSalvar)

	var conteudo *fyne.Container
	if checks != nil {
		conteudo = container.NewVBox(webhookRow, widget.NewSeparator(), pastaRow, checks, widget.NewSeparator(), contentRow, widget.NewSeparator(), btns, widget.NewSeparator(), progressRow, listaTitle(), logScroll, widget.NewSeparator(), resumoRow)
	} else {
		conteudo = container.NewVBox(webhookRow, widget.NewSeparator(), pastaRow, widget.NewSeparator(), contentRow, widget.NewSeparator(), btns, widget.NewSeparator(), progressRow, listaTitle(), logScroll, widget.NewSeparator(), resumoRow)
	}

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

func (ui *DiscordUploaderWindow) onSelecionarArquivosAndroid() {
	dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		if uri == nil { return }

		ui.lblArquivosSel.SetText("Lendo pasta...")
		
		go func() {
			list, err := uri.List()
			if err != nil {
				fyneDo(func() { dialog.ShowError(fmt.Errorf("Erro ao listar arquivos: %v", err), ui.window) })
				return
			}

			var validos []fyne.URI
			for _, u := range list {
				rc, err := storage.Reader(u)
				if err == nil {
					rc.Close()
					validos = append(validos, u)
				}
			}

			fyneDo(func() {
				ui.arquivosSelURI = validos
				if len(ui.arquivosSelURI) == 0 {
					ui.lblArquivosSel.SetText("Nenhum arquivo válido na pasta")
				} else {
					ui.lblArquivosSel.SetText(fmt.Sprintf("%d arquivo(s) selecionado(s)", len(ui.arquivosSelURI)))
				}
			})
		}()
	}, ui.window).Show()
}

func (ui *DiscordUploaderWindow) onIniciarAndroid() {
	webhook := strings.TrimSpace(ui.webhookEntry.Text)
	if webhook == "" {
		dialog.ShowError(fmt.Errorf("Informe o webhook do Discord."), ui.window)
		return
	}
	if !URLValida(webhook) {
		dialog.ShowError(fmt.Errorf("O formato da URL não corresponde a um webhook do Discord."), ui.window)
		return
	}
	if len(ui.arquivosSelURI) == 0 {
		dialog.ShowError(fmt.Errorf("Selecione uma pasta com arquivos primeiro."), ui.window)
		return
	}
	ui.cfg.Content = strings.TrimSpace(ui.contentEntry.Text)
	ui.executarAndroid(webhook, ui.arquivosSelURI)
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
			cancel()
			ui.cancelFn = nil
			ui.executando = false
			fyneDo(func() {
				ui.setEstadoExecucao(false)
				if ui.progresso.Max > 0 { ui.progresso.Value = ui.progresso.Max; ui.progresso.Refresh() }
				ui.lblResumo.SetText(fmt.Sprintf("Enviados: %d · Falhas: %d · Incertos: %d", ok, falhas, incertos))
				ui.btnSalvar.Enable()
			})
		}()

		if dryRun {
			fyneDo(func() {
				ui.adicionarLog("=== Dry-run ===")
				for i, arq := range arquivos {
					nome := arq
					if rel, err := filepath.Rel(pasta, arq); err == nil { nome = rel }
					ui.adicionarLog(fmt.Sprintf("[%d/%d] %s", i+1, total, nome))
				}
				ui.adicionarLog(fmt.Sprintf("\nTotal: %d arquivo(s)\n", total))
			})
			return
		}

		for i, arq := range arquivos {
			select {
			case <-ctx.Done():
				fyneDo(func() { ui.adicionarLog("Cancelado pelo usuário.") })
				return
			default:
			}

			nome := arq
			if rel, err := filepath.Rel(pasta, arq); err == nil { nome = rel }

			result := UploadFile(ctx, webhook, arq, "", ui.cfg)

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
			if incertos > 0 || falhas > 0 {
				dialog.ShowInformation("Concluído com falhas", "Alguns arquivos não foram enviados. Veja o log.", ui.window)
			} else {
				dialog.ShowInformation("Concluído", "Envio finalizado com sucesso.", ui.window)
			}
		})
	}()
}

func (ui *DiscordUploaderWindow) executarAndroid(webhook string, arquivos []fyne.URI) {
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
			cancel()
			ui.cancelFn = nil
			ui.executando = false
			fyneDo(func() {
				ui.setEstadoExecucao(false)
				if ui.progresso.Max > 0 { ui.progresso.Value = ui.progresso.Max; ui.progresso.Refresh() }
				ui.lblResumo.SetText(fmt.Sprintf("Enviados: %d · Falhas: %d · Incertos: %d", ok, falhas, incertos))
				ui.btnSalvar.Enable()
			})
		}()

		for i, u := range arquivos {
			select {
			case <-ctx.Done():
				fyneDo(func() { ui.adicionarLog("Cancelado pelo usuário.") })
				return
			default:
			}

			nome := u.Name()
			if nome == "" { nome = u.Path() }

			// CORREÇÃO CRÍTICA: Copia o arquivo para temp LOCAL e fecha a URI IMEDIATAMENTE
			rc, err := storage.Reader(u)
			if err != nil {
				fyneDo(func() {
					falhas++
					ui.adicionarLog(fmt.Sprintf("✘ [%d/%d] %s — Falha ao abrir arquivo", i+1, total, nome))
				})
				continue
			}

			tmp, err := os.CreateTemp("", "discord-upload-*.tmp")
			if err != nil {
				rc.Close()
				fyneDo(func() {
					falhas++
					ui.adicionarLog(fmt.Sprintf("✘ [%d/%d] %s — Falha ao criar temp", i+1, total, nome))
				})
				continue
			}
			tmpPath := tmp.Name()

			_, copyErr := io.Copy(tmp, rc)
			rc.Close() // Fecha o arquivo do Android JÁ! (Evita crash)
			tmp.Close()

			if copyErr != nil {
				os.Remove(tmpPath)
				fyneDo(func() {
					falhas++
					ui.adicionarLog(fmt.Sprintf("✘ [%d/%d] %s — Falha ao copiar", i+1, total, nome))
				})
				continue
			}

			// Agora envia o arquivo temporário (é seguro)
			result := UploadFile(ctx, webhook, tmpPath, nome, ui.cfg)
			os.Remove(tmpPath)

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
			if incertos > 0 || falhas > 0 {
				dialog.ShowInformation("Concluído com falhas", "Alguns arquivos não foram enviados. Veja o log.", ui.window)
			} else {
				dialog.ShowInformation("Concluído", "Envio finalizado com sucesso.", ui.window)
			}
		})
	}()
}

func (ui *DiscordUploaderWindow) setEstadoExecucao(executando bool) {
	habilitar := !executando
	if ui.btnTestar != nil { if habilitar { ui.btnTestar.Enable() } else { ui.btnTestar.Disable() } }
	if ui.btnIniciar != nil { if habilitar { ui.btnIniciar.Enable() } else { ui.btnIniciar.Disable() } }
	if ui.btnCancelar != nil { if executando { ui.btnCancelar.Enable() } else { ui.btnCancelar.Disable() } }
	if ui.webhookEntry != nil { if habilitar { ui.webhookEntry.Enable() } else { ui.webhookEntry.Disable() } }
	if ui.folderEntry != nil { if habilitar { ui.folderEntry.Enable() } else { ui.folderEntry.Disable() } }
	if ui.recursivoChk != nil { if habilitar { ui.recursivoChk.Enable() } else { ui.recursivoChk.Disable() } }
	if ui.ocultosChk != nil { if habilitar { ui.ocultosChk.Enable() } else { ui.ocultosChk.Disable() } }
	if ui.btnSelecionarArquivos != nil { if habilitar { ui.btnSelecionarArquivos.Enable() } else { ui.btnSelecionarArquivos.Disable() } }
	if ui.contentEntry != nil { if habilitar { ui.contentEntry.Enable() } else { ui.contentEntry.Disable() } }
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
	if ui.cancelFn != nil { ui.cancelFn() }
}

func (ui *DiscordUploaderWindow) onSalvar() {
	dialog.ShowInformation("Salvar relatório", "Funcionalidade a ser preenchida com o caminho do relatório gerado.", ui.window)
}

func fyneDo(f func()) {
	fyne.Do(f)
}