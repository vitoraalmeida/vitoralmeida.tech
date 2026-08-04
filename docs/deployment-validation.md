# Validação do deployment

## 4 de agosto de 2026

O fluxo independente foi validado na VPS de produção antes da ativação do
deployment automático pelo GitHub Actions.

### Estrutura e Nginx

- usuário `site-deploy` criado sem acesso a `sudo`;
- raiz criada em `/srv/www/vitoralmeida.tech`;
- versão anterior preservada em `releases/bootstrap`;
- `current` inicialmente apontado para `releases/bootstrap`;
- document root do Nginx alterado para
  `/srv/www/vitoralmeida.tech/current`;
- configuração anterior preservada em
  `/etc/nginx/sites-available/vitoralmeida.net.before-release-layout`;
- `nginx -t` aprovado e serviço recarregado com sucesso.

As rotas `/`, `/about`, `/portfolio`, `/blog`, um post existente,
`/robots.txt` e `/favicon.png` responderam com HTTP 200. Uma página inexistente
respondeu com HTTP 404 e manteve a página personalizada.

### Release válida

A release `ac581fa7740baee026729bf3838218604a4381e7` foi empacotada a partir de
`dist/`, teve seu SHA-256 validado e foi ativada pelo
`scripts/deploy-static.sh`. O health check e todas as rotas públicas foram
aprovados.

Durante o primeiro ensaio, o health check detectou HTTP 403 porque o diretório
criado por `mktemp` conservava modo `0700`. O rollback automático restaurou
`bootstrap` sem indisponibilidade. O script foi corrigido para publicar
diretórios com modo `0755` e arquivos com modo `0644`; a correção foi validada
localmente e em produção.

### Rollback controlado

Uma release de teste identificada por
`ffffffffffffffffffffffffffffffffffffffff` foi ativada com um health check
deliberadamente configurado para retornar HTTP 404.

O script:

1. detectou a falha;
2. restaurou `current` para
   `releases/ac581fa7740baee026729bf3838218604a4381e7`;
3. removeu a release defeituosa;
4. manteve o Nginx válido e ativo;
5. preservou HTTP 200 na raiz pública.

Com esse teste, o deployment independente, a troca atômica e o rollback
automático foram considerados validados. A ativação automática pelo GitHub
Actions permanece condicionada à configuração do environment `production` e
de suas credenciais.
