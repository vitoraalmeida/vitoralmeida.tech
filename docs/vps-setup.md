# Preparação de uma VPS para deployment

Este guia descreve como preparar uma VPS nova para receber os deployments do
site. Ele considera uma distribuição baseada em Debian, acesso administrativo
com `sudo` e Nginx como servidor HTTP.

Ao final, o GitHub Actions deverá conseguir enviar uma release usando o usuário
restrito `site-deploy`, enquanto o Nginx servirá o conteúdo apontado pelo symlink
`/srv/www/SEU_DOMINIO/current`.

Nos comandos e configurações, substitua os seguintes valores:

| Placeholder | Substitua por |
|---|---|
| `SEU_DOMINIO` | domínio principal do site, sem protocolo, como `example.com` |
| `IP_OU_HOST_DA_VPS` | endereço IP ou hostname usado para acessar a VPS |
| `USUARIO_ADMINISTRATIVO` | usuário da VPS com acesso a `sudo` |

O guia usa `/srv/www/SEU_DOMINIO` para separar os arquivos de cada site. O
valor escolhido precisa ser o mesmo no script de deployment, no workflow e no
Nginx.

## 1. Pré-requisitos

Antes de começar, tenha disponíveis:

- uma VPS com endereço IP público e acesso SSH administrativo;
- controle sobre os registros DNS do domínio que será publicado;
- permissão para configurar o environment `production` no GitHub;
- uma máquina local com `ssh`, `scp`, `ssh-keygen` e `ssh-keyscan`.

Os exemplos abaixo usam a porta SSH `22`. Substitua-a em todos os comandos se a
VPS usar outra porta.

## 2. Instalar os programas necessários

Na VPS, instale e habilite o servidor SSH, o Nginx, o Python e o Certbot:

```bash
sudo apt update
sudo apt install -y openssh-server nginx python3 certbot python3-certbot-nginx
sudo systemctl enable --now ssh nginx
```

O script de deployment usa apenas a biblioteca padrão do Python; não é
necessário instalar pacotes com `pip` nem a toolchain de Go na VPS.

Confirme as instalações:

```bash
python3 --version
nginx -v
sudo systemctl is-active ssh nginx
```

## 3. Criar o usuário restrito

Crie um usuário dedicado ao deployment e bloqueie a autenticação por senha:

```bash
sudo useradd --create-home --shell /bin/bash site-deploy
sudo passwd --lock site-deploy
```

Não adicione `site-deploy` ao grupo `sudo`. Esse usuário precisa modificar
somente a árvore da aplicação; Nginx, pacotes e serviços continuam sob controle
administrativo.

Crie os diretórios operacionais:

```bash
sudo install -d -m 0755 -o site-deploy -g site-deploy \
  /srv/www/SEU_DOMINIO \
  /srv/www/SEU_DOMINIO/incoming \
  /srv/www/SEU_DOMINIO/releases
sudo -u site-deploy touch /srv/www/SEU_DOMINIO/.deploy.lock
```

## 4. Criar e instalar a chave da Action

Gere a chave em uma máquina confiável, não na VPS. A chave privada será usada
pelo GitHub Actions; a VPS deve armazenar somente a chave pública.

```bash
install -d -m 0700 "$HOME/.local/share/site-deploy-key"
ssh-keygen -t ed25519 \
  -C "github-actions-SEU_DOMINIO" \
  -f "$HOME/.local/share/site-deploy-key/actions-deploy"
```

Para permitir execução automática, deixe a passphrase vazia. Proteja a chave
privada e nunca a inclua no repositório.

Envie apenas a chave pública para a VPS usando a conta administrativa:

```bash
scp -P 22 \
  "$HOME/.local/share/site-deploy-key/actions-deploy.pub" \
  USUARIO_ADMINISTRATIVO@IP_OU_HOST_DA_VPS:/tmp/actions-deploy.pub
```

Na VPS, instale-a para `site-deploy`:

```bash
sudo install -d -m 0700 -o site-deploy -g site-deploy /home/site-deploy/.ssh
sudo sh -c 'printf "restrict %s\n" "$(cat /tmp/actions-deploy.pub)" >> /home/site-deploy/.ssh/authorized_keys'
sudo chown site-deploy:site-deploy /home/site-deploy/.ssh/authorized_keys
sudo chmod 0600 /home/site-deploy/.ssh/authorized_keys
rm /tmp/actions-deploy.pub
```

Teste a chave a partir da máquina em que ela foi criada:

```bash
ssh -p 22 \
  -i "$HOME/.local/share/site-deploy-key/actions-deploy" \
  site-deploy@IP_OU_HOST_DA_VPS 'id && test -w /srv/www/SEU_DOMINIO'
```

O resultado deve mostrar o usuário `site-deploy` e terminar sem erro.

## 5. Registrar a identidade SSH da VPS

Na VPS, consulte a impressão digital da chave pública do servidor:

```bash
sudo ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Na máquina local, capture a mesma chave e compare a impressão digital:

```bash
ssh-keyscan -p 22 -t ed25519 IP_OU_HOST_DA_VPS > site-known-hosts
ssh-keygen -lf site-known-hosts
```

Só continue se as impressões digitais coincidirem. O conteúdo completo de
`site-known-hosts` será armazenado no secret `DEPLOY_KNOWN_HOSTS`.

## 6. Instalar a primeira release

Na máquina local, gere e valide o site:

```bash
make test
make build
make verify
```

Copie `dist/` para a VPS usando a conta administrativa:

```bash
scp -P 22 -r dist \
  USUARIO_ADMINISTRATIVO@IP_OU_HOST_DA_VPS:/tmp/site-bootstrap
```

Na VPS, transforme essa cópia na primeira release e crie o symlink `current`:

```bash
sudo install -d -m 0755 -o site-deploy -g site-deploy \
  /srv/www/SEU_DOMINIO/releases/bootstrap
sudo cp -a /tmp/site-bootstrap/. \
  /srv/www/SEU_DOMINIO/releases/bootstrap/
sudo chown -R site-deploy:site-deploy \
  /srv/www/SEU_DOMINIO/releases/bootstrap
sudo -u site-deploy ln -s releases/bootstrap \
  /srv/www/SEU_DOMINIO/.current-new
sudo -u site-deploy mv -Tf \
  /srv/www/SEU_DOMINIO/.current-new \
  /srv/www/SEU_DOMINIO/current
rm -rf /tmp/site-bootstrap
```

Confirme a estrutura antes de configurar o Nginx:

```bash
test -f /srv/www/SEU_DOMINIO/current/index.html
test -f /srv/www/SEU_DOMINIO/current/404.html
test -f /srv/www/SEU_DOMINIO/current/styles/global.css
test -f /srv/www/SEU_DOMINIO/current/robots.txt
readlink -f /srv/www/SEU_DOMINIO/current
```

## 7. Configurar o Nginx

Crie `/etc/nginx/sites-available/SEU_DOMINIO` com a configuração HTTP
inicial:

```nginx
server {
    listen 80;
    listen [::]:80;
    absolute_redirect off;

    server_name SEU_DOMINIO www.SEU_DOMINIO;
    root /srv/www/SEU_DOMINIO/current;
    index index.html;

    location / {
        if ($request_uri ~ ^/(.+)\.html(?:\?|$)) {
            return 301 /$1;
        }

        try_files $uri $uri.html $uri/ =404;
    }

    error_page 404 /404.html;

    location = /404.html {
        internal;
    }
}
```

Habilite o site, valide a sintaxe e recarregue o Nginx:

```bash
sudo ln -s /etc/nginx/sites-available/SEU_DOMINIO \
  /etc/nginx/sites-enabled/SEU_DOMINIO
sudo nginx -t
sudo systemctl reload nginx
```

Se o site padrão do Nginx conflitar com o domínio, remova apenas o symlink
`/etc/nginx/sites-enabled/default`, depois execute novamente `nginx -t` e o
reload. Não remova o arquivo do novo site.

Antes da troca de DNS, é possível testar a resposta pelo IP:

```bash
curl --fail --header 'Host: SEU_DOMINIO' http://IP_OU_HOST_DA_VPS/
```

## 8. Apontar o DNS e habilitar TLS

Crie ou atualize os registros `A` de `SEU_DOMINIO` e, se utilizado,
`www.SEU_DOMINIO` para o IP da nova VPS. Se houver IPv6 configurado e
testado, atualize também os registros `AAAA`; caso contrário, remova registros
`AAAA` antigos para não direcionar parte do tráfego ao servidor errado.

Depois que ambos os nomes resolverem para a VPS, emita o certificado:

```bash
sudo certbot --nginx \
  -d SEU_DOMINIO \
  -d www.SEU_DOMINIO \
  --redirect
```

Se o site não usar o subdomínio `www`, remova `www.SEU_DOMINIO` do
`server_name`, não crie seu registro DNS e omita o segundo argumento `-d`.

Valide a renovação automática:

```bash
sudo certbot renew --dry-run
```

## 9. Configurar o environment do GitHub

No repositório do GitHub, abra **Settings → Environments → production** e
configure:

| Tipo | Nome | Valor |
|---|---|---|
| Secret | `DEPLOY_SSH_KEY` | conteúdo da chave privada `actions-deploy` |
| Secret | `DEPLOY_KNOWN_HOSTS` | conteúdo validado de `site-known-hosts` |
| Variable | `DEPLOY_HOST` | IP ou hostname da VPS |
| Variable | `DEPLOY_PORT` | porta SSH, normalmente `22` |
| Variable | `DEPLOY_USER` | `site-deploy` |

Restrinja o environment à branch `main`. Uma aprovação manual pode ser exigida
durante os primeiros deployments.

Depois que os secrets forem copiados para o GitHub, a chave privada local pode
ser removida se não for necessária para manutenção. A chave pública continuará
na VPS e o GitHub conservará a cópia privada criptografada no secret.

Confirme também que o comando remoto do workflow define `APP_ROOT` como
`/srv/www/SEU_DOMINIO` e `HEALTHCHECK_URL` como `https://SEU_DOMINIO/`. Esses
valores devem corresponder à estrutura e ao domínio configurados nesta VPS.

## 10. Validar o primeiro deployment

Execute manualmente o workflow **Deploy** ou faça push de um commit aprovado em
`main`. Após o job terminar, verifique na VPS:

```bash
readlink /srv/www/SEU_DOMINIO/current
find /srv/www/SEU_DOMINIO/releases -mindepth 1 -maxdepth 1 -type d
ls -la /srv/www/SEU_DOMINIO/incoming
```

Em seguida, teste as rotas públicas:

```bash
curl --fail --location https://SEU_DOMINIO/
curl --fail --location https://SEU_DOMINIO/about
curl --fail --location https://SEU_DOMINIO/portfolio
curl --fail --location https://SEU_DOMINIO/blog
curl --fail --location https://SEU_DOMINIO/robots.txt
curl --fail --location https://SEU_DOMINIO/sitemap.xml
curl --fail --location https://SEU_DOMINIO/feed.xml
curl --fail --location https://SEU_DOMINIO/og-image.png
```

Uma URL inexistente deve retornar `404`:

```bash
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  https://SEU_DOMINIO/rota-inexistente)" = "404"
```

O procedimento de rollback e a explicação detalhada da arquitetura estão em
[`deployment.md`](deployment.md).

## 11. Checklist final

- `site-deploy` não possui senha utilizável nem acesso a `sudo`;
- somente a chave pública da Action existe na VPS;
- a chave privada está somente no secret `DEPLOY_SSH_KEY`;
- `DEPLOY_KNOWN_HOSTS` foi comparado com a chave vista diretamente na VPS;
- `/srv/www/SEU_DOMINIO` pertence a `site-deploy`;
- o Nginx aponta para `/srv/www/SEU_DOMINIO/current`;
- HTTP redireciona para HTTPS e a renovação do certificado funciona;
- o workflow publica uma release identificada pelo SHA do commit;
- as rotas principais e o retorno `404` foram validados.
