# Comportamento atual do site

Este documento registra a saída do gerador antes da refatoração para `dist/`.
A referência foi criada em 4 de agosto de 2026 a partir do commit então presente
em `main`.

## Geração

Na raiz do repositório, o site era gerado com:

```bash
go run .
```

O comando precisava ser executado na raiz porque os caminhos de `posts/`,
`templates/` e `src/` eram relativos ao diretório de trabalho. A geração
sobrescrevia arquivos conhecidos, mas não limpava artefatos obsoletos.

Uma cópia temporária da saída original foi guardada durante a refatoração para
comparação local. Ela não faz parte do repositório.

## Estrutura produzida

O diretório `src/` reunia tanto arquivos mantidos manualmente quanto arquivos
gerados:

```text
src/
├── 404.html
├── about.html
├── blog.html
├── blog/
│   ├── por-que-criei-um-site.html
│   └── seguranca-de-servidores.html
├── favicon.png
├── index.html
├── portfolio.html
├── public/
│   ├── posts_images/
│   │   ├── clapping.gif
│   │   ├── nao-seja-hackeado.png
│   │   ├── overengineering.png
│   │   ├── security-obscurity.jpg
│   │   └── sudo-meme.jpg
│   ├── video-exemplo.mp4
│   └── vitor.jpeg
├── robots.txt
└── styles/
    └── global.css
```

## Rotas esperadas

| URL pública | Arquivo correspondente | Observação |
|---|---|---|
| `/` | `index.html` | Página inicial |
| `/about` | `about.html` | A forma sem extensão depende da configuração do Nginx |
| `/portfolio` | `portfolio.html` | A forma sem extensão depende da configuração do Nginx |
| `/blog` | `blog.html` | A forma sem extensão depende da configuração do Nginx |
| `/blog/por-que-criei-um-site` | `blog/por-que-criei-um-site.html` | Post existente; a forma sem extensão depende do Nginx |
| `/blog/seguranca-de-servidores` | `blog/seguranca-de-servidores.html` | Post existente; a forma sem extensão depende do Nginx |
| `/robots.txt` | `robots.txt` | Arquivo estático obrigatório |
| `/favicon.png` | `favicon.png` | Arquivo estático obrigatório |
| `/pagina-inexistente` | `404.html` | O uso da página personalizada depende da configuração do servidor |

Os links internos existentes usavam explicitamente `.html` para `about`,
`portfolio`, `blog` e posts. O gerador não implementava redirecionamentos nem
resolução de URLs sem extensão; isso era responsabilidade do Nginx.

## Página 404

O artefato `404.html` tinha título `Vitor Almeida - Not found` e mostrava a
mensagem “Fim da linha”. Para uma URL inexistente exibir esse arquivo mantendo
status HTTP 404, o servidor precisava ter uma regra `error_page` apropriada.

## Arquivos estáticos obrigatórios

O comportamento visual e funcional dependia, no mínimo, de:

- `styles/global.css`;
- `favicon.png`;
- `robots.txt`;
- `public/vitor.jpeg`;
- `public/video-exemplo.mp4`;
- imagens referenciadas pelos posts em `public/posts_images/`.

## Referência de comparação

Após a refatoração, a saída canônica deve conter as mesmas páginas e os mesmos
arquivos estáticos, salvo mudanças deliberadas de caminhos documentadas no
projeto. Títulos, descrições, conteúdo renderizado, navegação e a página 404
devem ser comparados contra esta lista para detectar regressões.
