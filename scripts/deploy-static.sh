#!/usr/bin/env bash

# Ativa verificações estritas para interromper o deployment diante de erros,
# variáveis ausentes ou falhas em pipelines, evitando continuar com estado parcial.
set -Eeuo pipefail # Encerra em erros, preserva traps, rejeita variáveis indefinidas e propaga falhas de pipelines.

# Exibe o contrato de uso da CLI e encerra com código 2 porque argumentos
# inválidos representam erro de invocação, não uma falha durante o deployment.
usage() { # Declara a função usada quando a linha de comando é inválida.
  echo "usage: $0 <commit-sha>" >&2 # Escreve a sintaxe esperada na saída de erro.
  exit 2 # Retorna o código convencional para uso incorreto da linha de comando.
} # Encerra a função de ajuda.

# Valida a identidade da release antes de construir qualquer caminho, impedindo
# que argumentos vazios ou caracteres especiais sejam usados no filesystem.
[[ $# -eq 1 ]] || usage # Exige exatamente um argumento contendo o SHA do commit.
commit=$1 # Armazena o SHA recebido para nomear pacote e release.
[[ $commit =~ ^[0-9a-f]{7,64}$ ]] || { # Aceita somente SHAs hexadecimais minúsculos com comprimento seguro.
  echo "invalid commit SHA: $commit" >&2 # Informa qual identificador foi rejeitado.
  exit 2 # Encerra como erro de invocação antes de alterar o servidor.
} # Encerra o tratamento de SHA inválido.

# Carrega configurações com padrões de produção e valida a retenção para que o
# mesmo script possa ser usado em testes sem aceitar valores perigosos.
app_root=${APP_ROOT:-/srv/www/vitoralmeida.tech} # Define a raiz da aplicação ou usa o valor fornecido pelo ambiente.
healthcheck_url=${HEALTHCHECK_URL:-https://vitoralmeida.tech/} # Define a URL pública usada para aprovar a nova release.
keep_releases=${KEEP_RELEASES:-5} # Define quantas releases recentes devem permanecer armazenadas.
[[ $keep_releases =~ ^[1-9][0-9]*$ ]] || { # Exige uma quantidade inteira positiva para a política de retenção.
  echo "KEEP_RELEASES must be a positive integer" >&2 # Explica o formato aceito para a variável.
  exit 2 # Interrompe antes de qualquer limpeza baseada em valor inválido.
} # Encerra o tratamento de retenção inválida.

# Deriva todos os caminhos a partir de valores já validados para centralizar a
# convenção de nomes e evitar divergências entre validação, extração e limpeza.
incoming=$app_root/incoming # Aponta para a área temporária que recebe uploads.
releases=$app_root/releases # Aponta para o diretório de releases imutáveis.
current=$app_root/current # Aponta para o symlink servido pelo Nginx.
package_name=site-$commit.tar.gz # Monta o nome esperado do pacote para o commit.
checksum_name=$package_name.sha256 # Monta o nome esperado do arquivo de checksum.
package=$incoming/$package_name # Monta o caminho absoluto do pacote recebido.
checksum=$incoming/$checksum_name # Monta o caminho absoluto do checksum recebido.
release=$releases/$commit # Monta o destino imutável da release.
temporary= # Inicializa o caminho temporário para que o trap possa consultá-lo com segurança.
release_created=false # Registra se esta execução criou a release para decidir se deve removê-la no rollback.

# Prepara a árvore operacional e serializa deployments com flock para impedir
# que duas execuções concorrentes troquem o symlink ou removam arquivos entre si.
mkdir -p "$incoming" "$releases" # Garante que as áreas de entrada e releases existam.
exec 9>"$app_root/.deploy.lock" # Abre o arquivo de lock no descritor 9 durante toda a execução.
flock -x 9 # Aguarda e adquire acesso exclusivo ao estado de deployment.

# Remove artefatos transitórios em qualquer saída, porque pacotes enviados e
# extrações incompletas não devem permanecer após sucesso ou falha.
cleanup() { # Declara o handler executado automaticamente ao encerrar o script.
  local status=$? # Preserva o código de saída que acionou o trap.
  if [[ -n $temporary && -d $temporary ]]; then # Verifica se ainda existe uma extração temporária conhecida.
    rm -rf -- "$temporary" # Remove somente o diretório temporário resolvido por esta execução.
  fi # Encerra a limpeza condicional da extração.
  rm -f -- "$package" "$checksum" "$incoming/deploy-static.sh" # Remove pacote, checksum e cópia enviada do script.
  exit "$status" # Retorna o código original para não esconder sucesso ou falha.
} # Encerra a função de limpeza.
trap cleanup EXIT # Registra a limpeza para toda forma normal de encerramento do shell.

# Confirma que os dois arquivos de entrada existem antes de ler ou extrair,
# produzindo erros claros em vez de falhas indiretas de comandos posteriores.
[[ -f $package ]] || { echo "package not found: $package" >&2; exit 1; } # Exige o pacote nomeado pelo commit.
[[ -f $checksum ]] || { echo "checksum not found: $checksum" >&2; exit 1; } # Exige o checksum correspondente ao pacote.

# Valida o formato e o conteúdo do checksum para detectar uploads incompletos,
# arquivos trocados ou pacotes corrompidos antes de processar dados do archive.
read -r expected_digest expected_name extra < "$checksum" # Lê digest, nome e eventual campo excedente da primeira linha.
expected_name=${expected_name#\*} # Remove o marcador de modo binário que sha256sum pode adicionar ao nome.
[[ -z ${extra:-} && $expected_digest =~ ^[0-9a-fA-F]{64}$ && $expected_name == "$package_name" ]] || { # Exige uma entrada única, SHA-256 válido e nome exato.
  echo "invalid checksum file: $checksum" >&2 # Informa que o arquivo de checksum não segue o contrato.
  exit 1 # Interrompe antes de confiar no pacote.
} # Encerra o tratamento de checksum malformado.
actual_digest=$(sha256sum "$package" | awk '{print $1}') # Calcula o SHA-256 real e extrai somente o digest.
[[ ${actual_digest,,} == ${expected_digest,,} ]] || { # Compara os digests sem diferenciar letras maiúsculas de minúsculas.
  echo "checksum mismatch for $package" >&2 # Informa que o pacote não corresponde ao artifact produzido.
  exit 1 # Impede a extração de um pacote incorreto ou corrompido.
} # Encerra o tratamento de divergência de checksum.

# Inspeciona todos os caminhos antes da extração para impedir path traversal e,
# consequentemente, escrita fora do diretório temporário da release.
while IFS= read -r archived_path; do # Percorre cada caminho registrado no pacote sem interpretar espaços ou barras invertidas.
  clean_path=${archived_path#./} # Remove o prefixo relativo comum para simplificar a validação.
  [[ -n $clean_path ]] || continue # Ignora apenas a entrada vazia correspondente à raiz do archive.
  [[ $clean_path != /* && $clean_path != .. && $clean_path != ../* && $clean_path != */../* && $clean_path != */.. ]] || { # Rejeita caminhos absolutos e qualquer componente pai.
    echo "unsafe archive path: $archived_path" >&2 # Identifica o caminho inseguro encontrado.
    exit 1 # Interrompe antes que o caminho possa ser extraído.
  } # Encerra o tratamento de caminho inseguro.
done < <(tar -tzf "$package") # Alimenta o loop com a listagem de nomes do pacote compactado.

# Restringe o pacote a diretórios e arquivos regulares para evitar symlinks,
# hardlinks ou dispositivos que poderiam contornar os limites da extração.
while IFS= read -r archive_entry; do # Percorre a listagem detalhada de entradas do archive.
  entry_type=${archive_entry:0:1} # Obtém o caractere que representa o tipo da entrada no formato de tar.
  [[ $entry_type == d || $entry_type == - ]] || { # Permite somente diretórios e arquivos regulares.
    echo "archive contains unsupported entry type: $entry_type" >&2 # Informa o tipo de entrada proibido.
    exit 1 # Interrompe antes de extrair uma entrada especial.
  } # Encerra o tratamento de tipo não permitido.
done < <(tar -tvzf "$package") # Alimenta o loop com metadados de todas as entradas compactadas.

# Extrai em uma área isolada e normaliza permissões para que o Nginx possa ler a
# release sem herdar proprietário ou modos potencialmente perigosos do pacote.
temporary=$(mktemp -d "$releases/.deploy-$commit.XXXXXX") # Cria um diretório exclusivo dentro do mesmo filesystem das releases.
tar -xzf "$package" --no-same-owner --no-same-permissions -C "$temporary" # Extrai sem preservar proprietário nem permissões do archive.
find "$temporary" -type d -exec chmod 755 {} + # Torna todos os diretórios atravessáveis pelo Nginx.
find "$temporary" -type f -exec chmod 644 {} + # Torna os arquivos legíveis sem conceder escrita a outros usuários.

# Verifica o contrato mínimo do site antes da publicação para impedir que uma
# release incompleta substitua uma versão funcional.
for required in index.html 404.html styles/global.css robots.txt; do # Percorre os artefatos obrigatórios do site.
  [[ -f $temporary/$required ]] || { # Exige que cada caminho seja um arquivo regular.
    echo "required file missing from package: $required" >&2 # Identifica o artefato ausente.
    exit 1 # Interrompe antes de promover a extração.
  } # Encerra o tratamento de arquivo obrigatório ausente.
done # Encerra a validação dos artefatos obrigatórios.

# Torna a operação idempotente: reutiliza releases válidas do mesmo commit ou
# promove a extração por rename, sem sobrescrever estado imutável existente.
if [[ -e $release ]]; then # Verifica se este commit já foi publicado anteriormente.
  for required in index.html 404.html styles/global.css robots.txt; do # Revalida o contrato mínimo da release existente.
    [[ -f $release/$required ]] || { # Exige que a release reutilizada esteja completa.
      echo "existing release is incomplete: $release" >&2 # Informa que o estado existente não pode ser confiado.
      exit 1 # Impede reutilizar ou sobrescrever silenciosamente uma release inválida.
    } # Encerra o tratamento de release existente incompleta.
  done # Encerra a validação da release existente.
  rm -rf -- "$temporary" # Descarta a extração duplicada porque a release já existe e foi validada.
  temporary= # Limpa a referência para evitar uma segunda remoção pelo trap.
else # Executa a promoção apenas quando o commit ainda não possui release.
  mv -- "$temporary" "$release" # Renomeia a extração completa para o destino imutável no mesmo filesystem.
  temporary= # Limpa a referência porque o diretório temporário deixou de existir.
  release_created=true # Registra que um rollback desta execução pode remover a nova release.
fi # Encerra a promoção ou reutilização idempotente.

# Guarda o alvo atual antes da ativação para permitir restauração exata caso o
# health check da nova versão falhe.
previous_target= # Inicializa vazio para também representar uma primeira publicação sem versão anterior.
if [[ -L $current ]]; then # Verifica se há uma release ativa representada por symlink.
  previous_target=$(readlink "$current") # Captura o alvo textual relativo usado na restauração.
fi # Encerra a captura condicional da release anterior.

# Cria um novo symlink fora do nome ativo e usa rename para que a troca de
# versão seja atômica aos olhos do Nginx e dos clientes.
new_link=$app_root/.current-$commit # Define um nome transitório exclusivo para o novo symlink.
rm -f -- "$new_link" # Remove um symlink transitório abandonado por eventual execução anterior.
ln -s "releases/$commit" "$new_link" # Cria o ponteiro relativo para a release escolhida.
mv -Tf -- "$new_link" "$current" # Substitui current atomicamente sem seguir o symlink existente.

# Testa a aplicação pela URL pública depois da troca para validar não apenas os
# arquivos, mas também Nginx, TLS e roteamento; em falha, restaura a versão anterior.
if ! curl --fail --location --silent --show-error --max-time 20 "$healthcheck_url" > /dev/null; then # Exige resposta HTTP bem-sucedida dentro do limite de tempo.
  echo "health check failed; restoring previous release" >&2 # Registra que a ativação será revertida.
  if [[ -n $previous_target ]]; then # Verifica se existe um alvo anterior que possa ser restaurado.
    rollback_link=$app_root/.current-rollback # Define um nome temporário estável para o symlink de rollback.
    rm -f -- "$rollback_link" # Remove eventual symlink de rollback abandonado.
    ln -s "$previous_target" "$rollback_link" # Recria o ponteiro exatamente para a release anterior.
    mv -Tf -- "$rollback_link" "$current" # Restaura current atomicamente.
  else # Trata o caso excepcional de falha na primeira publicação sem release anterior.
    rm -f -- "$current" # Remove o symlink inválido porque não há alvo seguro para restaurar.
  fi # Encerra a restauração condicional.
  if [[ $release_created == true ]]; then # Verifica se a release defeituosa foi criada nesta execução.
    rm -rf -- "$release" # Remove somente a nova release, preservando releases reutilizadas.
  fi # Encerra a remoção condicional da release defeituosa.
  exit 1 # Propaga a falha para o GitHub Actions depois do rollback.
fi # Encerra o health check e seu mecanismo de rollback.

# Aplica a política de retenção após o sucesso para limitar uso de disco, sem
# remover a release atualmente servida mesmo que sua data seja antiga.
current_target=$(readlink "$current") # Obtém o alvo ativo depois do health check aprovado.
while IFS= read -r old_release; do # Percorre releases que excedem a quantidade configurada.
  [[ releases/$old_release == "$current_target" ]] && continue # Preserva incondicionalmente a release ativa.
  rm -rf -- "$releases/$old_release" # Remove uma release antiga pelo nome retornado dentro de releases.
done < <(find "$releases" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %f\n' | sort -rn | awk -v keep="$keep_releases" 'NR > keep {print $2}') # Ordena por modificação e seleciona somente releases além do limite.

# Emite uma confirmação final para tornar o resultado visível nos logs do
# chamador somente depois de ativação, health check e retenção concluídos.
echo "deployed $commit successfully" # Registra o SHA efetivamente publicado.
