#!/bin/sh
# Converte os assets dos posts para WebP e gera variantes responsivas
# (480/800/1280w) quando a imagem original é maior. GIFs animados são
# convertidos com ffmpeg; raster com cwebp. Requer cwebp (libwebp-tools) e
# ffmpeg/ffprobe; sem eles o build apenas avisa e segue (degradação).
#
# Uso: scripts/optimize_assets.sh [diretório-raiz de posts]
set -eu

ASSETS_ROOT="${1:-content/posts}"

if ! command -v cwebp >/dev/null 2>&1 || ! command -v ffmpeg >/dev/null 2>&1 || ! command -v ffprobe >/dev/null 2>&1; then
    echo "warning: cwebp/ffmpeg/ffprobe not found; skipping asset optimization" >&2
    exit 0
fi

image_width() {
    ffprobe -v error -select_streams v:0 -show_entries stream=width -of csv=s=x:p=0 "$1" 2>/dev/null | tr -d '\r\n'
}

# gera a versão base e as variantes de largura de um asset, regenerando quando
# a fonte for mais nova que a variante existente.
add_webp_variants() {
    src="$1"
    stem="$2"
    mode="$3" # cwebp | ffmpeg
    width="$(image_width "$src")"
    [ -n "${width:-}" ] || return 0

    base="$stem.webp"
    if [ ! -f "$base" ] || [ "$src" -nt "$base" ]; then
        if [ "$mode" = cwebp ]; then
            cwebp -quiet -q 80 "$src" -o "$base"
        else
            ffmpeg -y -v error -i "$src" -loop 0 -q:v 60 -f webp "$base"
        fi
    fi

    for size in 480 800 1280; do
        [ "$width" -gt "$size" ] || continue
        target="$stem-${size}w.webp"
        if [ ! -f "$target" ] || [ "$src" -nt "$target" ]; then
            if [ "$mode" = cwebp ]; then
                cwebp -quiet -q 80 -resize "$size" 0 "$src" -o "$target"
            else
                ffmpeg -y -v error -i "$src" -vf "scale=$size:-1" -loop 0 -q:v 60 -f webp "$target"
            fi
        fi
    done
}

for asset in "$ASSETS_ROOT"/*/assets/*; do
    [ -f "$asset" ] || continue
    case "$asset" in
    *.png | *.jpg | *.jpeg)
        add_webp_variants "$asset" "${asset%.*}" cwebp
        ;;
    *.gif)
        add_webp_variants "$asset" "${asset%.gif}" ffmpeg
        ;;
    esac
done
