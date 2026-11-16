package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// UploadResult é o resultado do upload de um único arquivo.
type UploadResult struct {
	Success   bool
	Uncertain bool // true se o servidor pode ter recebido o arquivo (não reenviar cegamente)
	Message   string
	BytesSent int64
}

// UploadConfig contém as configurações de upload.
type UploadConfig struct {
	MaxTentativas  int
	BackoffInicial time.Duration
	BackoffMaximo  time.Duration
	MaxRetryAfter  time.Duration
	CurlMaxTime    time.Duration // timeout de cada requisição HTTP
	Content        string
	ThreadID       string
	ThreadName     string
	Quiet          bool
	Verbose        bool
}

// DefaultConfig retorna uma configuração com valores padrão.
func DefaultConfig() UploadConfig {
	return UploadConfig{
		MaxTentativas:  defaultMaxTentativas,
		BackoffInicial: 2 * time.Second,
		BackoffMaximo:  30 * time.Second,
		MaxRetryAfter:  defaultMaxRetryAfter * time.Second,
		CurlMaxTime:    300 * time.Second,
	}
}

// UploadFile envia um único arquivo para o webhook do Discord, com retry e
// classificação de resposta fiel ao script Bash.
func UploadFile(ctx context.Context, webhookURL string, arquivo string, cfg UploadConfig) UploadResult {
	result := UploadResult{}

	info, err := os.Stat(arquivo)
	if err != nil {
		result.Message = fmt.Sprintf("Não foi possível ler o arquivo: %v", err)
		return result
	}
	result.BytesSent = info.Size()

	f, err := os.Open(arquivo)
	if err != nil {
		result.Message = fmt.Sprintf("Não foi possível abrir o arquivo: %v", err)
		return result
	}
	defer f.Close()

	nomeBase := filepath.Base(arquivo)
	tentativa := 1
	espera := cfg.BackoffInicial

	for {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Uncertain = false
			result.Message = "Cancelado antes de iniciar o envio."
			return result
		default:
		}

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("files[0]", nomeBase)
		if err != nil {
			result.Message = fmt.Sprintf("Erro ao montar multipart: %v", err)
			return result
		}
		if _, err := io.Copy(part, f); err != nil {
			result.Message = fmt.Sprintf("Erro ao ler o arquivo: %v", err)
			return result
		}
		// Volta ao início para a próxima tentativa
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			result.Message = fmt.Sprintf("Erro ao reposicionar o arquivo: %v", err)
			return result
		}

		if cfg.Content != "" {
			writer.WriteField("content", cfg.Content)
		}
		if cfg.ThreadID != "" {
			writer.WriteField("thread_id", cfg.ThreadID)
		}
		if cfg.ThreadName != "" {
			writer.WriteField("thread_name", cfg.ThreadName)
		}

		if err := writer.Close(); err != nil {
			result.Message = fmt.Sprintf("Erro ao fechar multipart: %v", err)
			return result
		}

		req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, body)
		if err != nil {
			result.Message = fmt.Sprintf("Erro ao criar requisição: %v", err)
			return result
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		reqCtx, cancel := context.WithTimeout(ctx, cfg.CurlMaxTime)
		req = req.WithContext(reqCtx)

		resp, err := http.DefaultClient.Do(req)

		// Correção: Garante que o context é liberado
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				result.Message = "Cancelado durante a requisição."
				return result
			}
			if tentativa < cfg.MaxTentativas {
				msg := fmt.Sprintf("Falha de conexão antes do envio (%v). Tentando novamente em %s...", err, espera)
				if !cfg.Quiet {
					fmt.Println(msg)
				}
				select {
				case <-ctx.Done():
					result.Message = "Cancelado durante o retry."
					return result
				case <-time.After(espera):
				}
				espera = backoffProximo(espera, cfg.BackoffMaximo)
				tentativa++
				continue
			}
			result.Message = fmt.Sprintf("Falha de conexão após %d tentativa(s): %v", cfg.MaxTentativas, err)
			return result
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel() // Libera o context após ler o body

		httpCode := resp.StatusCode

		if httpCode == 204 {
			result.Success = true
			result.Message = "Enviado."
			return result
		}

		if httpCode == 200 && bytes.Contains(respBody, []byte(`"id"`)) {
			result.Success = true
			result.Message = "Enviado."
			return result
		}

		if httpCode == 429 {
			retryAfter := extrairRetryAfter(resp.Header, respBody)
			if retryAfter > cfg.MaxRetryAfter {
				result.Message = fmt.Sprintf("Rate limit (429): servidor pediu %ds, acima do teto %ds.", int(retryAfter.Seconds()), int(cfg.MaxRetryAfter.Seconds()))
				return result
			}
			if tentativa < cfg.MaxTentativas {
				msg := fmt.Sprintf("Rate limit (429). Aguardando %ds...", int(retryAfter.Seconds()))
				if !cfg.Quiet {
					fmt.Println(msg)
				}
				select {
				case <-ctx.Done():
					result.Message = "Cancelado durante o rate limit."
					return result
				case <-time.After(retryAfter):
				}
				tentativa++
				continue
			}
			result.Message = fmt.Sprintf("Rate limit (429) esgotado após %d tentativa(s).", cfg.MaxTentativas)
			return result
		}

		if httpCode == 413 {
			result.Message = "Arquivo maior que o limite aceito pelo Discord (HTTP 413)."
			return result
		}

		if httpCode >= 500 && httpCode <= 599 {
			result.Uncertain = true
			result.Message = fmt.Sprintf("HTTP %d; resultado incerto. Verifique o canal antes de reenviar.", httpCode)
			return result
		}

		if httpCode == 200 {
			result.Uncertain = true
			result.Message = "HTTP 200 sem confirmação; resultado incerto. Verifique o canal."
			return result
		}

		result.Message = fmt.Sprintf("Falha permanente. HTTP %d.", httpCode)
		return result
	}
}

func extrairRetryAfter(header http.Header, body []byte) time.Duration {
	if v := header.Get("Retry-After"); v != "" {
		if d, err := parseRetryAfter(v); err == nil {
			return d
		}
	}
	// Usando JSON parse estruturado em vez de string hack
	var data struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &data); err == nil && data.RetryAfter > 0 {
		return time.Duration(data.RetryAfter * float64(time.Second))
	}
	return 0
}

func parseRetryAfter(v string) (time.Duration, error) {
	v = strings.TrimSpace(v)
	if s, err := strconv.ParseFloat(v, 64); err == nil {
		return time.Duration(s * float64(time.Second)), nil
	}
	if t, err := time.Parse(time.RFC1123, v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("inválido")
}

func backoffProximo(atual, maximo time.Duration) time.Duration {
	proximo := atual * 2
	if proximo > maximo {
		return maximo
	}
	return proximo
}

func EncontraArquivos(pasta string, recursivo, ignorarOcultos bool) ([]string, error) {
	var resultado []string
	caminhoFn := func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ignorarOcultos && strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		resultado = append(resultado, path)
		return nil
	}
	if recursivo {
		if err := filepath.WalkDir(pasta, caminhoFn); err != nil {
			return nil, err
		}
	} else {
		entradas, err := os.ReadDir(pasta)
		if err != nil {
			return nil, err
		}
		for _, d := range entradas {
			if d.IsDir() {
				continue
			}
			if ignorarOcultos && strings.HasPrefix(d.Name(), ".") {
				continue
			}
			resultado = append(resultado, filepath.Join(pasta, d.Name()))
		}
	}
	return resultado, nil
}

func URLValida(webhook string) bool {
	u, err := url.Parse(webhook)
	if err != nil || u.Scheme != "https" {
		return false
	}

	hostsValidos := []string{"discord.com", "discordapp.com", "ptb.discord.com", "canary.discord.com"}
	hostOK := false
	for _, h := range hostsValidos {
		if u.Host == h || strings.HasSuffix(u.Host, "."+h) {
			hostOK = true
			break
		}
	}
	if !hostOK {
		return false
	}
	if !strings.HasPrefix(u.Path, "/api/webhooks/") && !strings.HasPrefix(u.Path, "/api/v") {
		return false
	}
	return true
}