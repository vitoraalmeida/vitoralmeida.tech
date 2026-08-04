# Preparação de uma VPS para deployment

Este guia descreve como preparar uma VPS nova para receber os deployments do
site. Ele considera uma distribuição baseada em Debian, acesso administrativo
com `sudo`, Nginx como servidor HTTP e o domínio `vitoralmeida.tech`.

Ao final, o GitHub Actions deverá conseguir enviar uma release usando o usuário
restrito `site-deploy`, enquanto o Nginx servirá o conteúdo apontado pelo symlink
`/srv/www/vitoralmeida.tech/current`.

## 1. Pré-requisitos

Antes de começar, tenha disponíveis:

- uma VPS com endereço IP público e acesso SSH administrativo;
- os registros DNS de `vitoralmeida.tech` e `www.vitoralmeida.tech`;
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
  /srv/www/vitoralmeida.tech \
  /srv/www/vitoralmeida.tech/incoming \
  /srv/www/vitoralmeida.tech/releases
sudo -u site-deploy touch /srv/www/vitoralmeida.tech/.deploy.lock
```

## 4. Criar e instalar a chave da Action

Gere a chave em uma máquina confiável, não na VPS. A chave privada será usada
pelo GitHub Actions; a VPS deve armazenar somente a chave pública.

```bash
install -d -m 0700 "$HOME/.local/share/vitoralmeida-deploy"
ssh-keygen -t ed25519 \
  -C "github-actions-vitoralmeida.tech" \
  -f "$HOME/.local/share/vitoralmeida-deploy/actions-deploy"
```

Para permitir execução automática, deixe a passphrase vazia. Proteja a chave
privada e nunca a inclua no repositório.

Envie apenas a chave pública para a VPS usando a conta administrativa:

```bash
scp -P 22 \
  "$HOME/.local/share/vitoralmeida-deploy/actions-deploy.pub" \
  usuario-administrador@IP_DA_VPS:/tmp/actions-deploy.pub
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
  -i "$HOME/.local/share/vitoralmeida-deploy/actions-deploy" \
  site-deploy@IP_DA_VPS 'id && test -w /srv/www/vitoralmeida.tech'
```

O resultado deve mostrar o usuário `site-deploy` e terminar sem erro.

## 5. Registrar a identidade SSH da VPS

Na VPS, consulte a impressão digital da chave pública do servidor:

```bash
sudo ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Na máquina local, capture a mesma chave e compare a impressão digital:

```bash
ssh-keyscan -p 22 -t ed25519 IP_DA_VPS > vitoralmeida-known-hosts
ssh-keygen -lf vitoralmeida-known-hosts
```

Só continue se as impressões digitais coincidirem. O conteúdo completo de
`vitoralmeida-known-hosts` será armazenado no secret `DEPLOY_KNOWN_HOSTS`.

## 6. Instalar a primeira release

Na máquina local, gere e valide o site:

```bash
make test
make build
make verify
```

Copie `dist/` para a VPS usando a conta administrativa:

```bash
scp -P 22 -r dist usuario-administrador@IP_DA_VPS:/tmp/vitoralmeida-bootstrap
```

Na VPS, transforme essa cópia na primeira release e crie o symlink `current`:

```bash
sudo install -d -m 0755 -o site-deploy -g site-deploy \
  /srv/www/vitoralmeida.tech/releases/bootstrap
sudo cp -a /tmp/vitoralmeida-bootstrap/. \
  /srv/www/vitoralmeida.tech/releases/bootstrap/
sudo chown -R site-deploy:site-deploy \
  /srv/www/vitoralmeida.tech/releases/bootstrap
sudo -u site-deploy ln -s releases/bootstrap \
  /srv/www/vitoralmeida.tech/.current-new
sudo -u site-deploy mv -Tf \
  /srv/www/vitoralmeida.tech/.current-new \
  /srv/www/vitoralmeida.tech/current
rm -rf /tmp/vitoralmeida-bootstrap
```

Confirme a estrutura antes de configurar o Nginx:

```bash
test -f /srv/www/vitoralmeida.tech/current/index.html
test -f /srv/www/vitoralmeida.tech/current/404.html
test -f /srv/www/vitoralmeida.tech/current/styles/global.css
test -f /srv/www/vitoralmeida.tech/current/robots.txt
readlink -f /srv/www/vitoralmeida.tech/current
```

## 7. Configurar o Nginx

Crie `/etc/nginx/sites-available/vitoralmeida.tech` com a configuração HTTP
inicial:

```nginx
server {
    listen 80;
    listen [::]:80;

    server_name vitoralmeida.tech www.vitoralmeida.tech;
    root /srv/www/vitoralmeida.tech/current;
    index index.html;

    location / {
        if ($request_uri ~ ^/(.+)\.html(?:\?|$)) {
            return 302 /$1;
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
sudo ln -s /etc/nginx/sites-available/vitoralmeida.tech \
  /etc/nginx/sites-enabled/vitoralmeida.tech
sudo nginx -t
sudo systemctl reload nginx
```

Se o site padrão do Nginx conflitar com o domínio, remova apenas o symlink
`/etc/nginx/sites-enabled/default`, depois execute novamente `nginx -t` e o
reload. Não remova o arquivo do novo site.

Antes da troca de DNS, é possível testar a resposta pelo IP:

```bash
curl --fail --header 'Host: vitoralmeida.tech' http://IP_DA_VPS/
```

## 8. Apontar o DNS e habilitar TLS

Crie ou atualize os registros `A` de `vitoralmeida.tech` e
`www.vitoralmeida.tech` para o IP da nova VPS. Se houver IPv6 configurado e
testado, atualize também os registros `AAAA`; caso contrário, remova registros
`AAAA` antigos para não direcionar parte do tráfego ao servidor errado.

Depois que ambos os nomes resolverem para a VPS, emita o certificado:

```bash
sudo certbot --nginx \
  -d vitoralmeida.tech \
  -d www.vitoralmeida.tech \
  --redirect
```

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
| Secret | `DEPLOY_KNOWN_HOSTS` | conteúdo validado de `vitoralmeida-known-hosts` |
| Variable | `DEPLOY_HOST` | IP ou hostname da VPS |
| Variable | `DEPLOY_PORT` | porta SSH, normalmente `22` |
| Variable | `DEPLOY_USER` | `site-deploy` |

Restrinja o environment à branch `main`. Uma aprovação manual pode ser exigida
durante os primeiros deployments.

Depois que os secrets forem copiados para o GitHub, a chave privada local pode
ser removida se não for necessária para manutenção. A chave pública continuará
na VPS e o GitHub conservará a cópia privada criptografada no secret.

## 10. Validar o primeiro deployment

Execute manualmente o workflow **Deploy** ou faça push de um commit aprovado em
`main`. Após o job terminar, verifique na VPS:

```bash
readlink /srv/www/vitoralmeida.tech/current
find /srv/www/vitoralmeida.tech/releases -mindepth 1 -maxdepth 1 -type d
ls -la /srv/www/vitoralmeida.tech/incoming
```

Em seguida, teste as rotas públicas:

```bash
curl --fail --location https://vitoralmeida.tech/
curl --fail --location https://vitoralmeida.tech/about
curl --fail --location https://vitoralmeida.tech/portfolio
curl --fail --location https://vitoralmeida.tech/blog
curl --fail --location https://vitoralmeida.tech/robots.txt
```

Uma URL inexistente deve retornar `404`:

```bash
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  https://vitoralmeida.tech/rota-inexistente)" = "404"
```

O procedimento de rollback e a explicação detalhada da arquitetura estão em
[`deployment.md`](deployment.md).

## 11. Checklist final

- `site-deploy` não possui senha utilizável nem acesso a `sudo`;
- somente a chave pública da Action existe na VPS;
- a chave privada está somente no secret `DEPLOY_SSH_KEY`;
- `DEPLOY_KNOWN_HOSTS` foi comparado com a chave vista diretamente na VPS;
- `/srv/www/vitoralmeida.tech` pertence a `site-deploy`;
- o Nginx aponta para `/srv/www/vitoralmeida.tech/current`;
- HTTP redireciona para HTTPS e a renovação do certificado funciona;
- o workflow publica uma release identificada pelo SHA do commit;
- as rotas principais e o retorno `404` foram validados.

