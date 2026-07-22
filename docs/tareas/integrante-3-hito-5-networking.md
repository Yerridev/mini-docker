# Tarea — Integrante 3: Hito 5, Networking (CLONE_NEWNET + loopback)

> **Instruido a:** un LLM que va a **escribir código Go** y tests. Esta
> tarea es de implementación, no de redacción.

---

## Rol

Sos un ingeniero Go senior que implementa el **Hito 5 (opcional)** de un
runtime de contenedores llamado `mini-docker`. El hito añade **networking**
al contenedor: aislar la red con `CLONE_NEWNET` y levantar la interfaz
`lo` (loopback) dentro del namespace. Opcionalmente, configurar un par
`veth` para conectividad host↔contenedor.

## Por qué existe esta tarea

El Hito 5 es **opcional** (bonus de rúbrica, hasta 25% extra) pero completa
el stack de namespaces del runtime. La rúbrica dice:

> Configurar un par `veth` y un namespace de red para dar conectividad al
> contenedor, o al menos un `loopback` aislado. Opcional: empaquetar/
> desempaquetar una imagen como tar.

**Objetivo mínimo:** `CLONE_NEWNET` + `lo up` → el contenedor tiene red
loopback aislada (ping a 127.0.0.1 funciona, pero no al host).
**Objetivo completo:** par `veth` + bridge/NAT para conectividad externa.

## Contexto del proyecto

`mini-docker`: runtime de contenedores en Go. Patrón re-exec (padre ejecuta
`/proc/self/exe` como hijo con `MINIDOCKER_INIT=1`). Repositorio
`https://github.com/Yerridev/mini-docker`, rama `main`, HEAD `2e3d38a`.

**Namespaces ya activos:** UTS, PID, MNT (ver
`internal/namespace/namespace_linux.go`).
**Falta:** NET.

**Arquitectura clave para esta tarea:**
- `internal/namespace/namespace_linux.go` define `SysProcAttr()` con
  `Cloneflags` — ahí se añade `CLONE_NEWNET`.
- El proceso **hijo** (init) vive dentro de los namespaces y es quien debe
  configurar `lo` up (con privilegios dentro del namespace).
- El proceso **padre** puede crear el par `veth` antes de arrancar el hijo
  y mover un extremo al namespace del hijo (vía `/proc/<pid>/ns/net`).

## Entregables

1. **`internal/namespace/namespace_linux.go`**: añadir flag `CLONE_NEWNET`.
2. **`internal/netns/` (nuevo paquete)**: configuración de red del contenedor.
   - `netns.go` (común): interfaz `Setup`.
   - `netns_linux.go`: implementación que levanta `lo` (y opcionalmente veth).
   - `netns_other.go`: stub no-op para Windows/macOS (build tag `!linux`).
3. **`internal/container/setup_linux.go`**: invocar `netns.Setup` en
   `setupContainer` (después de `mountProc`, fase 4).
4. **`internal/config/config.go`**: campos opcionales `Hostname` (ya
   sugerido como ejercicio de defensa) + `NetEnabled bool` (default true
   cuando se activa `CLONE_NEWNET`).
5. **`cmd/minidocker/main.go`**: flag `--net` (`loopback` default, `veth`
   opcional, `none` para deshabilitar).
6. **Tests**: `internal/netns/netns_test.go` con unit tests de parseo de
   flags y de la lógica portable. Tests Linux-only en
   `internal/netns/netns_linux_test.go` con `//go:build linux` para lo que
   toque `/proc` o `ip`/`ifconfig`.
7. **`docs/hito-5-networking.md`**: sección breve explicando qué aísla
   `CLONE_NEWNET`, cómo se levanta `lo`, y cómo se configura `veth`
   (si se implementa).

## Restricciones

- **Linux obligatorio** para la implementación real. Stubs para otros SO.
- **No usar libnetwork, CNI ni runc.** Implementar con `netlink` directo
  (paquetes `golang.org/x/sys/unix` y/o `github.com/vishvananda/netlink`
  solo si ya está en `go.mod` — **verificá antes de agregar dependencias**).
  Preferí usar syscalls crudas (`unix.SIOCSIFFLAGS`, etc.) o el estándar
  `net` si alcanza.
- **Requiere root** (CAP_NET_ADMIN para crear interfaces). Documentarlo.
- **No romper los hitos existentes**: `go build ./...`, `go vet ./...`,
  `gofmt -l .` y `go test -race ./...` deben seguir pasando.
- **Commits como work-units** (tests + código juntos), con conventional
  commits. **NO agregar `Co-Authored-By`** en los mensajes de commit.
- **Forzar LF** en cualquier `.go` nuevo (ya está en `.gitattributes`,
  no hay que tocarlo).

## Información de base

### Archivos a leer antes de empezar

```
internal/namespace/namespace_linux.go   # añadir CLONE_NEWNET
internal/namespace/namespace_other.go   # stub — ver si NECESITA cambios
internal/container/setup_linux.go       # punto de invocación de netns.Setup
internal/container/container.go         # ver cómo el padre prepara al hijo
internal/cgroup/cgroup_linux.go         # referencia de split linux/other
internal/config/config.go               # añadir campos de red
cmd/minidocker/main.go                  # flag --net
go.mod                                   # verificar dependencias antes de agregar
```

### Flags de clone(2)

```
CLONE_NEWNET = 0x40000000   // ver syscall/linux/ztypes_linux*.go en x/sys
```
No está en `syscall` stdlib como constante nombrada — usá el valor literal
comentado o importalo de `golang.org/x/sys/unix.CLONE_NEWNET`.

### Funciones relevantes (Go stdlib + unix)

- `net.Interfaces()` y `(*net.Interface).Addrs()` para listar/verificar
  interfaces dentro del hijo (diagnóstico).
- `unix.IoctlSetInt(socket, unix.SIOCSIFFLAGS, flags|unix.IFF_UP|IFF_RUNNING)`
  para levantar `lo`. Documentación: `man 7 netlink`, `man 4 net_device`.
- Alternativa más simple para `lo`: usar comandos `ip link set lo up`
  ejecutados desde el hijo con `exec.Command` — válido pero menos idiomático.
  Preferí netlink/ioctl directo.

### Orden de setup en `setupContainer` (actual)

1. `sethostname`
2. `makeMountsPrivate`
3. `mountVolumes`
4. `prepareRootfs` + `pivotRootfs` (fallback chroot)
5. `mountProc`
6. **← NUEVO: `netns.Setup`** (fase 4, red)

## Pasos sugeridos

1. Verificá `go.mod` y decidí si usar `golang.org/x/sys/unix` (probablemente
   ya como dependencia indirecta vía `syscall`) o `github.com/vishvananda/
   netlink`. Preferí `x/sys/unix` para no agregar dependencias nuevas
   salvo que sea muy complejo.
2. Añadí `flagNEWNET = 0x40000000` en `namespace_linux.go` y OR con los
   otros flags. Verificá que el hijo arranca (sin red aún) sin romper H1-H4.
3. Implementá el paquete `internal/netns/`:
   - Setup mínimo: dentro del hijo, levantar `lo` con ioctl/netlink.
   - Test con `ping -c 1 127.0.0.1` dentro del contenedor (debe funcionar).
4. Invocá `netns.Setup` en `setup_linux.go` después de `mountProc`.
5. Añadí flag `--net` en `main.go` con valores `loopback`/`none`
   (veth opcional, fase 2).
6. Agregá tests: parseo de `--net`, validación, y un test Linux-only que
   ejecute un contenedor y verifique que `lo` está UP (con helper-process
   como en `signals_linux_test.go`).
7. Actualizá `docs/hito-5-networking.md` y agregá una sección en
   `docs/ejecucion-y-pruebas.md` (o un nuevo doc) con las pruebas del Hito 5.
8. **Opcional (veth completo):** crear par veth en el padre, mover un
   extremo al namespace del hijo vía `/proc/<pid>/ns/net`, asignar IPs
   (10.0.0.x/24), levantar NAT con `iptables`. Mucho más complejo —
   dejá para una segunda iteración si el tiempo apremia.

## Criterios de aceptación

### Mínimo (loopback)
- [ ] `CLONE_NEWNET` activado en `SysProcAttr.Cloneflags`.
- [ ] `lo` está UP dentro del contenedor (verificable con `ip addr` o
      `cat /proc/net/dev`).
- [ ] `ping -c 1 127.0.0.1` funciona dentro del contenedor.
- [ ] El contenedor **no** tiene acceso a la red del host (verificable:
      `ping <host_ip>` falla, o `ip route` muestra solo loopback).
- [ ] `--net none` deshabilita el setup (contenedor sin `lo` up).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` limpios.
- [ ] `go test -race ./...` verde.
- [ ] Tests nuevos en `internal/netns/`.
- [ ] `docs/hito-5-networking.md` existe y explica `CLONE_NEWNET` + `lo`.

### Completo (veth — bonus)
- [ ] Par `veth` creado en el padre, un extremo movido al namespace del hijo.
- [ ] IPs asignadas (host 10.0.0.1/24, contenedor 10.0.0.2/24).
- [ ] `ping` host↔contenedor funciona en ambos sentidos.
- [ ] NAT/masquerade opcional para salida a internet del contenedor.
- [ ] Limpieza del par veth al terminar el contenedor (en `Cleanup` o
      equivalente).

## Cómo verificar

```bash
# En Linux (VM Ubuntu/Kali), como root:
go build -o minidocker ./cmd/minidocker
./scripts/setup-rootfs.sh   # si no está ya

# Loopback:
sudo ./minidocker --rootfs rootfs run /bin/sh
# adentro:
ip addr 2>/dev/null || cat /proc/net/dev    # lo debe estar UP
ping -c 1 127.0.0.1                          # debe responder

# Sin red al host:
ping -c 1 <IP_del_host>                      # debe fallar (network unreachable)

# --net none:
sudo ./minidocker --rootfs rootfs --net none run /bin/sh
# adentro: lo no está UP (o no existe la interfaz con UP)

# Conformidad:
go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...
```

## No hacer

- No tocar `internal/cgroup/`, `internal/config/` más allá de los campos
  necesarios (NetEnabled, opcional Hostname).
- No romper los hitos 1-4 existentes.
- No agregar `Co-Authored-By` a los commits.
- No commitear directamente a `main` sin pasar por el dueño del repo
  (abrir rama + PR). **El estudiante decide cuándo mergear.**
- No usar emojis en código ni commits.
- No usar `runc`, `libnetwork` ni CNI.

## Riesgos / gotchas conocidos

- **`lo` no se levanta solo**: en un namespace NET nuevo, `lo` existe pero
  está DOWN. Hay que ponerlo UP explícitamente.
- **CAP_NET_ADMIN requerido**: dentro del namespace, root lo tiene por
  default, pero si se quita con `CLONE_NEWUSER` (no activo aquí), no.
- **Orden de `netns.Setup`**: debe ser después de `pivot_root`/`chroot` y
  `mountProc` — el diagnóstico vía `/proc/net/dev` requiere `/proc` propio.
- **`vishvananda/netlink` vs syscalls crudas**: la primera es más ergonómica
  pero agrega dependencia. Si el proyecto no la usa ya, preferí ioctl directo
  y documentá por qué.
- **Limpieza de veth**: el extremo del host del par veth debe borrarse al
  terminar el contenedor, si no queda interfaz zombie. Considerá un `defer`
  en el padre tipo `defer netns.Teardown(...)`.