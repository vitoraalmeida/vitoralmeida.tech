# vitoralmeida.tech

Gerador de site estático desenvolvido em Go para meu site pessoal. 

O conteúdo dos posts é escrito em Markdown, os metadados ficam em TOML e a
apresentação é composta por templates HTML e CSS. Ao executar o gerador, os
arquivos prontos para publicação são gravados em `src/`.

## Tecnologias

- [Go 1.21](https://go.dev/)
- [`text/template`](https://pkg.go.dev/text/template), da biblioteca padrão
- [Blackfriday](https://github.com/russross/blackfriday), para converter
  Markdown em HTML
- [BurntSushi/toml](https://github.com/BurntSushi/toml), para ler os metadados
  dos posts
- HTML e CSS
- [Giscus](https://giscus.app/), para os comentários dos posts

## Como o gerador funciona

O ponto de entrada é `main.go`. A geração segue este fluxo:

1. Lê os diretórios existentes em `posts/`.
2. Carrega o arquivo `meta.toml` e converte o `post.md` de cada publicação para
   HTML.
3. Ordena os posts pelo nome do diretório, em ordem decrescente.
4. Monta a listagem usada na página inicial e no blog.
5. Combina os templates de página com `templates/base-template.gohtml`.
6. Gera as páginas HTML em `src/` e copia as imagens dos posts para
   `src/public/posts_images/`.

As páginas geradas são:

```text
src/
├── index.html
├── about.html
├── blog.html
├── portfolio.html
├── 404.html
└── blog/
    └── <slug-do-post>.html
```


## Estrutura do projeto

```text
.
├── main.go                  # Orquestra a geração do site
├── ssg/
│   ├── page.go              # Renderiza páginas, posts e copia imagens
│   ├── post.go              # Lê, ordena e converte os posts
│   └── post-listing.go      # Monta a listagem de publicações
├── templates/
│   ├── base-template.gohtml # Estrutura HTML comum a todas as páginas
│   └── *.gohtml             # Conteúdo das páginas e dos posts
├── posts/
│   └── NN-slug/
│       ├── meta.toml
│       ├── post.md
│       └── images/          # Opcional
└── src/                     # Arquivos estáticos e saída do gerador
```

## Executando localmente

### Requisitos

- Go 1.21 ou uma versão compatível

Na raiz do repositório, execute:

```bash
go run .
```

O próprio comando baixa as dependências declaradas em `go.mod`, quando
necessário, e gera o site em `src/`.

Para visualizar o resultado, sirva esse diretório com um servidor HTTP local.
Por exemplo, caso tenha Python 3 instalado:

```bash
python3 -m http.server 8000 --directory src
```

Depois, acesse `http://localhost:8000`.

> O gerador usa caminhos relativos ao diretório atual e, portanto, deve ser
> executado a partir da raiz do repositório.

## Criando um post

Crie um diretório dentro de `posts/` com um prefixo numérico, seguido de hífen e
do slug que será usado na URL:

```text
posts/03-meu-novo-post/
```

O prefixo define a posição da publicação. Como a ordenação é lexicográfica e
decrescente, recomenda-se manter os números com a mesma quantidade de dígitos.
O exemplo acima será publicado em:

```text
/blog/meu-novo-post.html
```

Adicione um `meta.toml`:

```toml
title = "Título do post"
description = "Uma breve descrição"
date = "30/07/2026"
```

Escreva o conteúdo em `post.md`:

```markdown
## Primeiro tópico

Conteúdo do post em Markdown.
```

Se o post tiver imagens, coloque-as no diretório opcional `images/`:

```text
posts/03-meu-novo-post/images/exemplo.png
```

No Markdown, referencie a imagem pelo caminho público:

```markdown
![Descrição da imagem](/public/posts_images/exemplo.png)
```

Todos os arquivos de imagem são copiados para o mesmo diretório de destino.
Por isso, os nomes precisam ser únicos entre todos os posts para evitar que uma
imagem sobrescreva outra durante a geração.

## Alterando páginas e layout

- O conteúdo das páginas fica em `templates/index.gohtml`,
  `templates/about.gohtml`, `templates/blog.gohtml`,
  `templates/portfolio.gohtml` e `templates/404.gohtml`.
- A estrutura comum, os metadados, a navegação, o rodapé e os scripts globais
  ficam em `templates/base-template.gohtml`.
- A apresentação de um post e a listagem de posts ficam, respectivamente, em
  `templates/post.gohtml` e `templates/post-listing.gohtml`.
- Os estilos globais ficam em `src/styles/global.css`.
- Títulos e descrições HTML das páginas são definidos nas chamadas a
  `ssg.GeneratePage` em `main.go`.

Após qualquer alteração nos templates, posts ou metadados, execute novamente
`go run .`.


## Comportamentos a considerar

- O gerador espera que cada item de `posts/` seja um diretório no formato
  `NN-slug`, contendo obrigatoriamente `meta.toml` e `post.md`.
- A data é exibida exatamente como informada no TOML; ela não participa da
  ordenação.
- A geração cria ou sobrescreve os arquivos correspondentes, mas não limpa
  páginas e imagens antigas. Ao renomear ou excluir um post, pode ser necessário
  remover manualmente os artefatos obsoletos de `src/blog/` e
  `src/public/posts_images/`.
- O HTML produzido a partir do Markdown e dos templates é inserido diretamente
  na página base. O conteúdo do repositório deve ser considerado confiável.
