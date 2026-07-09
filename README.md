# Mini-Docker

Runtime de contenedores personalizado desde cero en Go.
Proyecto académico — Taller de Programación en Go, nivel medio-avanzado.

## Estado del proyecto

| Hito | Estado | Descripción |
|---|---|---|
| **Hito 1** | ✅ Completo, verificado en WSL2 | Aislamiento de namespaces (UTS, PID, MNT) |
| **Hito 2** | ✅ Completo, verificado en WSL2 | Rootfs propio con pivot_root (fallback chroot) |
| **Hito 3** | ❌ No iniciado | cgroups (memoria y CPU) |
| **Hito 4** | ❌ No iniciado | CLI pulido, env vars, volúmenes, limpieza |
| **Hito 5** | ❌ No iniciado | Aislamiento de red (veth pair) |

## Requisitos

- **Go 1.26+** (compila en Windows, Linux, macOS)
- **Linux** con kernel 4.18+ para ejecutar (WSL2 o VM)
- **Privilegios root** o `CAP_SYS_ADMIN` para namespaces y cgroups

```bash
go version
# → go version go1.26.2 windows/amd64
```

## Compilación

```bash
# Windows (solo código, no ejecuta containers)
go build -o minidocker.exe ./cmd/minidocker

# Linux (cross-compile desde Windows)
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o minidocker.linux ./cmd/minidocker

# En Linux nativo
GOOS=linux GOARCH=amd64 go build -o minidocker ./cmd/minidocker
```

## Uso rápido (en Linux)

```bash
# 1. Preparar el rootfs de Alpine (Hito 2) — una sola vez
./scripts/setup-rootfs.sh ./rootfs
# En WSL usar un directorio nativo de Linux (los symlinks del tarball
# no sobreviven en /mnt/c ni /mnt/d):
./scripts/setup-rootfs.sh /tmp/rootfs

# 2. Ejecutar un comando aislado con rootfs propio
sudo ./minidocker --rootfs ./rootfs run /bin/sh

# Sin rootfs válido cae en "modo Hito 1" (solo namespaces, avisa por stderr)
sudo ./minidocker run /bin/sh
```

## Arquitectura del proyecto

```
mini-docker/
├── cmd/minidocker/
│   └── main.go              ← Entry point, CLI, bifurcación parent/child
├── internal/
│   ├── config/config.go     ← Tipos de configuración
│   ├── container/
│   │   ├── container.go     ← Container.Run(): re-ejecuta el binario con namespaces
│   │   ├── exec.go          ← ExecInit(): orquesta setup + ejecución
│   │   ├── exec_linux.go    ← syscall.Exec() para Linux
│   │   ├── exec_other.go    ← stub para Windows/macOS
│   │   ├── setup_linux.go   ← ✅ Hito 2: pivot_root/chroot + /proc + hostname
│   │   └── setup_other.go   ← stub para Windows/macOS
│   ├── namespace/
│   │   ├── namespace_linux.go  ← SysProcAttr con CLONE_NEWUTS|NEWPID|NEWMNT
│   │   └── namespace_other.go  ← stub para compilar en Windows/macOS
│   ├── cgroup/              ← 🔲 Hito 3 — control de recursos
│   └── network/             ← 🔲 Hito 5 — aislamiento de red
├── rootfs/                  ← Extraer Alpine aquí para Hito 2 (gitignoreado)
├── scripts/
│   └── setup-rootfs.sh      ← ✅ Hito 2 — descarga/extrae Alpine minirootfs
├── go.mod
└── README.md
```

## Flujo de ejecución

```
minidocker run /bin/sh
  │
  ├─ main.go: detecta que NO es "init", entra a runContainer()
  │
  ├─ container.go: exec.Command("/proc/self/exe", "--rootfs", X, "init", "/bin/sh")
  │   └─ SysProcAttr con CLONE_NEWUTS | CLONE_NEWPID | CLONE_NEWMNT
  │   └─ env MINIDOCKER_INIT=1
  │
  ├─ fork → kernel crea namespaces UTS, PID, MNT para el hijo
  │
  ├─ [HIJO] main.go: detecta MINIDOCKER_INIT=1, entra a initContainer()
  │
  ├─ exec.go → setupContainer():
  │   ├─ sethostname("minidocker")             ← UTS
  │   ├─ mount / privado (MS_REC|MS_PRIVATE)   ← sin fugas de montajes al host
  │   ├─ pivot_root(rootfs) — fallback chroot  ← HITO 2: raíz propia
  │   └─ mount("proc", "/proc", "proc")        ← MNT + PID, DESPUÉS del pivot
  │
  ├─ execContainer(): syscall.Exec("/bin/sh")
  │   └─ reemplaza el proceso Go por /bin/sh — es PID 1
  │
  └─ /bin/sh corriendo con aislamiento de namespaces
```

## Hitos pendientes — asignación por integrante

### Integrante A — Hito 2: Rootfs propio ✅ COMPLETADO

> **Estado:** implementado en `internal/container/setup_linux.go` y
> `scripts/setup-rootfs.sh`; verificado en WSL2 (ver "Verificación del
> Hito 2" más abajo). Se usa **pivot_root** (nivel sobresaliente) con
> fallback automático a chroot.

**Archivos a modificar:**
- `internal/container/setup_linux.go`

**Tareas:**

1. **Script de extracción de Alpine:** ✅ `scripts/setup-rootfs.sh`
   - Descargar Alpine minirootfs desde `https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-minirootfs-3.21.0-x86_64.tar.gz` (la ruta correcta es `v3.21/releases`, no `edge/releases`)
   - Extraer a `rootfs/` con `tar -xzf`
   - El directorio debe contener `/bin`, `/etc`, `/lib`, `/usr`, etc.

2. **Implementar `chrootRootfs(path string)`:** ✅
   - Guardar el root actual con `os.Getwd()` ANTES de chroot (para poder volver si es necesario)
   - `syscall.Chroot(path)`
   - `os.Chdir("/")`
   - Manejar error: `fmt.Errorf("chroot %s: %w", path, err)`

3. **Implementar `pivotRootfs(path string)` (opcional — nivel sobresaliente):** ✅
   - Bind-mount rootfs sobre sí mismo: `syscall.Mount(path, path, "", syscall.MS_BIND|syscall.MS_REC, "")`
   - Crear directorio `.oldroot` dentro del rootfs
   - `syscall.PivotRoot(path, path+"/.oldroot")`
   - `os.Chdir("/")`
   - Unmount `.oldroot`: `syscall.Unmount("/.oldroot", syscall.MNT_DETACH)`
   - Eliminar `.oldroot`

4. **Montar `/proc` DESPUÉS del chroot:** ✅
   - Mover la llamada a `mountProc()` para que ocurra después de chroot/pivot_root
   - Verificar que `ps` funcione correctamente dentro del container

5. **Verificación:** ✅
   - Dentro del container: `ls /` debe mostrar solo archivos de Alpine
   - `ps aux` debe funcionar
   - El proceso NO debe poder ver archivos del host

**Criterios de éxito:**
- `sudo ./minidocker --rootfs ./rootfs run /bin/sh -c "ls /"` muestra sistema Alpine
- No hay fugas de montajes (`mount | grep rootfs` después de salir)

#### Verificación del Hito 2 (ejecutada el 2026-07-09, WSL2 kernel 6.6.87)

```bash
./scripts/setup-rootfs.sh /tmp/rootfs
sudo ./minidocker --rootfs /tmp/rootfs run /bin/sh -c "ls /"
# → bin dev etc home lib media mnt opt proc root run sbin srv sys tmp usr var

sudo ./minidocker --rootfs /tmp/rootfs run /bin/sh -c "cat /etc/os-release"
# → NAME="Alpine Linux" VERSION_ID=3.21.0  (el host NO es Alpine 3.21)

sudo ./minidocker --rootfs /tmp/rootfs run /bin/sh -c "ps aux"
# → PID 1 = /bin/sh; solo se ven los procesos del container

sudo ./minidocker --rootfs /tmp/rootfs run /bin/sh -c "ls /.oldroot"
# → No such file or directory  (el root viejo del host fue desmontado)

mount | grep /tmp/rootfs   # después de salir
# → (vacío: sin fugas de montajes)
```

**Detalles de implementación:**
- `pivot_root(2)` en lugar de `chroot(2)`: desmonta el root del host por
  completo (sin vías de escape). Si `pivot_root` falla (p. ej. en
  filesystems de red), cae automáticamente a `chroot` con un aviso.
- Antes del pivot se remonta `/` como `MS_REC|MS_PRIVATE`: en hosts con
  propagación *shared* (systemd) esto evita que los montajes del
  container se fuguen al host.
- Bugfix del Hito 1: el proceso padre ahora **reenvía `--rootfs` al
  proceso init** (antes el hijo siempre usaba el valor por defecto) y la
  ruta se normaliza a absoluta.
- Si el rootfs no existe o no contiene `bin/`, se ejecuta en "modo
  Hito 1" (solo namespaces) con un aviso por stderr.

---

### Integrante B — Hito 3: cgroups (límites de memoria y CPU)

**Archivos nuevos:**
- `internal/cgroup/cgroup.go` — tipos y funciones comunes
- `internal/cgroup/cgroup_linux.go` — implementación real
- `internal/cgroup/cgroup_other.go` — stub Windows/macOS

**Archivos a modificar:**
- `internal/container/container.go` — integrar cgroup en `Run()`
- `internal/config/config.go` — agregar campos `MemoryLimit`, `CPULimit`

**Tareas:**

1. **Detección de cgroups v2:**
   - Leer `/proc/filesystems` y buscar "cgroup2"
   - Verificar existencia de `/sys/fs/cgroup/unified/` o `/sys/fs/cgroup/systemd/`
   - Si cgroups v2 no están disponibles, retornar error descriptivo

2. **Crear cgroup hijo:**
   - En cgroups v2: crear subdirectorio en `/sys/fs/cgroup/minidocker/<container-id>/`
   - Escribir PID del proceso hijo en `cgroup.procs`
   - En cgroups v1: similar pero en `/sys/fs/cgroup/memory/minidocker/` y `/sys/fs/cgroup/cpu/minidocker/`

3. **Aplicar límite de memoria:**
   - Escribir límite en bytes en `memory.max` (v2) o `memory.limit_in_bytes` (v1)
   - Ejemplo: `echo "100000000" > /sys/fs/cgroup/minidocker/foo/memory.max`

4. **Aplicar límite de CPU:**
   - Escribir cuota y período en `cpu.max` (v2) o `cpu.cfs_quota_us` (v1)
   - Ejemplo: `echo "50000 100000" > /sys/fs/cgroup/minidocker/foo/cpu.max` (0.5 CPU)

5. **Integrar en `Container.Run()`:**
   - Crear cgroup ANTES de `cmd.Start()`
   - Agregar PID del hijo al cgroup DESPUÉS de `cmd.Start()`
   - Eliminar el cgroup en un `defer` después de `cmd.Wait()`

6. **Limpieza absoluta:**
   - Si el proceso es kill -9, el cgroup debe eliminarse igual
   - Usar `defer` con función de cleanup
   - Leer procesos restantes en `cgroup.procs` y matarlos si es necesario

7. **Verificación:**
   - `sudo ./minidocker run /bin/sh -c "cat /sys/fs/cgroup/..."` debe mostrar el límite
   - `stress --vm 1 --vm-bytes 200M` dentro de un container con 100MB de límite debe ser OOM-killed
   - Después de salir: `/sys/fs/cgroup/minidocker/` debe estar vacío

**Interfaz propuesta:**

```go
package cgroup

type Manager struct {
    path string   // ej: /sys/fs/cgroup/minidocker/<id>
}

func New(id string) (*Manager, error)
func (m *Manager) SetMemoryLimit(bytes int64) error
func (m *Manager) SetCPULimit(quota, period int64) error
func (m *Manager) AddProcess(pid int) error
func (m *Manager) Cleanup() error
```

**Criterios de éxito:**
- Límite de memoria aplicado y verificable
- OOM kill demostrable
- `Cleanup()` no deja residuos en el host

---

### Integrante C — Hito 5: Aislamiento de red (veth pair)

**Archivos nuevos:**
- `internal/network/network.go` — tipos y funciones comunes
- `internal/network/network_linux.go` — implementación real
- `internal/network/network_other.go` — stub Windows/macOS

**Tareas:**

1. **Par veth:**
   - Llamar a `syscall.Socketpair(unix.AF_UNIX, ...)` o usar `netlink`
   - Crear par veth con nombres veth0/veth1
   - Mover veth1 al namespace de red del container

2. **Configurar loopback:**
   - Dentro del container: levantar interfaz `lo`
   - Verificar con `ip addr` o `ifconfig`

3. **Configurar IP:**
   - Asignar IP al veth del host (ej: 10.0.0.1/24)
   - Asignar IP al veth del container (ej: 10.0.0.2/24)
   - Configurar ruta por defecto en el container

4. **Salida a internet (opcional):**
   - Habilitar IP forwarding en el host
   - Configurar iptables MASQUERADE
   - Verificar con `ping 8.8.8.8`

5. **Limpieza:**
   - Al salir: eliminar veth interfaces, limpiar iptables
   - Usar `defer` en el setup

**Interfaz propuesta:**

```go
package network

type Bridge struct {
    name string
}

func Setup() (*Bridge, error)
func (b *Bridge) Attach(pid int) error
func (b *Bridge) Cleanup() error
```

**Criterios de éxito:**
- Container tiene interfaz loopback propia
- `ping 127.0.0.1` funciona dentro del container
- Container puede comunicarse con el host (ping a la IP del bridge)

---

### Integrante D — Hito 4: CLI pulido, env vars, volúmenes, integración

**Archivos a modificar:**
- `cmd/minidocker/main.go` — nuevos flags
- `internal/config/config.go` — nuevos campos
- `internal/container/container.go` — pasar nueva config
- `internal/container/setup_linux.go` — bind mounts para volúmenes

**Tareas:**

1. **CLI con flags:**
   - `--env KEY=VALUE` (múltiple, como Docker): agregar un slice `[]string` de variables de entorno
   - `--volume /host:/container` (múltiple): agregar un slice `[]Volume` con origen y destino
   - `--memory 100m` o `--memory 100000000`: pasar a cgroups
   - `--cpu 0.5`: pasar a cgroups
   - Mensajes de error claros y consistentes

2. **Variables de entorno:**
   - En `Config.Env`, parsear "KEY=VALUE"
   - En `setup_linux.go`, aplicarlas con `os.Setenv()` ANTES de `syscall.Exec`
   - Verificar: `./minidocker --env MI_VAR=hola run /bin/sh -c "echo $MI_VAR"`

3. **Volúmenes (bind mounts):**
   - En `setup_linux.go`, montar cada volumen con `syscall.Mount(hostPath, containerPath, "", syscall.MS_BIND|syscall.MS_REC, "")`
   - Crear directorio destino si no existe
   - Verificar: crear archivo en host, debe verse en container

4. **Manejo de señales:**
   - Capturar SIGTERM/SIGINT en el proceso padre
   - Propagar la señal al proceso hijo (que es el container)
   - Si el hijo no termina en N segundos, enviar SIGKILL

5. **Integración:**
   - Integrar `cgroup.Manager` y `network.Bridge` en `Container.Run()`
   - Llamar a Cleanup() en el orden correcto
   - Asegurar que todos los recursos se limpien incluso si algo falla

6. **Verificación holística:**
   - Ciclo completo: container con rootfs, límites de memoria, variables de entorno, volúmenes y red
   - Al salir: sin procesos zombies, sin cgroups colgados, sin interfaces de red huérfanas, sin montajes colgados

**Criterios de éxito:**
- `--help` muestra todos los flags disponibles
- `--env` y `--volume` funcionan simultáneamente
- Ctrl+C mata el container limpio
- No quedan residuos en el host

---

## Tareas administrativas

- [ ] Verificar Hito 1 en VM Linux
- [ ] Bitácora de decisiones (Anexo A)
- [ ] Declaración de uso de IA (Anexo B)
- [ ] Defensa oral con modificación en vivo

## Licencia

Proyecto académico — Universidad Señor de Sipán.
