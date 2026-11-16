package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Estado do subprocesso em execução. Protegido por cancelMux.
var (
	cancelMux  sync.Mutex
	cancelProc *os.Process
)

// escreverWebhookTemp grava o webhook em arquivo temp com 0600 (nunca em argv).
func escreverWebhookTemp(webhook string) (string, error) {
	f, err := os.CreateTemp("", "discord-webhook-*.txt")
	if err != nil {
		return "", err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if _, err := f.WriteString(webhook); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func removerArquivo(path string) {
	_ = os.Remove(path)
}

// cancelUpload envia SIGTERM ao subprocesso e escala para SIGKILL após 5s
// se ele não sair. O trap do CLI captura SIGTERM para fazer cleanup.
func cancelUpload() {
	cancelMux.Lock()
	proc := cancelProc
	cancelMux.Unlock()

	if proc == nil {
		return
	}

	_ = proc.Signal(syscall.SIGTERM)

	go func(p *os.Process) {
		time.Sleep(5 * time.Second)
		if err := p.Signal(syscall.Signal(0)); err == nil {
			_ = p.Kill()
		}
	}(proc)
}

// runUpload executa o discord-uploader.sh via subprocesso.
// Retorno: (exitCode, stdoutCompleto, linhasDeStderr, resumo).
func runUpload(cliPath string, args []string, dryRun bool, logCb func(string, bool)) (int, string, []string, string) {
	_ = dryRun

	cmd := exec.Command(cliPath, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return -1, "", []string{fmt.Sprintf("erro ao abrir stdout: %v", err)}, ""
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return -1, "", []string{fmt.Sprintf("erro ao abrir stderr: %v", err)}, ""
	}

	if err := cmd.Start(); err != nil {
		return -1, "", []string{fmt.Sprintf("erro ao iniciar: %v", err)}, ""
	}

	cancelMux.Lock()
	cancelProc = cmd.Process
	cancelMux.Unlock()

	defer func() {
		cancelMux.Lock()
		if cancelProc == cmd.Process {
			cancelProc = nil
		}
		cancelMux.Unlock()
	}()

	var (
		saidaMu sync.Mutex
		saida   strings.Builder
		errosMu sync.Mutex
		erros   []string
	)

	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			linha := scanner.Text()
			saidaMu.Lock()
			saida.WriteString(linha + "\n")
			saidaMu.Unlock()
			logCb(linha, false)
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			linha := scanner.Text()
			errosMu.Lock()
			erros = append(erros, linha)
			errosMu.Unlock()
			logCb(linha, true)
		}
	}()

	<-done
	<-done

	_ = cmd.Wait()

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	saidaMu.Lock()
	saidaStr := saida.String()
	saidaMu.Unlock()

	errosMu.Lock()
	errosCopia := append([]string(nil), erros...)
	errosMu.Unlock()

	resumo := montarResumo(saidaStr, exitCode)

	return exitCode, saidaStr, errosCopia, resumo
}

// montarResumo extrai estatísticas do stdout do CLI para o rodapé.
func montarResumo(saida string, exitCode int) string {
	ok := 0
	falhas := 0
	incertos := 0
	for _, linha := range strings.Split(saida, "\n") {
		switch {
		case strings.Contains(linha, "✔ Enviado"):
			ok++
		case strings.Contains(linha, "incerto") || strings.Contains(linha, "429"):
			incertos++
		case strings.Contains(linha, "✘") ||
			strings.Contains(linha, "Falha permanente") ||
			strings.Contains(linha, "Erro do curl"):
			falhas++
		}
	}
	if ok == 0 && falhas == 0 && incertos == 0 && exitCode == 0 {
		return "Nenhum arquivo enviado"
	}
	return fmt.Sprintf("Enviados: %d · Falhas: %d · Incertos: %d", ok, falhas, incertos)
}

// localizarCLI procura o discord-uploader.sh.
func localizarCLI() string {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, cliName),
			filepath.Join(dir, "..", cliName),
		)
	}
	candidates = append(candidates,
		cliName,
		filepath.Join(".", cliName),
	)

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath(cliName); err == nil {
		return p
	}
	return ""
}