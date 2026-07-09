#!/bin/sh
# Hito 2 (tarea 1): descarga y extrae el minirootfs de Alpine.
#
# Uso:   ./scripts/setup-rootfs.sh [directorio-destino]
#        (por defecto: ./rootfs)
#
# Ejecutar en Linux o WSL. IMPORTANTE: el tarball contiene symlinks
# (busybox), que NO sobreviven en discos de Windows montados en WSL
# (/mnt/c, /mnt/d). En WSL usar un destino nativo, p. ej.:
#        ./scripts/setup-rootfs.sh /tmp/rootfs
set -eu

DEST="${1:-./rootfs}"
VERSION="${ALPINE_VERSION:-3.21.0}"
BRANCH="v${VERSION%.*}" # 3.21.0 → v3.21
ARCH="x86_64"
URL="https://dl-cdn.alpinelinux.org/alpine/${BRANCH}/releases/${ARCH}/alpine-minirootfs-${VERSION}-${ARCH}.tar.gz"

if [ -x "${DEST}/bin/sh" ]; then
    echo "rootfs ya existente en ${DEST} — nada que hacer"
    exit 0
fi

# plantilla con XXXXXX al final: busybox mktemp no acepta sufijos
TARBALL="$(mktemp /tmp/alpine-rootfs-XXXXXX)"
trap 'rm -f "$TARBALL"' EXIT

echo "Descargando ${URL} ..."
if command -v wget >/dev/null 2>&1; then
    wget -qO "$TARBALL" "$URL"
else
    curl -fsSL -o "$TARBALL" "$URL"
fi

mkdir -p "$DEST"
echo "Extrayendo en ${DEST} ..."
tar -xzf "$TARBALL" -C "$DEST"

echo "Listo. Contenido del rootfs:"
ls "$DEST"
echo
echo "Probar con:"
echo "  sudo ./minidocker --rootfs ${DEST} run /bin/sh"
