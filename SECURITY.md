# Política de Segurança

## 🔒 O Webhook é um Segredo

A URL do webhook do Discord permite que qualquer pessoa que a possua envie
mensagens para o seu canal. Trate-a como uma senha.

- **Não comite** arquivos contendo webhooks reais no repositório.
- **Não cole** webhooks reais em issues, pull requests ou comentários públicos.
- Ao compartilhar logs, substitua a URL por `[REDACTED]`.
- O script Bash nunca imprime o webhook; ele é enviado apenas via
  `--config -` (stdin do `curl`), nunca em argv visível via `ps`.

## 🚨 Reportando Vulnerabilidades

Se você encontrar uma falha de segurança (ex.: vazamento acidental do
webhook, bypass de validação de URL, execução de código não intencional),
**não abra uma issue pública**.

Entre em contato diretamente com o mantenedor pelo GitHub ou por outro canal
privado, descrevendo:

- O componente afetado (CLI Bash, GUI wrapper ou GUI nativa).
- Os passos para reproduzir a vulnerabilidade.
- O impacto potencial e uma sugestão de mitigação, se houver.

O mantenedor responderá o mais breve possível e trabalhará em uma correção
antes de qualquer divulgação pública.

## ✅ Boas Práticas para Usuários

- Use `--webhook-file`, `--webhook-stdin` ou `DISCORD_WEBHOOK` em vez de
  `--webhook URL` sempre que possível.
- Revise as permissões de arquivos que contenham webhooks (`chmod 600`).
- Revogue e gere um novo webhook no Discord se suspeitar que a URL vazou.
