# Deployment em produção

Este documento descreve o fluxo de deployment implementado em
`.github/workflows/deploy.yml` e `scripts/deploy_static.py`. A preparação de uma
VPS nova, incluindo usuário, chave SSH, Nginx, DNS e TLS, está documentada em
[`vps-setup.md`](vps-setup.md).

## 1. Visão geral

O site é gerado uma única vez no GitHub Actions. O conteúdo validado pelo job
de build é empacotado, transferido para a VPS e publicado sem um segundo build:

```text
push em main ou execução manual
              │
              ▼
       testes e validação
              │
              ▼
        geração de dist/
              │
              ▼
 pacote + checksum SHA-256
              │
              ▼
   artifact do GitHub Actions
              │
              ▼
      transferência por SSH
              │
              ▼
 validação e extração temporária
              │
              ▼
     release imutável na VPS
              │
              ▼
   troca atômica de current
              │
              ▼
 health check e verificação pública
```

O workflow é iniciado:

- automaticamente, a cada push na branch `main`;
- manualmente, por `workflow_dispatch` no GitHub Actions.

O grupo de concorrência `production-vitoralmeida-tech` faz os deployments
rodarem um de cada vez no GitHub Actions. Se uma execução já estiver publicando,
a próxima aguarda; `cancel-in-progress: false` garante que o deployment em
andamento não seja interrompido pelo novo.

## 2. Job de build

O job `build` executa, nesta ordem:

1. checkout do commit;
2. instalação da versão de Go definida em `go.mod`;
3. `make test`, com testes Go e testes do script Python;
4. `make check`, com a validação do conteúdo do site;
5. `make build`, que gera o site estático em `dist/`;
6. `make verify`, que verifica os arquivos essenciais do build;
7. criação do pacote e de seu checksum SHA-256.

Os arquivos produzidos são:

```text
site-<commit-sha>.tar.gz
site-<commit-sha>.tar.gz.sha256
```

O pacote contém diretamente o conteúdo de `dist/`, não o diretório `dist`.
Isso faz com que `index.html`, `404.html` e os demais arquivos fiquem na raiz
da release quando ela for extraída.

Os dois arquivos são armazenados em um artifact chamado
`site-<commit-sha>`, retido pelo GitHub Actions por 14 dias. Esse artifact é o
limite entre os jobs: o job de deployment baixa exatamente o resultado
produzido e aprovado pelo job de build.

## 3. Job de deployment

O job `deploy` só começa depois do sucesso de `build` e usa o environment
`production`. Ele executa:

1. checkout, para obter `scripts/deploy_static.py`;
2. download do artifact gerado por `build`;
3. criação temporária de `~/.ssh/deploy_key` e `~/.ssh/known_hosts` no runner;
4. envio do pacote, checksum e script Python para `incoming/` com `scp`;
5. execução remota do script como `site-deploy`;
6. um segundo teste público com `curl`, com até três tentativas.

As conexões usam `BatchMode=yes` e `StrictHostKeyChecking=yes`. Portanto, o job
não aceita senha interativa nem uma identidade de servidor diferente da que foi
registrada em `DEPLOY_KNOWN_HOSTS`.

O comando remoto atual equivale a:

```bash
APP_ROOT=/srv/www/vitoralmeida.tech \
HEALTHCHECK_URL=https://vitoralmeida.tech/ \
python3 /srv/www/vitoralmeida.tech/incoming/deploy_static.py <commit-sha>
```

O workflow também confirma que `python3` existe antes de executar o script.

## 4. Credenciais e configuração

O environment `production` contém:

| Tipo | Nome | Conteúdo |
|---|---|---|
| Secret | `DEPLOY_SSH_KEY` | chave privada exclusiva da Action |
| Secret | `DEPLOY_KNOWN_HOSTS` | chave pública SSH da VPS previamente validada |
| Variable | `DEPLOY_HOST` | hostname ou IP da VPS |
| Variable | `DEPLOY_PORT` | porta SSH |
| Variable | `DEPLOY_USER` | usuário restrito `site-deploy` |

O usuário `site-deploy` não possui acesso a `sudo` e modifica somente
`/srv/www/vitoralmeida.tech`. Alterações de Nginx, pacotes e serviços exigem uma
conta administrativa separada.

O script aceita três variáveis de ambiente:

| Variável | Padrão | Finalidade |
|---|---|---|
| `APP_ROOT` | `/srv/www/vitoralmeida.tech` | raiz operacional do site |
| `HEALTHCHECK_URL` | `https://vitoralmeida.tech/` | URL testada após a ativação |
| `KEEP_RELEASES` | `5` | quantidade de releases recentes considerada na limpeza |

`KEEP_RELEASES` precisa ser um inteiro positivo. O workflow define
explicitamente `APP_ROOT` e `HEALTHCHECK_URL`; ele usa o padrão de
`KEEP_RELEASES`.

## 5. Estrutura na VPS

Depois de um deployment, a árvore possui esta forma:

```text
/srv/www/vitoralmeida.tech/
├── incoming/
├── releases/
│   ├── bootstrap/
│   ├── <commit-anterior>/
│   └── <commit-ativo>/
├── current -> releases/<commit-ativo>
└── .deploy.lock
```

Cada item tem uma responsabilidade:

- `incoming/`: recebe o pacote, o checksum e uma cópia do script;
- `releases/<commit-sha>/`: guarda uma versão identificável e imutável do site;
- `bootstrap/`: preserva o conteúdo que existia antes do primeiro deployment;
- `current`: aponta para a release que o Nginx deve servir;
- `.deploy.lock`: impede que duas instâncias do script alterem a VPS ao mesmo
  tempo, inclusive quando uma delas é iniciada manualmente.

O Nginx usa `/srv/www/vitoralmeida.tech/current` como document root. Como
`current` é um symlink, a troca de release não exige alterar nem recarregar o
Nginx.

Arquivos temporários de extração podem aparecer em `releases/` com o prefixo
`.deploy-<commit>.` enquanto o script está em execução. Eles são removidos ao
final, inclusive quando ocorre uma falha esperada.

## 6. Validação de uma release

O script recebe o SHA do commit como único argumento. Ele aceita de 7 a 64
caracteres hexadecimais minúsculos, impedindo que o argumento seja usado para
formar caminhos arbitrários.

Antes da extração, o script valida:

- a presença do pacote e do checksum esperados em `incoming/`;
- o formato do arquivo de checksum;
- o nome do pacote registrado no checksum;
- a correspondência do SHA-256 com os bytes recebidos;
- todos os caminhos armazenados no arquivo compactado;
- a ausência de caminhos absolutos, `..`, symlinks e tipos especiais;
- a ausência de destinos duplicados no pacote.

A extração acontece primeiro em um diretório temporário. Diretórios recebem
permissão `0755` e arquivos recebem `0644`, permitindo a leitura pelo Nginx sem
conceder escrita pública.

Uma release só pode ser promovida se contiver:

```text
index.html
404.html
styles/global.css
robots.txt
```

O `make verify` examina mais arquivos durante o build, enquanto essa lista
menor representa o contrato mínimo que o servidor exige antes da ativação.

## 7. Promoção e troca atômica

Depois da validação, o diretório temporário é renomeado para
`releases/<commit-sha>`. Um rename no mesmo filesystem evita que uma release
parcial apareça no destino final.

Se uma release válida com o mesmo SHA já existir, ela é reutilizada. Isso torna
segura a repetição de um deployment do mesmo commit, sem sobrescrever uma
release pronta.

Para ativar a versão, o script cria primeiro um symlink temporário, como
`.current-<commit>`, e então o substitui por `current` com `os.replace`. O Nginx
observa o link antigo ou o novo, nunca um intervalo no qual `current` não
existe. O nome `.current-new` usado na preparação inicial segue o mesmo
princípio, embora não faça parte das execuções recorrentes do script.

## 8. Health check, rollback e limpeza

Depois da troca de `current`, o script acessa `HEALTHCHECK_URL` com timeout de
20 segundos e exige uma resposta HTTP entre `200` e `299`.

Se o health check falhar:

1. `current` volta atomicamente ao alvo anterior;
2. a release reprovada é removida se tiver sido criada nessa execução;
3. o script termina com código diferente de zero;
4. o job do GitHub Actions falha.

Se ainda não existia uma release anterior, o rollback remove `current`. Uma
release preexistente e reutilizada não é apagada em caso de falha.

Após um deployment bem-sucedido, releases são ordenadas pela data de
modificação. O script remove as releases não ativas que ficarem além do limite
de `KEEP_RELEASES`; a release ativa nunca é removida. Dependendo da posição da
release ativa nessa ordenação, a árvore pode conservar uma release adicional.

O pacote, o checksum e a cópia enviada de `deploy_static.py` são removidos de
`incoming/` no final, tanto no sucesso quanto nas falhas tratadas pelo script.

## 9. Inspeção operacional

Para identificar a release ativa:

```bash
readlink /srv/www/vitoralmeida.tech/current
readlink -f /srv/www/vitoralmeida.tech/current
```

Para listar releases da mais recente para a mais antiga:

```bash
find /srv/www/vitoralmeida.tech/releases \
  -mindepth 1 -maxdepth 1 -type d \
  -printf '%T@ %f\n' | sort -nr
```

Para confirmar que não sobraram uploads depois do deployment:

```bash
ls -la /srv/www/vitoralmeida.tech/incoming
```

Teste pelo menos `/`, `/about`, `/portfolio`, `/blog`, um post existente,
`/robots.txt`, `/favicon.png`, `/sitemap.xml`, `/feed.xml`, `/og-image.png`
e uma URL inexistente. A URL inexistente deve retornar `404`.

## 10. Rollback manual

Use rollback manual somente quando for necessário restaurar uma release que já
existe em `releases/`. Execute como `site-deploy`:

```bash
cd /srv/www/vitoralmeida.tech
test -f releases/<commit-anterior>/index.html
ln -s releases/<commit-anterior> .current-rollback-manual
mv -Tf .current-rollback-manual current
curl --fail --location --silent --show-error https://vitoralmeida.tech/
```

A troca também é atômica e não exige reload do Nginx. Se o `curl` falhar,
investigue a release, o Nginx, o DNS e o TLS; o comando manual não possui o
rollback automático oferecido pelo script.

Registre o SHA que estava publicado, o SHA restaurado, o motivo e o resultado
da verificação pública.

## 11. Garantias e limites

O desenho atual oferece:

- correspondência entre o build validado no CI e o conteúdo enviado à VPS;
- verificação de integridade por SHA-256;
- proteção contra escrita fora da release durante a extração;
- publicação sem estado parcialmente extraído;
- troca atômica da versão servida;
- rollback automático quando o health check interno falha;
- execução sequencial dos deployments no GitHub Actions e bloqueio de scripts
  simultâneos na VPS;
- acesso remoto com privilégio mínimo;
- repetição segura do mesmo commit.

O health check confirma disponibilidade HTTP, Nginx, DNS e TLS, mas não compara
todo o conteúdo publicado com o artifact. O rollback automático também cobre
somente falhas detectadas durante a execução do script; a verificação final do
workflow ocorre depois que o script já concluiu.

Mudanças de domínio, raiz da aplicação ou VPS precisam manter sincronizados o
workflow, as variáveis do environment, o Nginx e os parâmetros passados ao
script.
