# vitoralmeida.tech

Gerador estático do site pessoal de Vitor Almeida. Posts em Markdown,
metadados TOML, templates HTML e arquivos estáticos são transformados pelo
`sitegen` no artefato canônico `dist/`.

## Requisitos

- Go 1.26 ou versão compatível;
- GNU Make;
- Python 3, opcionalmente, para servir o resultado localmente.

## Estrutura do projeto

```text
.
├── cmd/sitegen/       # CLI do gerador
├── internal/sitegen/  # carregamento, validação, renderização e build
├── content/posts/     # posts e assets próprios de cada post
├── templates/         # templates HTML
├── static/            # arquivos copiados sem transformação
├── dist/              # artefato gerado; não versionado
├── docs/              # documentação operacional
├── scripts/           # scripts de deployment
└── Makefile           # contrato comum de desenvolvimento e CI
```

`dist/` contém somente arquivos gerados. Nenhum conteúdo mantido manualmente
deve ser colocado nele.

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

O prefixo numérico define a ordenação; os maiores aparecem primeiro. A parte
depois do primeiro hífen é o slug, portanto o exemplo será publicado em
`/blog/meu-novo-post.html`.

`meta.toml` deve conter título, descrição e data no formato `DD/MM/YYYY`:

```toml
title = "Título do post"
description = "Uma breve descrição"
date = "04/08/2026"
```

Escreva o conteúdo em `post.md`. Assets pertencem ao próprio post e devem ser
referenciados pelo slug para evitar colisões com outros posts:

```markdown
![Diagrama](/public/posts/meu-novo-post/diagram.png)
```

Durante o build, `assets/diagram.png` será copiado para
`dist/public/posts/meu-novo-post/diagram.png`.

## Como validar o conteúdo

```bash
make check
```

A validação não escreve em `dist/`. Ela verifica diretórios de entrada,
templates obrigatórios, nomes e slugs, metadados, datas, assets referenciados e
destinos duplicados.

## Como executar os testes

```bash
make test
```

Os testes unitários e de integração protegem o contrato completo do gerador.
Quando uma alteração intencional modificar o HTML aprovado nos golden tests,
revise a diferença e atualize os arquivos explicitamente:

```bash
go test ./... -update
```

## Como gerar o site

```bash
make build
make verify
```

`make build` recria `dist/`; `make verify` confirma os arquivos obrigatórios.
O gerador constrói a saída em um diretório temporário e somente substitui seu
destino após renderização, cópia e validação completas.

A CLI também pode ser executada diretamente com caminhos explícitos:

```bash
go run ./cmd/sitegen build \
  --content ./content \
  --templates ./templates \
  --static ./static \
  --output ./dist
```

## Como visualizar localmente

```bash
make build
python3 -m http.server 8080 --directory dist
```

Abra `http://localhost:8080`. O servidor simples do Python espera os caminhos
com `.html`; resolução sem extensão e a página 404 personalizada dependem das
regras do servidor de produção.

## Como funciona o deployment

O contrato de publicação é sempre o mesmo:

```text
content + templates + static → sitegen → dist/ → release imutável → Nginx
```

A integração contínua executa `make test`, `make check`, `make build` e
`make verify`. No deployment, o conteúdo exato de `dist/` é empacotado, tem seu
checksum validado na VPS e vira uma release identificada pelo commit. O symlink
`current` é trocado atomicamente; uma falha no health check restaura a release
anterior. A infraestrutura e as credenciais necessárias estão documentadas em
`docs/deployment.md`.
