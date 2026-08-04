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
nem executar o gerador na VPS. A VPS precisa ter Python 3.11 ou versão
compatível disponível para
executar o script de deployment.

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
deploy_static.py
```

O pacote contém diretamente o conteúdo de `dist/`, não o diretório `dist` em
si. O deployment é executado como `site-deploy`:

```bash
APP_ROOT=/srv/www/vitoralmeida.tech \
HEALTHCHECK_URL=https://vitoralmeida.tech/ \
python3 /srv/www/vitoralmeida.tech/incoming/deploy_static.py <commit>
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

## 7. Racional da arquitetura

O deployment foi estruturado para que o conteúdo validado pela integração
contínua seja exatamente o conteúdo publicado em produção. O GitHub Actions
gera `dist/` uma única vez, cria um pacote imutável e envia esse artefato para a
VPS. O servidor não clona o repositório, não instala a toolchain de Go e não
executa um novo build, evitando diferenças entre os ambientes de CI e produção.

A publicação acontece somente depois que o pacote foi completamente extraído
e validado em um diretório temporário. Enquanto isso, o Nginx continua servindo
a release anterior. A nova versão entra em produção por meio da troca atômica
do symlink `current`, impedindo que usuários recebam uma combinação de arquivos
antigos e novos durante o deployment.

Cada release é identificada pelo SHA do commit:

```text
releases/<commit-sha>/
```

Essa organização permite identificar a versão ativa, preservar releases
anteriores e executar rollback sem reconstruir o site. Depois da troca do
symlink, o script realiza um health check público. Se a validação falhar,
`current` é restaurado para o alvo anterior e a release defeituosa é removida.

O checksum SHA-256 confirma que o pacote recebido corresponde ao pacote
produzido pelo workflow. A validação dos caminhos e tipos de arquivos do
arquivo compactado impede que a extração escreva fora do diretório da release.
O lock adquirido com `flock` impede que dois deployments alterem o estado da
aplicação simultaneamente.

O acesso remoto segue o princípio de privilégio mínimo. O GitHub Actions usa o
usuário exclusivo `site-deploy`, que não possui acesso a `sudo` e pode modificar
somente a árvore da aplicação. Alterações no Nginx e em outras configurações do
sistema continuam sendo responsabilidades administrativas separadas.

As responsabilidades também são divididas entre os componentes:

- o GitHub Actions executa testes, validação, build, empacotamento e transporte;
- `deploy_static.py` valida o pacote, controla o lock, cria a release, ativa a
  versão, executa o health check, realiza rollback e remove arquivos antigos;
- o Nginx serve exclusivamente o conteúdo apontado por `current`.

Manter a lógica operacional em um script versionado torna o processo mais
fácil de testar, executar manualmente e diagnosticar do que distribuir uma
sequência extensa de comandos remotos dentro do workflow. O mesmo commit pode
ser reenviado com segurança, pois releases existentes são validadas e
reutilizadas sem corromper o estado da aplicação.
