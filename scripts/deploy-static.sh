#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  echo "usage: $0 <commit-sha>" >&2
  exit 2
}

[[ $# -eq 1 ]] || usage
commit=$1
[[ $commit =~ ^[0-9a-f]{7,64}$ ]] || {
  echo "invalid commit SHA: $commit" >&2
  exit 2
}

app_root=${APP_ROOT:-/srv/www/vitoralmeida.tech}
healthcheck_url=${HEALTHCHECK_URL:-https://vitoralmeida.tech/}
keep_releases=${KEEP_RELEASES:-5}
[[ $keep_releases =~ ^[1-9][0-9]*$ ]] || {
  echo "KEEP_RELEASES must be a positive integer" >&2
  exit 2
}

incoming=$app_root/incoming
releases=$app_root/releases
current=$app_root/current
package_name=site-$commit.tar.gz
checksum_name=$package_name.sha256
package=$incoming/$package_name
checksum=$incoming/$checksum_name
release=$releases/$commit
temporary=
release_created=false

mkdir -p "$incoming" "$releases"
exec 9>"$app_root/.deploy.lock"
flock -x 9

cleanup() {
  local status=$?
  if [[ -n $temporary && -d $temporary ]]; then
    rm -rf -- "$temporary"
  fi
  rm -f -- "$package" "$checksum" "$incoming/deploy-static.sh"
  exit "$status"
}
trap cleanup EXIT

[[ -f $package ]] || { echo "package not found: $package" >&2; exit 1; }
[[ -f $checksum ]] || { echo "checksum not found: $checksum" >&2; exit 1; }

read -r expected_digest expected_name extra < "$checksum"
expected_name=${expected_name#\*}
[[ -z ${extra:-} && $expected_digest =~ ^[0-9a-fA-F]{64}$ && $expected_name == "$package_name" ]] || {
  echo "invalid checksum file: $checksum" >&2
  exit 1
}
actual_digest=$(sha256sum "$package" | awk '{print $1}')
[[ ${actual_digest,,} == ${expected_digest,,} ]] || {
  echo "checksum mismatch for $package" >&2
  exit 1
}

while IFS= read -r archived_path; do
  clean_path=${archived_path#./}
  [[ -n $clean_path ]] || continue
  [[ $clean_path != /* && $clean_path != .. && $clean_path != ../* && $clean_path != */../* && $clean_path != */.. ]] || {
    echo "unsafe archive path: $archived_path" >&2
    exit 1
  }
done < <(tar -tzf "$package")

while IFS= read -r archive_entry; do
  entry_type=${archive_entry:0:1}
  [[ $entry_type == d || $entry_type == - ]] || {
    echo "archive contains unsupported entry type: $entry_type" >&2
    exit 1
  }
done < <(tar -tvzf "$package")

temporary=$(mktemp -d "$releases/.deploy-$commit.XXXXXX")
tar -xzf "$package" --no-same-owner --no-same-permissions -C "$temporary"
find "$temporary" -type d -exec chmod 755 {} +
find "$temporary" -type f -exec chmod 644 {} +

for required in index.html 404.html styles/global.css robots.txt; do
  [[ -f $temporary/$required ]] || {
    echo "required file missing from package: $required" >&2
    exit 1
  }
done

if [[ -e $release ]]; then
  for required in index.html 404.html styles/global.css robots.txt; do
    [[ -f $release/$required ]] || {
      echo "existing release is incomplete: $release" >&2
      exit 1
    }
  done
  rm -rf -- "$temporary"
  temporary=
else
  mv -- "$temporary" "$release"
  temporary=
  release_created=true
fi

previous_target=
if [[ -L $current ]]; then
  previous_target=$(readlink "$current")
fi

new_link=$app_root/.current-$commit
rm -f -- "$new_link"
ln -s "releases/$commit" "$new_link"
mv -Tf -- "$new_link" "$current"

if ! curl --fail --location --silent --show-error --max-time 20 "$healthcheck_url" > /dev/null; then
  echo "health check failed; restoring previous release" >&2
  if [[ -n $previous_target ]]; then
    rollback_link=$app_root/.current-rollback
    rm -f -- "$rollback_link"
    ln -s "$previous_target" "$rollback_link"
    mv -Tf -- "$rollback_link" "$current"
  else
    rm -f -- "$current"
  fi
  if [[ $release_created == true ]]; then
    rm -rf -- "$release"
  fi
  exit 1
fi

current_target=$(readlink "$current")
while IFS= read -r old_release; do
  [[ releases/$old_release == "$current_target" ]] && continue
  rm -rf -- "$releases/$old_release"
done < <(
  find "$releases" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %f\n' \
    | sort -rn \
    | awk -v keep="$keep_releases" 'NR > keep {print $2}'
)

echo "deployed $commit successfully"
