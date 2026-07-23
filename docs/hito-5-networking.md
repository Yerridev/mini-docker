# Hito 5 — Networking (`CLONE_NEWNET` + loopback)

Este hito aísla la pila de red del contenedor usando el namespace `NET` de
Linux (`CLONE_NEWNET`). Por defecto se levanta la interfaz `lo` (loopback)
dentro del namespace, de modo que el contenedor puede hacer `ping 127.0.0.1`
sin acceder a la red del host.

> **Nota:** el objetivo mínimo de la rúbrica es loopback aislado. La
> conectividad host↔contenedor mediante un par `veth` queda como extensión
> bonus.

---

## Qué aísla `CLONE_NEWNET`

Cuando un proceso nace con el flag `CLONE_NEWNET`, el kernel le crea un
namespace de red propio. Eso significa que:

- Tiene sus propias interfaces virtuales (`lo`, más cualquier `veth` que se
  le mueva).
- Tiene su propia tabla de rutas.
- No ve ni puede usar las interfaces físicas del host.
- Los sockets y conexiones del host son invisibles.

Esto es distinto de un firewall: es separación a nivel de kernel, no solo
bloqueo de paquetes.

---

## Cómo se levanta `lo`

El proceso init del contenedor (el hijo con `MINIDOCKER_INIT=1`) llama a
`netns.Setup(cfg.NetMode)` después de `mountProc`. En modo `loopback` (default)
se levanta la interfaz `lo` con ioctl directo:

1. Se abre un socket `AF_INET`/`SOCK_DGRAM`.
2. `SIOCGIFFLAGS` lee los flags actuales de `lo`.
3. Se activan `IFF_UP` e `IFF_RUNNING`.
4. `SIOCSIFFLAGS` escribe los flags de vuelta.

Si el ioctl no está disponible, hay un fallback a `ip link set lo up`.

---

## Flags de red

```bash
# Default: loopback aislado.
sudo ./minidocker --rootfs rootfs run /bin/sh

# Sin red: no se levanta lo.
sudo ./minidocker --rootfs rootfs --net none run /bin/sh

# Bonus (no implementado): veth + bridge/NAT.
sudo ./minidocker --rootfs rootfs --net veth run /bin/sh
```

---

## Archivos involucrados

| Archivo | Cambio |
|---|---|
| `internal/namespace/namespace_linux.go` | Añade `flagNEWNET` a `Cloneflags`. |
| `internal/netns/netns.go` | Tipos `Mode`, `ParseMode`, `String`. |
| `internal/netns/netns_linux.go` | `Setup()` y `setLinkUp()` con ioctl. |
| `internal/netns/netns_other.go` | Stub no-op para Windows/macOS. |
| `internal/config/config.go` | Campos `Hostname` y `NetMode`. |
| `cmd/minidocker/main.go` | Flags `--net` y `--hostname`. |
| `internal/container/container.go` | Reenvía `--net` y `--hostname` al hijo. |
| `internal/container/setup_linux.go` | Usa `cfg.Hostname` y llama `netns.Setup`. |

---

## Verificación

```bash
# Compilar
go build -o minidocker ./cmd/minidocker

# Loopback:
sudo ./minidocker --rootfs rootfs run /bin/sh
# adentro:
ip addr 2>/dev/null || cat /proc/net/dev
ping -c 1 127.0.0.1

# Sin red:
sudo ./minidocker --rootfs rootfs --net none run /bin/sh
# adentro: lo no está UP

# Conformidad:
go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...
```

---

## Bonus: par `veth`

Para dar conectividad host↔contenedor se necesita:

1. El **padre** crea un par `veth` (p. ej. `veth0` en host, `veth1` en
   contenedor) antes de `cmd.Start()`.
2. El padre mueve `veth1` al namespace del hijo vía
   `/proc/<pid>/ns/net` después de `cmd.Start()`.
3. Dentro del contenedor se levanta `veth1` y se le asigna IP
   (`10.0.0.2/24`).
4. En el host se levanta `veth0` y se le asigna IP (`10.0.0.1/24`).
5. Para salida a internet se habilita IP forwarding y NAT con iptables.
6. Al terminar se eliminan las interfaces (`Cleanup` / `Teardown`).

Esa parte no está implementada en la versión mínima; el modo `--net veth`
actualmente solo levanta `lo` y deja la configuración veth pendiente.
