# vitoralmeida.tech

Gerador estático do meu site pessoal. O projeto transforma posts
em Markdown, metadados TOML, templates HTML e arquivos estáticos em um artefato
publicável no diretório `dist/`.

O mesmo contrato é usado no desenvolvimento local, na integração contínua e no
deployment em produção:

```bash
make test
make check
make build
make verify
```

## Requisitos

- Go 1.26 ou versão compatível;
- GNU Make;
- Python 3.11 ou versão compatível, para testar o deployment e, opcionalmente,
  servir o resultado localmente.

## Estrutura do projeto

```text
.
├── .github/workflows/ # integração contínua e deployment
├── cmd/sitegen/       # CLI do gerador
├── content/posts/     # posts e assets próprios de cada post
├── docs/              # documentação operacional
├── internal/sitegen/  # carregamento, validação, renderização e build
├── scripts/           # scripts de deployment
├── static/            # arquivos copiados sem transformação
├── templates/         # templates HTML
├── dist/              # artefato gerado; não versionado
└── Makefile           # contrato de build
```

`dist/` contém somente arquivos gerados. Conteúdo mantido manualmente deve
ficar em `content/`, `templates/` ou `static/`.

## Fluxo de geração

```text
content + templates + static
              ↓
           sitegen
              ↓
            dist/
```

Durante o build, o `sitegen`:

1. valida os diretórios de entrada;
2. carrega e valida os posts;
3. renderiza as páginas em um diretório temporário;
4. copia arquivos estáticos e assets dos posts;
5. valida os arquivos obrigatórios;
6. publica a saída somente depois da geração completa.

Uma falha não deixa o destino parcialmente atualizado.

## Como criar um post

Crie um diretório numerado em `content/posts/`. O nome deve usar letras
minúsculas, números e hífens:

```text
content/posts/03-meu-novo-post/
├── meta.toml
├── post.md
└── assets/
    └── diagram.png
```

O prefixo numérico define a ordenação, com os maiores aparecendo primeiro. A
parte após o primeiro hífen é usada como slug; o exemplo será publicado em
`/blog/meu-novo-post`.

O arquivo `meta.toml` deve conter título, descrição e data no formato
`DD/MM/YYYY`:

```toml
title = "Título do post"
description = "Uma breve descrição"
date = "04/08/2026"
```

O conteúdo deve ser escrito em `post.md`. Arquivos específicos do post ficam
em `assets/` e são publicados em um caminho isolado pelo slug:

```markdown
![Diagrama](/public/posts/meu-novo-post/diagram.png)
```

Nesse exemplo, `assets/diagram.png` será copiado para:

```text
dist/public/posts/meu-novo-post/diagram.png
```

## Como validar o conteúdo

```bash
make check
```

A validação não escreve em `dist/`. Ela verifica:

- diretórios de entrada;
- templates obrigatórios;
- nomes de diretórios e slugs;
- títulos, descrições e datas;
- slugs e destinos duplicados;
- existência dos assets referenciados.

## Como executar os testes

```bash
make test
```

Os testes unitários, de integração e golden tests protegem o contrato de
geração. Quando uma alteração intencional modificar o HTML aprovado, revise a
diferença e atualize os arquivos golden explicitamente:

```bash
go test ./... -update
```

## Como gerar o site

```bash
make build
make verify
```

`make build` recria o artefato canônico em `dist/`. `make verify` confirma a
presença das páginas e arquivos estáticos obrigatórios.

A CLI também pode ser executada diretamente:

```bash
go run ./cmd/sitegen build \
  --content ./content \
  --templates ./templates \
  --static ./static \
  --output ./dist
```

Para validar sem gerar arquivos:

```bash
go run ./cmd/sitegen check \
  --content ./content \
  --templates ./templates \
  --static ./static
```

## Como visualizar localmente

```bash
make build
python3 -m http.server 8080 --directory dist
```

Abra `http://localhost:8080`. O servidor simples do Python não replica regras
do Nginx, como resolução de páginas sem `.html` e a página 404 personalizada.

## Integração contínua

O workflow `.github/workflows/ci.yml` é executado em pull requests e pushes
para `main` e `staging`. Ele configura a versão de Go declarada em `go.mod` e
executa:

```text
make test
    ↓
make check
    ↓
make build
    ↓
make verify
```

Alterações que não satisfazem esse contrato devem ser impedidas de chegar à
branch de produção por meio das regras de proteção de `main`.

## Deployment

Hoje o site é publicado com auxílio do **pneuma**, uma ferramenta que eu criei
para rodar meus projetos: um CLI de deployment para aplicações containerizadas
em um único host, que publica releases OCI imutáveis com Podman rootless, faz
health checks e expõe os apps públicos através do Caddy.

O workflow `.github/workflows/deploy.yml` builda a imagem OCI, faz um smoke test,
publica no GHCR e implanta automaticamente com o pneuma. O artifact do commit é
descoberto pela convenção `image:<commit-sha>`:

```text
push em staging ou main
              ↓
    testes e validação
              ↓
        geração de dist/
              ↓
        build da imagem OCI
              ↓
        push para o GHCR
              ↓
   ssh ... "deploy <app> <branch>"
              ↓
        release imutável + health check
```

O deploy é automático para staging (push na branch `staging`) e protegido por
aprovação manual para production (push na branch `main`, via environment
`production`):

- `vitoralmeida-tech-staging` → `ssh ... "deploy vitoralmeida-tech-staging staging"`
- `vitoralmeida-tech-prod` → `ssh ... "deploy vitoralmeida-tech-prod main"`

A chave CI é restrita (`restrict,command="pneuma ci dispatch"`), então o
workflow envia apenas `deploy <app> <branch>`; o dispatcher resolve o commit da
branch, descobre o artifact `image:<commit>` e implanta. O import inicial de
cada app na VPS é feito uma única vez com `pneuma app import <git-url>
--manifest deploy/<env>/pneuma.toml`.

Mantenho também um fallback de deploy estático (tarball enviado por SSH) para o
caso de voltar a servir o site sem o pneuma. O workflow
`.github/workflows/deploy-static.yml` executa esse método apenas manualmente,
por `workflow_dispatch` com confirmação explícita; a automação em pushes fica a
cargo do pneuma. O fluxo completo do método estático e seu rollback estão em
[`docs/deployment.md`](docs/deployment.md).

## Configuração do environment de produção

O environment `production` usa as seguintes configurações:

| Tipo | Nome | Finalidade |
|---|---|---|
| Secret | `DEPLOY_SSH_KEY` | chave privada exclusiva do usuário de deployment |
| Secret | `DEPLOY_KNOWN_HOSTS` | host key SSH previamente validada |
| Variable | `DEPLOY_HOST` | hostname ou endereço da VPS |
| Variable | `DEPLOY_PORT` | porta SSH |
| Variable | `DEPLOY_USER` | usuário SSH que executa `pneuma app deploy` no deploy |

O usuário de deployment (conectado por `DEPLOY_USER`) é o próprio usuário do
pneuma na VPS: o workflow se conecta a ele e envia `deploy <app> <branch>`,
que o dispatcher restrito (`pneuma ci dispatch`) interpreta. Ele não possui
acesso a `sudo`; a importação inicial de cada app é feita uma única vez como
esse usuário (o comando é descrito em [`docs/deployment.md`](docs/deployment.md)).

## Releases e rollback

As seções abaixo descrevem somente o método de deploy estático (o fallback
manual descrito em [Deployment](#deployment)). Quando o site é publicado por
meio do pneuma, releases e rollback são tratados por ele: `pneuma app
deployments <app>` lista o histórico e `pneuma deployment rollback <app>`
restaura a release anterior. O fluxo completo de rollback via pneuma está em
[`docs/deployment.md`](docs/deployment.md).

A VPS mantém a seguinte estrutura:

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

O script Python de deployment:

1. adquire um lock para impedir execuções simultâneas na VPS;
2. valida o SHA-256, o conteúdo e os caminhos do pacote;
3. rejeita symlinks, tipos especiais e caminhos que escapem da release;
4. extrai e verifica os arquivos obrigatórios em um diretório temporário;
5. cria ou reutiliza `releases/<commit>` de maneira idempotente;
6. troca o symlink `current` atomicamente;
7. executa um health check público e restaura a versão anterior se ele falhar;
8. remove uploads temporários e releases antigas não ativas.

O Nginx usa `/srv/www/vitoralmeida.tech/current` como document root, portanto a
troca de versão não exige reload do serviço.

O rollback automático cobre falhas detectadas pelo health check executado
dentro do script. O workflow faz ainda uma verificação pública final com
`curl`, depois que o script termina; uma falha nessa última etapa sinaliza o job
como reprovado, mas exige inspeção ou rollback manual.

O passo a passo para preparar uma VPS nova está em
[`docs/vps-setup.md`](docs/vps-setup.md). O fluxo completo, seu contrato,
procedimentos de inspeção e rollback estão em
[`docs/deployment.md`](docs/deployment.md).

