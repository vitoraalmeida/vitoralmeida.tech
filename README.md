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
- Python 3, opcionalmente, para servir o resultado localmente.

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
`/blog/meu-novo-post.html`.

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
para `main`. Ele configura a versão de Go declarada em `go.mod` e executa:

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

O workflow `.github/workflows/deploy.yml` é executado automaticamente em pushes
para `main` e também pode ser iniciado manualmente pelo GitHub Actions.

O processo de publicação é:

```text
checkout
   ↓
testes e validação
   ↓
build de dist/
   ↓
site-<commit>.tar.gz + SHA-256
   ↓
upload por SSH
   ↓
release imutável na VPS
   ↓
troca atômica de current
   ↓
health check público
```

O job de build produz o pacote uma única vez e o armazena como artifact do
workflow. O job de deployment baixa esse artifact e envia o pacote, o checksum
e `scripts/deploy-static.sh` para a VPS. Assim, o conteúdo publicado é o mesmo
conteúdo produzido e validado pelo job de build.

Deployments usam o environment `production` e são serializados pelo grupo de
concorrência `production-vitoralmeida-tech`.

## Configuração do environment de produção

O environment `production` usa as seguintes configurações:

| Tipo | Nome | Finalidade |
|---|---|---|
| Secret | `DEPLOY_SSH_KEY` | chave privada exclusiva do usuário de deployment |
| Secret | `DEPLOY_KNOWN_HOSTS` | host key SSH previamente validada |
| Variable | `DEPLOY_HOST` | hostname ou endereço da VPS |
| Variable | `DEPLOY_PORT` | porta SSH |
| Variable | `DEPLOY_USER` | usuário restrito `site-deploy` |

O usuário `site-deploy` não possui acesso a `sudo` e é responsável somente
pela árvore `/srv/www/vitoralmeida.tech`.

## Releases e rollback

A VPS mantém a seguinte estrutura:

```text
/srv/www/vitoralmeida.tech/
├── incoming/
├── releases/
│   └── <commit>/
├── current -> releases/<commit>
└── .deploy.lock
```

O script de deployment:

1. adquire um lock com `flock`;
2. valida o checksum e os caminhos do pacote;
3. extrai e verifica a release em um diretório temporário;
4. move a release para `releases/<commit>`;
5. troca o symlink `current` atomicamente;
6. executa o health check;
7. restaura a release anterior em caso de falha;
8. remove arquivos temporários e releases antigas.

O Nginx usa `/srv/www/vitoralmeida.tech/current` como document root, portanto a
troca de versão não exige reload do serviço.

Detalhes de preparação da VPS, credenciais e rollback manual estão em
[`docs/deployment.md`](docs/deployment.md). O registro de validação do fluxo está
em [`docs/deployment-validation.md`](docs/deployment-validation.md).
