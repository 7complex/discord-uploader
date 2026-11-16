#!/usr/bin/env bash
#
# Discord Uploader
# Envia arquivos para um canal do Discord via webhook.
#
# Licença: MIT
# Requer Bash 4.4+ e date com suporte a -d (GNU coreutils; Debian/Ubuntu).

set -u
set -o pipefail

# Números sempre com ponto (evita "19,49 MiB" em locales com vírgula).
export LC_NUMERIC=C

# Detecção de plataforma para portabilidade mínima (Linux/macOS).
UNAME_S="$(uname -s 2>/dev/null || echo Linux)"

# ===== Configurações (sobrescrevíveis por variáveis de ambiente) =====
DISCORD_MAX_BYTES="${DISCORD_MAX_BYTES:-10485760}"   # 10 MiB (conservador)
MAX_TENTATIVAS="${MAX_TENTATIVAS:-4}"
BACKOFF_INICIAL="${BACKOFF_INICIAL:-2}"
BACKOFF_MAXIMO="${BACKOFF_MAXIMO:-30}"
CURL_MAX_TIME="${CURL_MAX_TIME:-300}"
MAX_RETRY_AFTER="${MAX_RETRY_AFTER:-60}"

# WEBHOOK é preenchido pelo parser/env/prompt (set -u exige inicialização).
WEBHOOK=""

VERSION="1.0.1"

# ===== Cores =====
if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
    RESET=$'\033[0m'
    BLUE=$'\033[38;2;88;101;242m'
    WHITE=$'\033[97m'
    GREEN=$'\033[32m'
    RED=$'\033[31m'
    YELLOW=$'\033[33m'
    GRAY=$'\033[90m'
    BOLD=$'\033[1m'
else
    RESET=''; BLUE=''; WHITE=''; GREEN=''; RED=''; YELLOW=''; GRAY=''; BOLD=''
fi

info()    { printf '%s%s%s\n' "$WHITE"  "$*" "$RESET"; }
success() { printf '%s✔ %s%s\n' "$GREEN" "$*" "$RESET"; }
failure() { printf '%s✘ %s%s\n' "$RED"   "$*" "$RESET"; }
warning() { printf '%s! %s%s\n' "$YELLOW" "$*" "$RESET"; }
nota()    { printf '%s%s%s\n' "$GRAY"  "$*" "$RESET"; }

clear_screen() { [[ -t 1 ]] && printf '\033[2J\033[H'; }

banner() {
    printf '%s' "$BLUE"
    cat <<'EOF'
 ____  _                       _
|  _ \(_)___  ___ ___  _ __ __| |
| | | | / __|/ __/ _ \| '__/ _` |
| |_| | \__ \ (_| (_) | | | (_| |
|____/|_|___/\___\___/|_|  \__,_|

EOF
    printf '%s' "$RESET"
    printf '%s             File Transfer Tool%s\n\n' "$GRAY" "$RESET"
}

# ===== Utilidades =====
formatar_bytes() {
    local b="${1:-0}"
    awk -v b="$b" 'BEGIN{
        if (b < 1024)       { printf "%d B", b;        exit }
        if (b < 1048576)    { printf "%.1f KiB", b/1024; exit }
        if (b < 1073741824) { printf "%.2f MiB", b/1048576; exit }
        printf "%.2f GiB", b/1073741824
    }'
}

# Retorna o tamanho em bytes de um arquivo, portável entre Linux e macOS.
tamanho_bytes() {
    if [[ "$UNAME_S" == Darwin ]]; then
        stat -f %z -- "$1"
    else
        stat -c %s -- "$1"
    fi
}

# Lê uma variável de forma segura em interativo e não-tty.
#   prompt_var VAR "Texto: " [silencioso]
# - se stdin é tty: usa read normal (-s se silencioso);
# - se não é tty: usa read -t 0 para detectar EOF e retorna 1 (o chamador trata
#   como "erro limpo, exit 2").
prompt_var() {
    local __var="$1" __msg="$2" __silencioso="${3:-}" __r=""
    if [[ -t 0 ]]; then
        if [[ -n "$__silencioso" ]]; then
            IFS= read -r -s -p "$__msg" __r || return 1
            printf '\n'
        else
            IFS= read -r -p "$__msg" __r || return 1
        fi
    else
        # Não é tty (CI/pipe): tenta ler um byte com timeout zero para detectar
        # EOF antes de travar.
        if ! IFS= read -r -t 0 __r 2>/dev/null; then
            return 1
        fi
        IFS= read -r __r || return 1
    fi
    printf -v "$__var" '%s' "$__r"
    return 0
}

validar_webhook() {
    local u="$1"
    [[ "$u" =~ ^https://(discord\.com|discordapp\.com|ptb\.discord\.com|canary\.discord\.com)/(api|api/v[0-9]+)/webhooks/[0-9]+/[A-Za-z0-9_\-]+(\?.*)?$ ]]
}

# Best-effort: substitui URL e token por [REDACTED] antes de exibir.
# Defesa primária é nunca imprimir $WEBHOOK; isto é defesa secundária.
sanitizar() {
    local s="$1"
    [[ -z "${WEBHOOK:-}" ]] && { printf '%s' "$s"; return; }
    local token="${WEBHOOK##*/}"
    token="${token%%\?*}"
    local sem_query="${WEBHOOK%%\?*}"
    local id="${sem_query##*/webhooks/}"
    id="${id%%/*}"
    [[ -n "$token" ]] && s="${s//"$token"/[REDACTED]}"
    [[ -n "$id" && "$id" != "$sem_query" ]] && s="${s//"$id"/[REDACTED]}"
    s="${s//"$WEBHOOK"/[REDACTED]}"
    printf '%s' "$s"
}

# Escapa caracteres de controle para que nomes maliciosos não manipulem o terminal.
# Também torna quebras de linha em nomes de arquivo visíveis como \n.
escapar_saida() {
    local s
    s="$(sanitizar "$1")"
    printf '%q' "$s"
}

escape_form_filename() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    printf '%s' "$s"
}

# Escapa a URL para configuração do curl. Ela é enviada por stdin com
# --config -, em vez de aparecer em argv visível em ferramentas como ps.
escapar_url_curl_config() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    printf '%s' "$s"
}

# Normaliza e valida inteiro. Retorna valor normalizado em stdout.
# Erros vão para stderr e retornam 1.
validar_config_inteiro() {
    local nome="$1" valor="$2" min="$3" max="$4" normalizado
    if [[ ! "$valor" =~ ^[0-9]+$ ]]; then
        printf 'Configuração inválida: %s=%s (esperado inteiro não negativo)\n' \
            "$nome" "$valor" >&2
        return 1
    fi
    # Remove zeros à esquerda
    valor="${valor#"${valor%%[!0]*}"}"
    [[ -z "$valor" ]] && valor="0"
    if (( ${#valor} > 18 )); then
        printf 'Configuração inválida: %s (valor excessivamente grande)\n' \
            "$nome" >&2
        return 1
    fi
    # 10# força base decimal e evita interpretação octal
    normalizado=$(( 10#$valor ))
    if (( normalizado < min || normalizado > max )); then
        printf 'Configuração inválida: %s=%s (fora do intervalo %s..%s)\n' \
            "$nome" "$normalizado" "$min" "$max" >&2
        return 1
    fi
    printf '%s' "$normalizado"
}

# Valida todas as configs antes de prosseguir.
DISCORD_MAX_BYTES="$(validar_config_inteiro DISCORD_MAX_BYTES "$DISCORD_MAX_BYTES" 1 1073741824)" || exit 1
MAX_TENTATIVAS="$(validar_config_inteiro MAX_TENTATIVAS "$MAX_TENTATIVAS" 1 100)" || exit 1
BACKOFF_INICIAL="$(validar_config_inteiro BACKOFF_INICIAL "$BACKOFF_INICIAL" 1 3600)" || exit 1
BACKOFF_MAXIMO="$(validar_config_inteiro BACKOFF_MAXIMO "$BACKOFF_MAXIMO" 1 3600)" || exit 1
CURL_MAX_TIME="$(validar_config_inteiro CURL_MAX_TIME "$CURL_MAX_TIME" 1 86400)" || exit 1
MAX_RETRY_AFTER="$(validar_config_inteiro MAX_RETRY_AFTER "$MAX_RETRY_AFTER" 1 3600)" || exit 1

if (( BACKOFF_MAXIMO < BACKOFF_INICIAL )); then
    printf 'BACKOFF_MAXIMO deve ser >= BACKOFF_INICIAL\n' >&2
    exit 1
fi

# ===== Temporários e limpeza =====
TMPDIR_RUN=""
RESP_BODY=""; RESP_HEADERS=""; RESP_META=""; FIND_ERRS=""; FIND_OUT=""
# O ShellCheck não reconhece chamadas indiretas feitas por `trap EXIT`.
# A supressão fica limitada a esta função de callback.
# shellcheck disable=SC2317
cleanup() {
    unset WEBHOOK 2>/dev/null || true
    if [[ -n "$TMPDIR_RUN" && -d "$TMPDIR_RUN" ]]; then
        rm -rf -- "$TMPDIR_RUN" 2>/dev/null || true
    fi
}
trap cleanup EXIT

criar_tmp() {
    TMPDIR_RUN="$(mktemp -d "${TMPDIR:-/tmp}/discord-uploader.XXXXXX")" || {
        printf 'Falha ao criar diretório temporário.\n' >&2; exit 1; }
    TMPDIR_RUN="$(cd -- "$TMPDIR_RUN" && pwd -P)" || {
        printf 'Falha ao resolver o diretório temporário.\n' >&2; exit 1; }
    RESP_BODY="$TMPDIR_RUN/body"
    RESP_HEADERS="$TMPDIR_RUN/headers"
    RESP_META="$TMPDIR_RUN/meta"
    FIND_ERRS="$TMPDIR_RUN/find_errs"
    FIND_OUT="$TMPDIR_RUN/find_out"
    : > "$RESP_BODY"; : > "$RESP_HEADERS"; : > "$RESP_META"
    : > "$FIND_ERRS"; : > "$FIND_OUT"
}

# ===== Cancelamento (Ctrl+C) =====
INTERROMPIDO=0
# O ShellCheck não reconhece chamadas indiretas feitas por `trap INT`.
# A supressão fica limitada a esta função de callback.
# shellcheck disable=SC2317
on_interrupt() {
    if (( INTERROMPIDO == 0 )); then
        INTERROMPIDO=1
        printf '\n'
        warning "Cancelamento solicitado. Encerrando de forma segura..."
    fi
}
trap on_interrupt INT TERM HUP

# ===== Retry-After (segundos, JSON ou data HTTP) =====
extrair_retry_after() {
    local headers="$1" body="$2" valor=""
    local alvo="" agora="" diff=""

    # 1. Header Retry-After
    valor="$(awk '
        tolower($0) ~ /^retry-after:/ {
            sub(/^[^:]*:[[:space:]]*/, "", $0)
            sub(/\r$/, "", $0)
            print
            exit
        }
    ' "$headers" 2>/dev/null)"

    # 2. JSON retry_after
    if [[ -z "$valor" ]]; then
        if command -v jq >/dev/null 2>&1; then
            valor="$(jq -r '.retry_after // empty' "$body" 2>/dev/null | head -n1)"
        else
            valor="$(sed -n 's/.*"retry_after"[[:space:]]*:[[:space:]]*\([0-9][0-9.]*\).*/\1/p' \
                "$body" 2>/dev/null | head -n1)"
        fi
        if [[ -n "$valor" ]]; then
            awk -v v="$valor" 'BEGIN{ i=int(v); if (v>i) i++; print i }'
            return 0
        fi
        return 1
    fi

    # 3. Valor numérico (segundos, talvez com casas decimais)
    if [[ "$valor" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
        awk -v v="$valor" 'BEGIN{ i=int(v); if (v>i) i++; print i }'
        return 0
    fi

    # 4. Data HTTP (RFC 7231)
    alvo="$(date -d "$valor" +%s 2>/dev/null)" || alvo=""
    [[ -z "$alvo" ]] && return 1
    agora="$(date +%s)"
    diff=$(( alvo - agora ))
    (( diff < 0 )) && diff=0
    printf '%s' "$diff"
    return 0
}

# ===== Upload de um arquivo =====
# Retorna 0 em sucesso; 1 em falha.
# Define UPLOAD_ERRO e UPLOAD_ESTADO_INCERTO (1 = resultado possivelmente aceito).
# Parâmetros opcionais (sem alterar comportamento quando não usados):
#   $2 = content, $3 = thread_id, $4 = thread_name
UPLOAD_ERRO=""
UPLOAD_ESTADO_INCERTO=0
upload_file() {
    local arquivo="$1"
    local tentativa=1
    local espera="$BACKOFF_INICIAL"
    local codigo_curl http retry_after mensagem retryable
    local form_valor
    local ultima_falha=""
    local form_args=()

    UPLOAD_ERRO=""
    UPLOAD_ESTADO_INCERTO=0
    form_valor="files[0]=@\"$(escape_form_filename "$arquivo")\""

    # content / thread_id / thread_name (opcionais; sem eles, igual ao antes).
    [[ -n "${2:-}"     ]] && form_args+=(--form-string "content=$2")
    [[ -n "${3:-}"     ]] && form_args+=(--form-string "thread_id=$3")
    [[ -n "${4:-}"     ]] && form_args+=(--form-string "thread_name=$4")

    while :; do
        # Cancelamento antes de iniciar nova tentativa → estado conhecido.
        if (( INTERROMPIDO )); then
            if [[ -z "$ultima_falha" ]]; then
                UPLOAD_ERRO="Cancelado antes de iniciar o envio."
            else
                UPLOAD_ERRO="Cancelado entre retentativas. Última falha conhecida: $ultima_falha"
            fi
            UPLOAD_ESTADO_INCERTO=0
            return 1
        fi

        codigo_curl=0
        : > "$RESP_BODY"; : > "$RESP_HEADERS"; : > "$RESP_META"

        # Sem --location: não seguimos redirecionamentos. O segredo vai por
        # stdin via --config -, e não em argv visível em ps.
        printf 'url = "%s"\n' "$(escapar_url_curl_config "$WEBHOOK")" | \
            curl \
                --config - \
                --silent \
                --show-error \
                --max-time "$CURL_MAX_TIME" \
                --dump-header "$RESP_HEADERS" \
                --output "$RESP_BODY" \
                --write-out '%{http_code}' \
                --form "$form_valor" \
                "${form_args[@]}" \
                > "$RESP_META" 2> "$TMPDIR_RUN/curl_err" || codigo_curl=$?

        # curl falhou e o usuário interrompeu durante o request: a resposta
        # pode ter sido perdida depois que o Discord recebeu o arquivo.
        if (( codigo_curl != 0 )) && (( INTERROMPIDO )); then
            UPLOAD_ERRO="Envio interrompido durante a requisição; o arquivo pode ter sido recebido. Verifique o canal antes de reenviar."
            UPLOAD_ESTADO_INCERTO=1
            return 1
        fi

        http="$(tr -d '\r\n' < "$RESP_META" 2>/dev/null || true)"
        [[ -z "$http" ]] && http="000"

        # Sucesso: 204 (webhook sem retorno) ou 200 com "id" no JSON.
        if (( codigo_curl == 0 )) && [[ "$http" == "204" ]]; then
            return 0
        fi
        if (( codigo_curl == 0 )) && [[ "$http" == "200" ]]; then
            if command -v jq >/dev/null 2>&1; then
                if jq -e '.id // empty' "$RESP_BODY" >/dev/null 2>&1; then
                    return 0
                fi
            elif grep -q '"id"[[:space:]]*:' "$RESP_BODY" 2>/dev/null; then
                return 0
            fi
        fi

        retryable=0
        mensagem=""

        if [[ "$http" == "429" ]]; then
            # 429 informa rate limit; repetir após Retry-After é esperado.
            retry_after="$(extrair_retry_after "$RESP_HEADERS" "$RESP_BODY" || true)"
            if [[ -z "$retry_after" ]]; then
                retry_after="$espera"
            fi
            if (( retry_after > MAX_RETRY_AFTER )); then
                UPLOAD_ERRO="Rate limit (HTTP 429): servidor pediu ${retry_after}s, acima do teto ${MAX_RETRY_AFTER}s."
                return 1
            fi
            retryable=1
            mensagem="Rate limit (HTTP 429). Aguardando ${retry_after}s."
            espera="$retry_after"
        elif [[ "$http" == "413" ]]; then
            retryable=0
            mensagem="Arquivo maior que o limite aceito pelo Discord (HTTP 413)."
        elif [[ "$http" =~ ^5[0-9][0-9]$ ]]; then
            # Webhook executa um POST não idempotente: 5xx pode ocorrer depois
            # de o servidor processar a mensagem. Não repetir automaticamente.
            UPLOAD_ERRO="Resposta HTTP $http; resultado incerto. Verifique o canal antes de reenviar para evitar duplicação."
            UPLOAD_ESTADO_INCERTO=1
            return 1
        elif (( codigo_curl != 0 )); then
            case "$codigo_curl" in
                5|6|7|35)
                    # Falhas de DNS/conexão/TLS antes do envio HTTP.
                    retryable=1
                    mensagem="Falha de conexão antes do envio (curl $codigo_curl)."
                    ;;
                16|18|28|52|55|56|92)
                    # A requisição pode ter chegado ao servidor; retentar pode duplicar.
                    UPLOAD_ERRO="Erro do curl $codigo_curl; resultado incerto. Verifique o canal antes de reenviar para evitar duplicação."
                    UPLOAD_ESTADO_INCERTO=1
                    return 1
                    ;;
                *)
                    retryable=0
                    mensagem="Erro do curl: $codigo_curl."
                    ;;
            esac
        elif [[ "$http" == "200" ]]; then
            # 200 sem um ID no corpo não confirma o upload, mas também não prova falha.
            UPLOAD_ERRO="Resposta HTTP 200 sem confirmação de upload; resultado incerto. Verifique o canal antes de reenviar."
            UPLOAD_ESTADO_INCERTO=1
            return 1
        else
            mensagem="HTTP $http."
            retryable=0
        fi

        if (( retryable == 0 )); then
            UPLOAD_ERRO="Falha permanente. $mensagem"
            UPLOAD_ESTADO_INCERTO=0
            return 1
        fi

        if (( tentativa >= MAX_TENTATIVAS )); then
            UPLOAD_ERRO="Falha temporária esgotada após ${tentativa} tentativa(s). $mensagem"
            UPLOAD_ESTADO_INCERTO=0
            return 1
        fi

        warning "Tentativa ${tentativa}/${MAX_TENTATIVAS}: $mensagem"
        ultima_falha="$mensagem"

        # Espera interrompível.
        if ! sleep "$espera" 2>/dev/null; then
            # Se não foi interrupção do usuário, tenta um backoff curto.
            if (( INTERROMPIDO == 0 )); then
                sleep 1 2>/dev/null || true
            fi
        fi

        if [[ "$http" != "429" ]]; then
            espera=$(( espera * 2 ))
            (( espera > BACKOFF_MAXIMO )) && espera="$BACKOFF_MAXIMO"
        fi
        tentativa=$(( tentativa + 1 ))
    done
}

# ===== Exportação de relatório =====
exportar_relatorio() {
    local destino="$1"
    if [[ -e "$destino" ]]; then
        printf '%sArquivo já existe. Sobrescrever? [s/N]: %s' "$WHITE" "$RESET"
        local r=""
        IFS= read -r r || r=""
        [[ "$r" =~ ^[sS]$ ]] || { warning "Exportação cancelada."; return 2; }
    fi
    {
        printf 'Discord Uploader — Relatório\n'
        printf 'Data: %s\n' "$(date '+%Y-%m-%d %H:%M:%S')"
        printf 'Pasta: %q\n' "$PASTA"
        printf 'Modo: %s\n' "$MODO"
        printf 'Duração: %ss\n\n' "$DURACAO"
        printf 'Total encontrado: %d\n' "$TOTAL"
        printf 'Processados: %d\n' "$ATUAL"
        printf 'Enviados: %d\n' "$ENVIADOS"
        printf 'Falhas: %d\n' "$FALHAS"
        printf 'Resultados incertos (possível duplicação): %d\n' "$RESULTADOS_INCERTOS"
        printf 'Ignorados (tamanho): %d\n' "$IGNORADOS"
        printf 'Não processados (interrupção): %d\n' "$NAO_PROCESSADOS"
        printf 'Bytes enviados com sucesso: %d (%s)\n\n' \
            "$BYTES_OK" "$(formatar_bytes "$BYTES_OK")"
        if (( FALHAS > 0 )); then
            printf 'Arquivos que falharam:\n'
            local i
            for ((i=0; i<${#LISTA_FALHAS[@]}; i++)); do
                printf ' - %q | %q\n' \
                    "${LISTA_FALHAS[$i]}" "${LISTA_FALHAS_MSG[$i]}"
            done
            printf '\n'
        fi
        if (( RESULTADOS_INCERTOS > 0 )); then
            printf 'Arquivos com resultado incerto (verificar no Discord antes de reenviar):\n'
            local f
            for f in "${LISTA_RESULTADOS_INCERTOS[@]}"; do
                printf ' - %q\n' "$f"
            done
            printf '\n'
        fi
        if (( IGNORADOS > 0 )); then
            printf 'Arquivos ignorados por tamanho:\n'
            local g
            for g in "${LISTA_IGNORADOS[@]}"; do
                printf ' - %q\n' "$g"
            done
        fi
    } > "$destino"
    local status=$?
    if (( status == 0 )); then
        success "Relatório salvo em: $(escapar_saida "$destino")"
        return 0
    fi
    failure "Falha ao salvar relatório."
    return 1
}

# ===== Helpers de CLI =====

mostrar_ajuda() {
    cat <<EOF
Uso: discord-upload [OPÇÕES] [PATH]

Envia arquivos para um canal do Discord via webhook.

Se nenhuma flag for passada e stdin for um terminal, executa o modo
interativo (prompts, idêntico ao comportamento original). Com flags,
executa em modo CLI (sem prompts).

Opções:
  -w, --webhook URL        URL do webhook. AVISO: ficará visível em ps;
                           prefira --webhook-file, --webhook-stdin ou
                           DISCORD_WEBHOOK.
  -W, --webhook-file FILE  lê a 1ª linha de FILE como webhook.
      --webhook-stdin      lê a 1ª linha de stdin como webhook.
  -d, --dir PATH           pasta dos arquivos (ou argumento posicional).
  -f, --file PATH          arquivo específico (repeatable; pode omitir -d).
      --files-from FILE    lista de arquivos, um por linha ("-" = stdin).
  -r, --recursive          inclui subpastas.
  -H, --ignore-hidden      ignora nomes iniciados por ponto.
  -i, --include GLOB       inclui apenas nomes que casarem (repeatable).
  -x, --exclude GLOB       exclui nomes que casarem (repeatable).
      --min-size BYTES     pula arquivos menores que isso.
  -m, --max-bytes N        tamanho máximo por arquivo (default DISCORD_MAX_BYTES).
  -c, --content STRING     mensagem enviada junto do upload.
  -t, --thread-id ID       envia em thread existente.
      --thread-name NAME   cria thread a partir da mensagem.
  -y, --yes                não pede confirmação; assume "sim".
  -n, --dry-run            lista o que faria, não envia; exit 0.
  -o, --report PATH        salva relatório ao final (pula prompt).
      --log FILE           log por linha TSV (ts, status, bytes, arquivo).
  -j, --jobs N             default 1; aceito, mas roda com 1 e avisa.
  -q, --quiet              suprime info/success; mantém warning/failure.
  -v, --verbose            mostra detalhes extras (código HTTP, retry).
      --no-color           desativa cores (força NO_COLOR=1).
  -h, --help               esta ajuda com códigos de saída.
  -V, --version            imprime a versão.

Variáveis de ambiente:
  DISCORD_WEBHOOK          fallback do webhook.
  DISCORD_MAX_BYTES, MAX_TENTATIVAS, BACKOFF_INICIAL, BACKOFF_MAXIMO,
  CURL_MAX_TIME, MAX_RETRY_AFTER

Códigos de saída:
  0   ok
  1   falhas/incertos/incompleto/relatório falhou
  2   erro de uso/config (inclui falta de webhook em não-tty)
  4   webhook inválido (formato ou preflight 401/403/404 com --yes)
  130 interrompido (SIGINT/SIGTERM/SIGHUP)

Segurança: o webhook nunca viaja em argv visível via ps.
EOF
}

# Lê a 1ª linha de um arquivo (ou de stdin, se origem="-").
ler_primeira_linha() {
    local origem="$1" linha=""
    if [[ "$origem" == "-" ]]; then
        IFS= read -r linha <&0 || true
    else
        IFS= read -r linha < "$origem" || true
    fi
    linha="${linha//$'\r'/}"
    printf '%s' "$linha"
}

# Resolve o webhook com a precedência:
# --webhook-file > --webhook-stdin > --webhook > env DISCORD_WEBHOOK > prompt(tty).
resolver_webhook() {
    if [[ -n "$WEBHOOK_FILE" ]]; then
        WEBHOOK="$(ler_primeira_linha "$WEBHOOK_FILE")" || true
    elif (( WEBHOOK_STDIN )); then
        WEBHOOK="$(ler_primeira_linha "-")" || true
    elif [[ -n "$WEBHOOK_URL" ]]; then
        WEBHOOK="$WEBHOOK_URL"
        warning "A URL do webhook passada por --webhook ficará visível em ps."
        warning "Prefira --webhook-file, --webhook-stdin ou DISCORD_WEBHOOK."
    elif [[ -n "${DISCORD_WEBHOOK:-}" ]]; then
        WEBHOOK="$DISCORD_WEBHOOK"
    fi

    if [[ -z "${WEBHOOK:-}" ]]; then
        if [[ -t 0 ]]; then
            prompt_var WEBHOOK "$(printf '%sURL do webhook do Discord: %s' "$WHITE" "$RESET")" silencioso
            WEBHOOK="${WEBHOOK//$'\r'/}"
        else
            failure "Nenhum webhook informado e stdin não é um terminal."
            info "Use --webhook, --webhook-file, --webhook-stdin ou DISCORD_WEBHOOK."
            exit 2
        fi
    fi

    if [[ -z "${WEBHOOK:-}" ]]; then
        failure "Nenhuma URL informada."
        exit 1
    fi
    if ! validar_webhook "$WEBHOOK"; then
        failure "O formato da URL não corresponde a um webhook do Discord."
        info "Esperado: https://discord.com/api/webhooks/<id>/<token>"
        exit 1
    fi
    nota "Formato do webhook aceito. A validade real só é confirmada pelo servidor."
}

# Preflight opcional do webhook (HEAD seguro via --config, sem argv visível).
preflight_webhook() {
    local codigo continuar=""
    codigo="$(printf 'url = "%s"\n' "$(escapar_url_curl_config "$WEBHOOK")" | \
        curl --config - --silent --output /dev/null --write-out '%{http_code}' \
        --max-time 10 --head 2>/dev/null || true)"
    case "$codigo" in
        200|204|405)
            ;;
        401|403|404)
            warning "Preflight do webhook: servidor respondeu HTTP $codigo."
            if [[ -t 0 ]]; then
                prompt_var continuar "Continuar mesmo assim? [s/N]: "
                if [[ ! "$continuar" =~ ^[sS]$ ]]; then
                    failure "Webhook inválido (HTTP $codigo)."
                    exit 4
                fi
            else
                failure "Webhook inválido (HTTP $codigo) e não-interativo: abortando."
                exit 4
            fi
            ;;
        *)
            warning "Preflight do webhook: não foi possível consultar o servidor (curl falhou). Seguindo."
            ;;
    esac
}

# Prompts do modo interativo (ordem idêntica ao original).
# A confirmação "Deseja iniciar?" acontece uma única vez no fluxo principal,
# após a listagem (para mostrar N e tamanho); enviar_tudo NÃO repete o prompt.
coletar_interativo() {
    resolver_webhook

    printf '\n'
    prompt_var PASTA "$(printf '%sCaminho da pasta: %s' "$WHITE" "$RESET")"
    PASTA="${PASTA/#\~/$HOME}"
    if [[ ! -d "$PASTA" ]]; then
        failure "A pasta informada não existe."
        exit 1
    fi
    PASTA="$(cd -- "$PASTA" && pwd -P)" || { failure "Falha ao resolver o caminho."; exit 1; }

    printf '\n'
    info "Quais arquivos deseja enviar?"
    info "  1) Somente arquivos da pasta principal"
    info "  2) Arquivos da pasta e de todas as subpastas"
    printf '\n'
    prompt_var OPCAO "$(printf '%sEscolha [1/2]: %s' "$WHITE" "$RESET")"

    MODO=""
    case "$OPCAO" in
        1) MODO="principal" ;;
        2) MODO="recursivo" ;;
        *) failure "Opção inválida."; exit 1 ;;
    esac

    printf '\n'
    prompt_var OCULTOS "$(printf '%sIgnorar arquivos ocultos (nomes iniciados por ponto)? [s/N]: %s' "$WHITE" "$RESET")"
    IGNORAR_OCULTOS=0
    [[ "$OCULTOS" =~ ^[sS]$ ]] && IGNORAR_OCULTOS=1
}

# Testa se o nome base de um arquivo casa com um glob.
# O glob é intencional: -i/-x aceitam wildcards (não quote $pattern).
# shellcheck disable=SC2053
casar_glob() {
    local arquivo="$1" pattern="$2"
    local base="${arquivo##*/}"
    [[ "$base" == $pattern ]]
}

# Aplica filtros INCLUDE/EXCLUDE/MIN_SIZE sobre ARQUIVOS_ARR.
filtrar_arquivos() {
    local nova=() f pat sz
    local -i bate
    for f in "${ARQUIVOS_ARR[@]}"; do
        bate=0
        # include: se houver, só quem bater algum include
        if (( ${#INCLUDE[@]} > 0 )); then
            bate=0
            for pat in "${INCLUDE[@]}"; do
                if casar_glob "$f" "$pat"; then bate=1; break; fi
            done
            (( bate == 0 )) && continue
        fi
        # exclude: sempre exclui quem bater
        for pat in "${EXCLUDE[@]}"; do
            if casar_glob "$f" "$pat"; then bate=1; break; fi
        done
        (( bate == 1 )) && continue
        # min-size
        if [[ -n "$MIN_SIZE" ]]; then
            sz="$(tamanho_bytes "$f" 2>/dev/null || echo 0)"
            if (( sz < MIN_SIZE )); then continue; fi
        fi
        nova+=("$f")
    done
    ARQUIVOS_ARR=("${nova[@]}")
}

# Lista arquivos em FIND_OUT (NUL-delimited) e preenche ARQUIVOS_ARR,
# respeitando RECURSIVO, IGNORAR_OCULTOS e aplicando filtros.
# -samefile vem antes dos testes -type/-prune (opções globais do find),
# como exigido pelo GNU find.
listar_arquivos() {
    local find_status
    if [[ "$MODO" == "principal" ]]; then
        if (( IGNORAR_OCULTOS )); then
            find "$PASTA" -maxdepth 1 -samefile "$TMPDIR_RUN" -prune -o \
                -type f ! -name '.*' -print0 > "$FIND_OUT" 2> "$FIND_ERRS"
        else
            find "$PASTA" -maxdepth 1 -samefile "$TMPDIR_RUN" -prune -o \
                -type f -print0 > "$FIND_OUT" 2> "$FIND_ERRS"
        fi
    else
        if (( IGNORAR_OCULTOS )); then
            # -mindepth impede que uma pasta-base cujo nome começa com ponto seja podada.
            find "$PASTA" -mindepth 1 -samefile "$TMPDIR_RUN" -prune -o \
                -name '.*' -prune -o -type f -print0 > "$FIND_OUT" 2> "$FIND_ERRS"
        else
            find "$PASTA" -mindepth 1 -samefile "$TMPDIR_RUN" -prune -o \
                -type f -print0 > "$FIND_OUT" 2> "$FIND_ERRS"
        fi
    fi
    find_status=$?

    ARQUIVOS_ARR=()
    while IFS= read -r -d '' f; do
        ARQUIVOS_ARR+=("$f")
    done < "$FIND_OUT"

    FIND_STATUS=$find_status

    filtrar_arquivos
}

# Loop de envio (equivalente ao original; recebe ARQUIVOS_ARR via global).
enviar_tudo() {
    local ARQUIVO NOME TAM continuar

    SECONDS=0

    printf '\n'
    printf '%s%s=== Enviando arquivos ===%s\n\n' "$BLUE" "$BOLD" "$RESET"

    for ARQUIVO in "${ARQUIVOS_ARR[@]}"; do
        if (( INTERROMPIDO )); then
            NAO_PROCESSADOS=$(( TOTAL - ATUAL ))
            break
        fi

        ATUAL=$(( ATUAL + 1 ))
        NOME="${ARQUIVO##*/}"

        if [[ ! -e "$ARQUIVO" ]]; then
            warning "[$ATUAL/$TOTAL] Arquivo sumiu antes do envio: $(escapar_saida "$NOME")"
            FALHAS=$(( FALHAS + 1 ))
            LISTA_FALHAS+=("$ARQUIVO")
            LISTA_FALHAS_MSG+=("Arquivo removido durante a execução.")
            [[ -n "$LOG_FILE" ]] && printf '%s\t%s\t%s\t%s\n' "$(date -Is)" "falha" "0" "$(printf %q "$ARQUIVO")" >&3
            continue
        fi

        if [[ ! -f "$ARQUIVO" || ! -r "$ARQUIVO" ]]; then
            warning "[$ATUAL/$TOTAL] Arquivo não é mais um arquivo legível: $(escapar_saida "$NOME")"
            FALHAS=$(( FALHAS + 1 ))
            LISTA_FALHAS+=("$ARQUIVO")
            LISTA_FALHAS_MSG+=("Arquivo removido, alterado ou sem permissão de leitura.")
            [[ -n "$LOG_FILE" ]] && printf '%s\t%s\t%s\t%s\n' "$(date -Is)" "falha" "0" "$(printf %q "$ARQUIVO")" >&3
            continue
        fi

        TAM="$(tamanho_bytes "$ARQUIVO" 2>/dev/null || echo 0)"

        if (( TAM > DISCORD_MAX_BYTES )); then
            warning "[$ATUAL/$TOTAL] Ignorado ($(formatar_bytes "$TAM") > limite): $(escapar_saida "$NOME")"
            IGNORADOS=$(( IGNORADOS + 1 ))
            LISTA_IGNORADOS+=("$ARQUIVO")
            printf '\n'
            continue
        fi

        printf '%s[%d/%d]%s %s %s(%s)%s\n' \
            "$WHITE" "$ATUAL" "$TOTAL" "$RESET" \
            "$(escapar_saida "$NOME")" "$GRAY" "$(formatar_bytes "$TAM")" "$RESET"

        if (( DRY_RUN )); then
            info "[dry-run] enviaria: $(escapar_saida "$NOME") ($(formatar_bytes "$TAM"))"
            [[ -n "$LOG_FILE" ]] && printf '%s\t%s\t%s\t%s\n' "$(date -Is)" "dry-run" "$TAM" "$(printf %q "$ARQUIVO")" >&3
            continue
        fi

        if (( VERBOSE )); then
            nota "código HTTP será avaliado após o upload de $(escapar_saida "$NOME")"
        fi

        if upload_file "$ARQUIVO" "$CONTENT" "$THREAD_ID" "$THREAD_NAME"; then
            ENVIADOS=$(( ENVIADOS + 1 ))
            BYTES_OK=$(( BYTES_OK + TAM ))
            success "Enviado."
            [[ -n "$LOG_FILE" ]] && printf '%s\t%s\t%s\t%s\n' "$(date -Is)" "ok" "$TAM" "$(printf %q "$ARQUIVO")" >&3
        else
            if (( UPLOAD_ESTADO_INCERTO )); then
                RESULTADOS_INCERTOS=$(( RESULTADOS_INCERTOS + 1 ))
                LISTA_RESULTADOS_INCERTOS+=("$ARQUIVO")
                warning "$(sanitizar "$UPLOAD_ERRO")"
                [[ -n "$LOG_FILE" ]] && printf '%s\t%s\t%s\t%s\n' "$(date -Is)" "incerto" "$TAM" "$(printf %q "$ARQUIVO")" >&3
                if (( INTERROMPIDO )); then
                    NAO_PROCESSADOS=$(( TOTAL - ATUAL ))
                    printf '\n'
                    break
                fi
                printf '\n'
                continue
            fi
            FALHAS=$(( FALHAS + 1 ))
            LISTA_FALHAS+=("$ARQUIVO")
            LISTA_FALHAS_MSG+=("$(sanitizar "$UPLOAD_ERRO")")
            failure "$(sanitizar "$UPLOAD_ERRO")"
            [[ -n "$LOG_FILE" ]] && printf '%s\t%s\t%s\t%s\n' "$(date -Is)" "falha" "$TAM" "$(printf %q "$ARQUIVO")" >&3
        fi
        printf '\n'
    done

    DURACAO="$SECONDS"
}

# ===== Fluxo principal =====
criar_tmp

# ---------- Parser de CLI ----------
SHOW_HELP=0
SHOW_VERSION=0
FROM_CLI=0
if (( $# > 0 )); then
    FROM_CLI=1
fi

WEBHOOK_URL=""
WEBHOOK_FILE=""
WEBHOOK_STDIN=0
PASTA_CLI=""
RECURSIVO=0
IGNORAR_OCULTOS=0
ARQUIVOS_EXTRA=()
INCLUDE=()
EXCLUDE=()
MIN_SIZE=""
MAX_BYTES_CLI=""
CONTENT=""
THREAD_ID=""
THREAD_NAME=""
CONFIRMAR=0
DRY_RUN=0
REPORT_PATH=""
LOG_FILE=""
QUIET=0
VERBOSE=0
JOBS=1

while (( $# )); do
    case "$1" in
        -w|--webhook)
            shift
            [[ $# -gt 0 ]] || { failure "--webhook requer um valor."; exit 2; }
            WEBHOOK_URL="$1" ;;
        -W|--webhook-file)
            shift
            [[ $# -gt 0 ]] || { failure "--webhook-file requer um valor."; exit 2; }
            WEBHOOK_FILE="$1" ;;
        --webhook-stdin)
            WEBHOOK_STDIN=1 ;;
        -d|--dir)
            shift
            [[ $# -gt 0 ]] || { failure "--dir requer um valor."; exit 2; }
            PASTA_CLI="$1" ;;
        -f|--file)
            shift
            [[ $# -gt 0 ]] || { failure "--file requer um valor."; exit 2; }
            ARQUIVOS_EXTRA+=("$1") ;;
        --files-from)
            shift
            [[ $# -gt 0 ]] || { failure "--files-from requer um valor."; exit 2; }
            if [[ "$1" == "-" ]]; then
                while IFS= read -r linha; do
                    [[ -z "$linha" ]] && continue
                    ARQUIVOS_EXTRA+=("$linha")
                done
            else
                while IFS= read -r linha; do
                    [[ -z "$linha" ]] && continue
                    ARQUIVOS_EXTRA+=("$linha")
                done < "$1"
            fi ;;
        -r|--recursive) RECURSIVO=1 ;;
        -H|--ignore-hidden) IGNORAR_OCULTOS=1 ;;
        -i|--include)
            shift
            [[ $# -gt 0 ]] || { failure "--include requer um valor."; exit 2; }
            INCLUDE+=("$1") ;;
        -x|--exclude)
            shift
            [[ $# -gt 0 ]] || { failure "--exclude requer um valor."; exit 2; }
            EXCLUDE+=("$1") ;;
        --min-size)
            shift
            [[ $# -gt 0 ]] || { failure "--min-size requer um valor."; exit 2; }
            MIN_SIZE="$1" ;;
        -m|--max-bytes)
            shift
            [[ $# -gt 0 ]] || { failure "--max-bytes requer um valor."; exit 2; }
            MAX_BYTES_CLI="$1" ;;
        -c|--content)
            shift
            [[ $# -gt 0 ]] || { failure "--content requer um valor."; exit 2; }
            CONTENT="$1" ;;
        -t|--thread-id)
            shift
            [[ $# -gt 0 ]] || { failure "--thread-id requer um valor."; exit 2; }
            THREAD_ID="$1" ;;
        --thread-name)
            shift
            [[ $# -gt 0 ]] || { failure "--thread-name requer um valor."; exit 2; }
            THREAD_NAME="$1" ;;
        -y|--yes) CONFIRMAR=1 ;;
        -n|--dry-run) DRY_RUN=1 ;;
        -o|--report)
            shift
            [[ $# -gt 0 ]] || { failure "--report requer um valor."; exit 2; }
            REPORT_PATH="$1" ;;
        --log)
            shift
            [[ $# -gt 0 ]] || { failure "--log requer um valor."; exit 2; }
            LOG_FILE="$1" ;;
        -j|--jobs)
            shift
            [[ $# -gt 0 ]] || { failure "--jobs requer um valor."; exit 2; }
            JOBS="$1" ;;
        -q|--quiet) QUIET=1 ;;
        -v|--verbose) VERBOSE=1 ;;
        --no-color) NO_COLOR=1 ;;
        -h|--help) SHOW_HELP=1; break ;;
        -V|--version) SHOW_VERSION=1; break ;;
        --)
            shift
            while (( $# )); do
                if [[ -n "$PASTA_CLI" ]]; then
                    failure "Argumento extra: $1"
                    exit 2
                fi
                PASTA_CLI="$1"
                shift
            done
            break ;;
        -*)
            failure "Opção desconhecida: $1"
            exit 2 ;;
        *)
            if [[ -n "$PASTA_CLI" ]]; then
                failure "Argumento extra: $1"
                exit 2
            fi
            PASTA_CLI="$1" ;;
    esac
    shift
done

# --help e --version saem 0 ANTES de criar_tmp / banner / resolver_webhook.
if (( SHOW_HELP )); then
    mostrar_ajuda
    exit 0
fi
if (( SHOW_VERSION )); then
    printf '%s\n' "$VERSION"
    exit 0
fi

# Aplica --no-color (sobrepõe as cores detectadas no topo do script).
if [[ -n "${NO_COLOR:-}" ]]; then
    RESET=''; BLUE=''; WHITE=''; GREEN=''; RED=''; YELLOW=''; GRAY=''; BOLD=''
fi

# Quiet: anula info/success.
if (( QUIET )); then
    info()  { :; }
    success() { :; }
fi

# --jobs > 1: aviso e força 1 (sem concorrência de fato nesta rodada).
if (( JOBS > 1 )); then
    warning "Paralelismo ainda não habilitado; use -j 1. Rodando com -j 1."
    JOBS=1
fi

# max-bytes via CLI sobrescreve DISCORD_MAX_BYTES (validado).
if [[ -n "$MAX_BYTES_CLI" ]]; then
    DISCORD_MAX_BYTES="$(validar_config_inteiro DISCORD_MAX_BYTES "$MAX_BYTES_CLI" 1 1073741824)" || exit 1
fi

# Log por linha: abre fd 3 no início (se solicitado).
if [[ -n "$LOG_FILE" ]]; then
    exec 3>>"$LOG_FILE" || { failure "Não foi possível abrir o log em $LOG_FILE."; exit 1; }
fi

if (( FROM_CLI == 0 )); then
    clear_screen
    banner
fi

# ---------- Dependências e date -d ----------
faltando=()
for cmd in curl find sed awk grep tr stat mktemp sleep head date rm; do
    command -v "$cmd" >/dev/null 2>&1 || faltando+=("$cmd")
done
if (( ${#faltando[@]} > 0 )); then
    failure "Dependências ausentes: ${faltando[*]}"
    info "Instale com: sudo apt install curl findutils coreutils gawk grep sed"
    exit 1
fi

# date -d é requisito GNU (documentado no cabeçalho). Testa antes de prosseguir.
if ! date -d '1970-01-01' +%s >/dev/null 2>&1; then
    failure "O comando 'date -d' (GNU coreutils) não é suportado nesta plataforma."
    info "O script requer date com suporte a -d (GNU coreutils; Debian/Ubuntu)."
    exit 1
fi

# ---------- Webhook ----------
# Dry-run apenas lista arquivos; não precisa resolver/validar webhook.
if (( FROM_CLI )) && (( ! DRY_RUN )); then
    resolver_webhook
    # Preflight apenas em modo CLI (antes do loop de envio).
    preflight_webhook
elif (( FROM_CLI == 0 )); then
    coletar_interativo   # resolve webhook + pasta + modo + ocultos (interativo)
fi

# ---------- Pasta / seleção (modo CLI) ----------
if (( FROM_CLI )); then
    if [[ -z "$PASTA_CLI" && ${#ARQUIVOS_EXTRA[@]} -eq 0 ]]; then
        failure "Faltou a pasta/diretório (-d) ou arquivo (-f)."
        exit 2
    fi
    if [[ -n "$PASTA_CLI" ]]; then
        PASTA="${PASTA_CLI/#\~/$HOME}"
        if [[ ! -d "$PASTA" ]]; then
            failure "A pasta informada não existe."
            exit 1
        fi
        PASTA="$(cd -- "$PASTA" && pwd -P)" || { failure "Falha ao resolver o caminho."; exit 1; }
        MODO="principal"
        if (( RECURSIVO )); then MODO="recursivo"; fi
    fi
fi

# ---------- Listagem e filtros ----------
listar_arquivos

# Acrescenta arquivos passados por -f / --files-from (se houver pasta base).
if (( FROM_CLI )) && (( ${#ARQUIVOS_EXTRA[@]} > 0 )); then
    for extra in "${ARQUIVOS_EXTRA[@]}"; do
        extra="${extra/#\~/$HOME}"
        if [[ -z "$PASTA" ]]; then
            # arquivo absoluto ou relativo sem pasta base
            if [[ -e "$extra" ]]; then
                ARQUIVOS_ARR+=("$extra")
            else
                warning "Arquivo não encontrado: $(escapar_saida "$extra")"
            fi
        else
            # resolve relativo à PASTA
            if [[ "$extra" == /* ]]; then
                alvo="$extra"
            else
                alvo="$PASTA/$extra"
            fi
            if [[ -e "$alvo" ]]; then
                ARQUIVOS_ARR+=("$alvo")
            else
                warning "Arquivo não encontrado: $(escapar_saida "$alvo")"
            fi
        fi
    done
fi

TOTAL=${#ARQUIVOS_ARR[@]}

BUSCA_INCOMPLETA=0
if (( FIND_STATUS != 0 )); then
    BUSCA_INCOMPLETA=1
    warning "find terminou com código $FIND_STATUS — a lista pode estar incompleta."
fi
if [[ -s "$FIND_ERRS" ]]; then
    BUSCA_INCOMPLETA=1
    warning "Avisos durante a busca de arquivos (alguns podem ter sido omitidos):"
    while IFS= read -r linha; do
        nota "  $(escapar_saida "$linha")"
    done < "$FIND_ERRS"
fi

if (( TOTAL == 0 )); then
    warning "Nenhum arquivo encontrado."
    (( BUSCA_INCOMPLETA == 0 )) && exit 0
    exit 1
fi

TOTAL_BYTES=0
GRANDES=0
for f in "${ARQUIVOS_ARR[@]}"; do
    sz="$(tamanho_bytes "$f" 2>/dev/null || echo 0)"
    TOTAL_BYTES=$(( TOTAL_BYTES + sz ))
    (( sz > DISCORD_MAX_BYTES )) && GRANDES=$(( GRANDES + 1 ))
done

# ---------- Resumo ----------
if (( ! QUIET )); then
    printf '\n'
    printf '%s%s=== Resumo do envio ===%s\n' "$BLUE" "$BOLD" "$RESET"
    info "Pasta:               $(escapar_saida "$PASTA")"
    info "Modo:                $MODO"
    info "Arquivos:            $TOTAL"
    info "Tamanho total:       $(formatar_bytes "$TOTAL_BYTES")"
    info "Limite por arquivo:  $(formatar_bytes "$DISCORD_MAX_BYTES") (configurável via DISCORD_MAX_BYTES)"
    if (( GRANDES > 0 )); then
        warning "$GRANDES arquivo(s) excedem o limite e serão ignorados."
    fi
    nota "O limite é apenas preventivo. A resposta real do Discord é a confirmação definitiva."
    printf '\n'
fi

# ---------- Confirmação ----------
# Pergunta apenas em tty e sem --yes. Com --yes (CONFIRMAR=1) não pergunta.
# Aparece uma única vez aqui (não se repete em enviar_tudo).
if [[ -t 0 ]] && (( CONFIRMAR == 0 )); then
    printf '%sDeseja iniciar o envio? [s/N]: %s' "$WHITE" "$RESET"
    prompt_var CONFIRMA "$(printf '%sDeseja iniciar o envio? [s/N]: %s' "$WHITE" "$RESET")"
    if [[ ! "$CONFIRMA" =~ ^[sS]$ ]]; then
        warning "Envio cancelado."
        exit 0
    fi
fi

# ---------- Dry-run ----------
if (( DRY_RUN )); then
    printf '%s%s=== Dry-run ===%s\n' "$BLUE" "$BOLD" "$RESET"
    i=1
    for f in "${ARQUIVOS_ARR[@]}"; do
        sz="$(tamanho_bytes "$f" 2>/dev/null || echo 0)"
        printf '[%d/%d] %s (%s)\n' "$i" "$TOTAL" "$(escapar_saida "${f##*/}")" "$(formatar_bytes "$sz")"
        i=$(( i + 1 ))
    done
    printf '\nTotal: %d arquivo(s)\n' "$TOTAL"
    exit 0
fi

# ---------- Loop de envio ----------
ENVIADOS=0
FALHAS=0
IGNORADOS=0
RESULTADOS_INCERTOS=0
NAO_PROCESSADOS=0
ATUAL=0
BYTES_OK=0
LISTA_FALHAS=()
LISTA_FALHAS_MSG=()
LISTA_IGNORADOS=()
LISTA_RESULTADOS_INCERTOS=()

enviar_tudo

# ===== Resultado final =====
printf '\n'
printf '%s%s=== Resultado final ===%s\n\n' "$BLUE" "$BOLD" "$RESET"

info "Duração:                 ${DURACAO}s"
info "Total encontrado:        $TOTAL"
info "Processados:             $ATUAL"
if (( BUSCA_INCOMPLETA )); then
    warning "Busca incompleta:        alguns arquivos podem ter sido omitidos"
fi
success "Enviados com sucesso:    $ENVIADOS"
if (( BYTES_OK > 0 )); then
    info "Bytes enviados (ok):     $BYTES_OK ($(formatar_bytes "$BYTES_OK"))"
fi
if (( FALHAS > 0 )); then
    failure "Falhas:                  $FALHAS"
else
    success "Falhas:                  0"
fi
if (( RESULTADOS_INCERTOS > 0 )); then
    warning "Resultados incertos:     $RESULTADOS_INCERTOS (verificar antes de reenviar)"
fi
if (( IGNORADOS > 0 )); then
    warning "Ignorados por tamanho:   $IGNORADOS"
fi
if (( NAO_PROCESSADOS > 0 )); then
    warning "Não processados:         $NAO_PROCESSADOS (interrupção)"
fi

if (( FALHAS > 0 )); then
    printf '\n'
    printf '%s%sArquivos que falharam:%s\n' "$RED" "$BOLD" "$RESET"
    idx=0
    for (( idx=0; idx<${#LISTA_FALHAS[@]}; idx++ )); do
        printf '%s - %q%s\n' "$RED" "${LISTA_FALHAS[$idx]}" "$RESET"
        printf '%s   → %s%s\n' "$GRAY" "${LISTA_FALHAS_MSG[$idx]}" "$RESET"
    done
fi

if (( RESULTADOS_INCERTOS > 0 )); then
    printf '\n'
    printf '%s%sArquivos com resultado incerto (verifique no Discord antes de reenviar):%s\n' \
        "$YELLOW" "$BOLD" "$RESET"
    for f in "${LISTA_RESULTADOS_INCERTOS[@]}"; do
        printf '%s - %q%s\n' "$YELLOW" "$f" "$RESET"
    done
fi

if (( IGNORADOS > 0 )); then
    printf '\n'
    printf '%s%sArquivos ignorados (limite de tamanho):%s\n' \
        "$YELLOW" "$BOLD" "$RESET"
    for f in "${LISTA_IGNORADOS[@]}"; do
        printf '%s - %q%s\n' "$YELLOW" "$f" "$RESET"
    done
fi

# ---------- Relatório ----------
RELATORIO_FALHOU=0
if [[ -n "$REPORT_PATH" ]]; then
    DEST="$REPORT_PATH"
    if [[ -e "$DEST" ]]; then
        n=1
        while [[ -e "${DEST}.${n}" ]]; do
            n=$(( n + 1 ))
        done
        DEST="${DEST}.${n}"
        nota "Relatório: arquivo já existe, salvando em $(escapar_saida "$DEST")"
    fi
    if ! exportar_relatorio "$DEST"; then
        RELATORIO_FALHOU=1
    fi
else
    if (( FROM_CLI == 0 )) && [[ -t 0 ]]; then
        printf '\n'
        printf '%sExportar relatório em arquivo? [s/N]: %s' "$WHITE" "$RESET"
        EXP=""
        prompt_var EXP "$(printf '%sExportar relatório em arquivo? [s/N]: %s' "$WHITE" "$RESET")"
        if [[ "$EXP" =~ ^[sS]$ ]]; then
            printf '%sCaminho do arquivo: %s' "$WHITE" "$RESET"
            DEST=""
            prompt_var DEST "$(printf '%sCaminho do arquivo: %s' "$WHITE" "$RESET")"
            if [[ -n "$DEST" ]]; then
                DEST="${DEST/#\~/$HOME}"
                if exportar_relatorio "$DEST"; then
                    :
                else
                    relatorio_status=$?
                    (( relatorio_status == 1 )) && RELATORIO_FALHOU=1
                fi
            fi
        fi
    fi
fi

printf '\n'
printf '%sDiscord Uploader — sessão encerrada.%s\n' "$GRAY" "$RESET"

# Fecha o fd de log, se aberto.
[[ -n "$LOG_FILE" ]] && exec 3>&- || true

# Códigos de saída úteis também para scripts que chamem o uploader.
if (( INTERROMPIDO )); then
    exit 130
elif (( FALHAS > 0 || RESULTADOS_INCERTOS > 0 || NAO_PROCESSADOS > 0 || BUSCA_INCOMPLETA > 0 || RELATORIO_FALHOU > 0 )); then
    exit 1
fi
exit 0
