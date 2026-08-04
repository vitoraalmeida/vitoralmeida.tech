# Deployment em produção

Este runbook descreve a preparação única da VPS e o fluxo recorrente de
deployment. Os comandos de infraestrutura devem ser revisados e executados na
VPS por um administrador; criar os arquivos no repositório não configura o
servidor automaticamente.

## 1. Usuário e diretórios

Crie um usuário exclusivo, sem acesso geral a `sudo`, e a raiz da aplicação:

```bash
sudo useradd --create-home --shell /bin/bash site-deploy
sudo install -d -o site-deploy -g site-deploy \
  /srv/www/vitoralmeida.tech \
  /srv/www/vitoralmeida.tech/incoming \
  /srv/www/vitoralmeida.tech/releases
sudo -u site-deploy touch /srv/www/vitoralmeida.tech/.deploy.lock
```

Instale somente a chave SSH usada pelo environment `production` em
`/home/site-deploy/.ssh/authorized_keys`. O usuário não precisa modificar Nginx
nem executar o gerador na VPS.

## 2. Release inicial

Antes de alterar o Nginx, copie o conteúdo atualmente publicado para uma
release imutável e crie o symlink inicial:

```bash
sudo -u site-deploy cp -a /caminho/do/document-root-atual \
  /srv/www/vitoralmeida.tech/releases/bootstrap
sudo -u site-deploy ln -s releases/bootstrap \
  /srv/www/vitoralmeida.tech/.current-new
sudo -u site-deploy mv -Tf \
  /srv/www/vitoralmeida.tech/.current-new \
  /srv/www/vitoralmeida.tech/current
```

Confirme que `current/index.html`, `current/404.html`,
`current/styles/global.css` e `current/robots.txt` existem antes de continuar.

## 3. Nginx

Altere somente o `root` do server block existente:

```nginx
root /srv/www/vitoralmeida.tech/current;
```

Preserve TLS, domínio, logs, redirects, cache, `try_files` e `error_page`. Em
seguida, valide e recarregue:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

Teste `/`, `/about`, `/portfolio`, `/blog`, um post existente, `/robots.txt`,
`/favicon.png` e uma URL inexistente. A troca futura de `current` não exige
novo reload do Nginx.

## 4. Contrato do pacote

A Action envia para `incoming/`:

```text
site-<commit>.tar.gz
site-<commit>.tar.gz.sha256
deploy-static.sh
```

O pacote contém diretamente o conteúdo de `dist/`, não o diretório `dist` em
si. O deployment é executado como `site-deploy`:

```bash
APP_ROOT=/srv/www/vitoralmeida.tech \
HEALTHCHECK_URL=https://vitoralmeida.tech/ \
bash /srv/www/vitoralmeida.tech/incoming/deploy-static.sh <commit>
```

Variáveis opcionais:

- `APP_ROOT`: raiz da aplicação;
- `HEALTHCHECK_URL`: URL verificada depois da troca;
- `KEEP_RELEASES`: quantidade de releases mantidas, padrão `5`.

## 5. Credenciais do GitHub

No environment `production`, configure:

| Tipo | Nome | Conteúdo |
|---|---|---|
| Secret | `DEPLOY_SSH_KEY` | chave privada exclusiva da Action |
| Secret | `DEPLOY_KNOWN_HOSTS` | linha validada previamente para a VPS |
| Variable | `DEPLOY_HOST` | hostname ou IP da VPS |
| Variable | `DEPLOY_PORT` | porta SSH |
| Variable | `DEPLOY_USER` | `site-deploy` |

Não use senha, usuário `root`, chave compartilhada ou `StrictHostKeyChecking=no`.
Restrinja o environment à branch `main` e, inicialmente, considere exigir
aprovação manual. Proteja `main` exigindo o workflow de CI aprovado.

## 6. Validação e rollback

Para o teste controlado:

1. publique uma release válida;
2. repita o fluxo com `HEALTHCHECK_URL` apontando temporariamente para uma URL
   que falhe;
3. confirme que `current` voltou ao alvo anterior;
4. confirme que o domínio continua respondendo;
5. revise os logs do script e da Action.

Rollback manual para uma release já existente:

```bash
cd /srv/www/vitoralmeida.tech
ln -s releases/<commit-anterior> .current-new
mv -Tf .current-new current
curl --fail --location --silent --show-error https://vitoralmeida.tech/
```

Registre no histórico operacional o commit publicado, o commit restaurado e o
resultado do health check.
