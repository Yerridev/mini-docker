# Mini-Docker — Conceptos base

Referencia teórica concisa para entender el runtime `mini-docker`. Cada
sección explica el concepto general y después dónde aparece en el código
(`file_path:line`).

> This doc is **concept-first**, not code-first. Para verlo correr:
> `docs/ejecucion-y-pruebas.md`. Para decisiones de diseño: `docs/anexo-a-bitacora.md`.

---

## 1. Qué es un contenedor

Un contenedor **no es una máquina virtual**. No hay hipervisor, no hay
segunda instancia del kernel. Un contenedor es un **proceso normal de Linux**
al que el kernel le presenta una **vista filtrada** del sistema y al que le
limita recursos.

| Característica | Proceso normal | VM | Contenedor |
|---|---|---|---|
| Kernel que ejecuta | el del host | el suyo propio | el del host |
| Aislamiento | ninguno | completo | parcial (lo que pidas) |
| Arranque | instantáneo | boot de SO | instantáneo |
| Recursos limitados | no | los de la VM | sí (cgroups) |
| Peso | bytes | GB | MB (solo rootfs) |

El kernel de Linux expone dos APIs para construir contenedores:
- **namespaces** → aislamiento (qué ve el proceso)
- **cgroups** → límites (cuánto puede consumir)

El rol de Go en este proyecto es llamar a esos syscalls de manera
conveniente, porque Go mapea prácticamente toda la API POSIX, incluyendo `clone`, `mount`, `pivot_root`, `SIG*`.

---

## 2. Linux por debajo

### 2.1 Procesos

- **PID**: identificador de proceso. El init (systemd en Ubuntu) es PID 1.
- **fork**: un proceso crea una copia de sí mismo.
- **exec**: un proceso reemplaza su imagen por otro binario (su PID no
  cambia, pero el código que ejecuta sí).
- **jerarquía**: todos los procesos descienden de PID 1 (formando un árbol).

> En el proyecto: el proceso init del contenedor es **PID 1 de su namespace**
> (no del host). El kernel lo numera desde 1 dentro del PID namespace.

### 2.2 Sistema de archivos

- **montaje**: una "fuente" (disco, procfs, bind) se cuelga en un directorio.
- **`/proc`**: filesystem virtual que expone info del kernel sobre procesos
  (`/proc/<pid>/...`). Cada PID namespace necesita su propio `/proc`.
- **rootfs**: el directorio raíz que un proceso ve como `/`. Alpine
  minirootfs es una distro mínima empaquetada.

### 2.3 Señales

Notificación asíncrona del kernel a un proceso. Las relevantes:

| Señal | Default action | Nota |
|---|---|---|
| `SIGINT` | terminar | la envía `Ctrl-C` |
| `SIGTERM` | terminar | `docker stop`, graceful shutdown |
| `SIGKILL` | matar (no catchable) | `docker kill`, forzar |

**Gotcha crítico:** PID 1 **no recibe señales por default** (no tiene handler
instalado como cualquier proceso). Por eso `docker stop` reenvía SIGTERM y,
si no responde, SIGKILL tras grace period.

### 2.4 Syscalls claves

| Syscall | Qué hace |
|---|---|
| `clone(2)` | crea un proceso hijo, con opcion de nuevos namespaces |
| `mount(2)` | cuelga un filesystem |
| `umount2(2)` | desmonta (con `MNT_DETACH` para lazy) |
| `chroot(2)` | cambia la raíz aparente del proceso |
| `pivot_root(2)` | intercambia el root del proceso y desmonta el viejo |
| `sethostname(2)` | cambia el hostname (UTS namespace) |
| `execve(2)` | reemplaza la imagen del proceso |

### 2.5 Permisos y capacidades

- **root (UID 0)**: bypass la mayoría de checks.
- **capabilities**: subdividen el poder de root. Relevantes:
  - `CAP_SYS_ADMIN` → namespaces (exigida por `CLONE_NEW*` y mount).
  - `CAP_NET_ADMIN` → configurar interfaces (`lo`, `veth`).
- **cgroups v2**: escritura en `/sys/fs/cgroup` requiere root.

---

## 3. Namespaces — aislamiento

### 3.1 Qué es

Un namespace es una **vista parcial** del sistema. El kernel, al crear un
proceso, puede etiquetarlo con namespaces distintos a los del padre. A
partir de ese momento el proceso solo ve lo que está en sus namespaces.

> No es virtualización: el kernel es el mismo. Es el kernel filtrando qué
> ve cada proceso.

### 3.2 Los 6 tipos

| Namespace | Aísla | Flag `clone(2)` | Ejemplo de efecto |
|---|---|---|---|
| UTS | hostname, domainname | `CLONE_NEWUTS` | `hostname` distinto al host |
| PID | numeración de procesos | `CLONE_NEWPID` | primer proceso es PID 1 |
| MNT | árbol de montajes | `CLONE_NEWMNT` | mounts nuevos no afectan al host |
| NET | interfaces de red, rutas, sockets | `CLONE_NEWNET` | solo ves `lo` (o nada) |
| USER | UID/GID mapping | `CLONE_NEWUSER` | root del contenedor ≠ root host |
| IPC | colas de mensajes, semáforos | `CLONE_NEWIPC` | aislamiento SysV IPC |

### 3.3 Cómo se piden

En Go, vía `syscall.SysProcAttr.Cloneflags` al crear el hijo con
`os/exec`:

```go
cmd := exec.Command(...)
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWMNT,
}
```

Las flags solo aplican **al crear** el proceso — no se pueden activar después
sobre un proceso ya vivo. Esto fuerza el patrón *re-exec* (sección 6).

### 3.4 En el proyecto

`internal/namespace/namespace_linux.go:13`:
```go
Cloneflags: flagNEWUTS | flagNEWPID | flagNEWMNT   // 0x04000000 | 0x20000000 | 0x00020000
```

Importante: **no** se crean `CLONE_NEWNET` ni `CLONE_NEWUSER` en los hitos
1-4. `CLONE_NEWNET` es el Hito 5 (opcional).

---

## 4. Cambio de raíz — chroot vs pivot_root

### 4.1 `chroot(2)`

Cambia la raíz **aparente** del proceso. Los lookups de `/...` se resuelven
dentro del nuevo root. **Pero el root viejo sigue montado** y es accesible
con técnicas conocidas (clásico escape de chroot via `mkdir .old; chroot .
old; ...`). Es una cárcel no muy robusta.

### 4.2 `pivot_root(2)`

Más fuerte. **Intercambia** el root del proceso por uno nuevo y el viejo
termina en un directorio (`.oldroot`). Después lo desmontás (`umount` /
`MNT_DETACH`) y eliminás (`rmdir`). No queda vía de escape: el root viejo
del host literalmente desaparece para el contenedor.

Requisitos de `pivot_root`:
- `new_root` debe ser un mount point (se logra con un bind mount sobre sí
  mismo, `MS_BIND`).
- El árbol de montajes del proceso debe marcarse como privado (`MS_REC|MS_PRIVATE`) o `pivot_root` puede rechazar.

### 4.3 `MS_REC|MS_PRIVATE` (make-rprivate)

Cada mount tiene un **tipo de propagación**:
- `MS_SHARED`: cambios se propagan al peer (y viceversa).
- `MS_PRIVATE`: cambios no se propagan en ningún sentido.

En systemd, `/` está como `MS_SHARED` por default. Si el contenedor monta
cosas sin tocar esto, los mounts **se fugan al host** (síntoma real: la
primera corrida anda, las siguientes fallan con `fork/exec /proc/self/exe:
no such file` porque `/proc` del host se corrompe).

Por eso `makeMountsPrivate()` corre **antes** que cualquier otro mount.

### 4.4 En el proyecto

`internal/container/setup_linux.go:67` (`pivotRootfs`) intenta `pivot_root`
con **fallback automático a `chroot`** (línea 146) si falla (p. ej. en
NFS/tmpfs especial).

Orden crítico de `setupContainer` (línea 121):
```
sethostname → makeMountsPrivate → mountVolumes → prepareRootfs
→ pivotRootfs (fallback chroot) → mountProc
```

`mountProc` va **último**, después del cambio de raíz: así ve el PID
namespace correcto dentro del nuevo rootfs.

---

## 5. cgroups v2 — limitar recursos

### 5.1 Jerarquía unificada

En v2 existe un **único árbol** colgando de `/sys/fs/cgroup`. Cada
subdirectorio es un cgroup. Un proceso pertenece a exactamente un cgroup por
jerarquía.

### 5.2 Delegación de controladores

Un cgroup solo puede aplicar un controlador (memory/cpu) si el **padre**
tiene ese controlador habilitado en su `cgroup.subtree_control`. Por eso
antes de crear `/sys/fs/cgroup/minidocker/<id>` hay que escribir
`+memory +cpu` en `/sys/fs/cgroup/cgroup.subtree_control` **y** en
`/sys/fs/cgroup/minidocker/cgroup.subtree_control`.

### 5.3 `memory.max` → OOM

Límite de memoria en bytes. Si un proceso lo supera, el kernel selecciona
el proceso del cgroup con mayor uso y lo mata:
```
Memory cgroup out of memory: Killed process (1234) (awk)
```

### 5.4 `cpu.max` → quota/period

Formato `"quota period"` (microsegundos). Ejemplos:

| `cpu.max` | Signado | CPU efectiva |
|---|---|---|
| `50000 100000` | cada 100 ms, 50 ms de CPU | 0.5 CPU |
| `200000 100000` | cada 100 ms, 200 ms de CPU | 2 CPUs (si hay) |
| `max 100000` | sin límite | todas |

El CPU quota es **no hard-cap**: un proceso puede rabiar si el sistema está
idle, el scheduler lo limita solo cuando hay contención.

### 5.5 `cgroup.procs` → mover proceso

Para aplicar límites a un proceso existente, escribís su PID en
`cgroup.procs` del cgroup destino. El proceso queda sujeto a los límites
inmediatamente (y todos sus hijos futuros).

### 5.6 `cgroup.kill` → limpieza

`cgroup.kill "1"` (kernel 5.14+) mata todo el subárbol del cgroup de una
sola escritura. Alternativa para kernels viejos: leer `cgroup.procs`,
SIGKILL uno por uno.

### 5.7 En el proyecto

`internal/cgroup/cgroup_linux.go`:
- `New(id)` crea `/sys/fs/cgroup/minidocker/<id>` y delega controladores
  en el padre (línea 34).
- `SetMemoryLimit` escribe `memory.max` (línea 57).
- `SetCPULimit` escribe `cpu.max` vía `FormatCPUMax(quota, period)`
  (línea 67).
- `Cleanup` mata y elimina con `removeWithRetry` (reintenta `rmdir` ante
  `EBUSY` — el kernel tarda ms en vaciar el cgroup).

---

## 6. Patrón re-exec (`/proc/self/exe` + `MINIDOCKER_INIT`)

### 6.1 Por qué re-ejecutarse

Las flags `CLONE_NEW*` solo aplican **al nacer** del proceso. No podés
"activar un namespace" sobre un proceso que ya está corriendo. Por eso
para que el hijo tenga namespaces propios, el **padre** debe crearlo con
`SysProcAttr.Cloneflags` activo desde el arranque.

El truco: el padre ejecuta `/proc/self/exe` (un symlink del kernel al
binario que está corriendo) como **hijo**, pasándole variables de entorno
que el hijo detecta para saber que es el "init".

### 6.2 Flujo padre → hijo → syscall.Exec

1. **Padre** armar argv y env (`MINIDOCKER_INIT=1`).
2. **Padre** `exec.Command("/proc/self/exe", ...)` con `SysProcAttr.Cloneflags`.
3. **Hijo** arranca en `main()`, ve `MINIDOCKER_INIT=1` y bifurca a
   `initContainer` en vez de `runContainer`.
4. **Hijo** ya dentro de sus namespaces: ejecuta setup (hostname, mounts,
   pivot_root, /proc).
5. **Hijo** `syscall.Exec(cmd, args, env)` **reemplaza su imagen** por el
   comando del usuario (`/bin/sh`).

El PID no cambia entre pasos 3 y 5: el proceso init de minidocker
desaparece y queda el proceso del usuario con el mismo PID (1, porque nació
en un PID namespace nuevo).

### 6.3 En el proyecto

- `cmd/minidocker/main.go:41`: `if isInit() { initContainer(...) }` —
  bifurcación temprana.
- `internal/container/container.go:43`: `exec.Command("/proc/self/exe",
  initArgs...)` — el re-exec.
- `internal/container/exec_linux.go:10`: `syscall.Exec(...)` — reemplazo
  final de la imagen.

Es el mismo patrón de **runc** y Docker. No es invención del proyecto.

---

## 7. Go — conceptos justos que usa el runtime

### 7.1 `os/exec` — `cmd.Start` vs `cmd.Run`

`cmd.Run()` = `Start()` + `Wait()`. El runtime usa `Start` solo (sin
`Wait` automático) para obtener el PID y agregarlo al cgroup antes de
esperar a que termine.

`cmd.SysProcAttr` es el hook donde se meten las flags de namespaces.

### 7.2 `syscall` y `syscall.SysProcAttr`

- `syscall.SysProcAttr.Cloneflags` → bits OR de `CLONE_NEW*`.
- `syscall.Sethostname` → syscall de UTS.
- `syscall.Mount` / `syscall.PivotRoot` / `syscall.Unmount` → syscalls de
  MNT.
- `syscall.Exec` → reemplaza imagen del proceso (no crea uno nuevo).
- `syscall.Kill` → envía señal a un PID.

`syscall` stdlib es portable en su API pero las flags concretas son
Linux-only — por eso el proyecto separa por archivos con build tags.

### 7.3 Goroutines + channels + `signal.Notify`

Goroutine = función que corre **concurrentemente**. Channel = tubería
tipada para sincronizar goroutines sin locks explícitos.

`signal.Notify(ch, sig...)` registra un canal donde el runtime de Go
**replica** las señales que el proceso recibe. En el proyecto: la goroutine
de `forwardSignals` escucha ese canal y reenvía al hijo.

### 7.4 `defer` — ciclo de vida de recursos

`defer f()` difiere `f()` hasta el return de la función, LIFO. El runtime
lo usa para garantías de cleanup:

```go
cg = cgroup.New(...)
defer cg.Cleanup()   // pase lo que pase, limpiar cgroup
```

Y `forwardSignals` devuelve un `stop` que se llama con `defer stop()`:
cierra el channel `done` y desregistra la señal al salir.

### 7.5 Build tags — `//go:build linux`

Comentario mágico al **inicio** del archivo que restringe a un build
constraint. El proyecto los usa para separar Linux real de stubs:

- `setup_linux.go` (Linux) — la implementación real.
- `setup_other.go` (`!linux`) — stubs que compilan pero no hacen nada.
- `signals_linux_test.go` (`linux`) — tests de `forwardSignals` (usan
  `/bin/sleep` y señales Linux).

Permite `go build ./...` y `go test ./...` en Windows/macOS sin fallar.

### 7.6 `go test -race` — detector de carreras

Flag del runtime de Go que **instrumenta el código** para detectar accesos
concurrentes inseguros a memoria. Cuando hay concurrencia (goroutines +
channels sin locks), `-race` es la herramienta que detecta data races que
pasan silenciados en producción. La rúbrica lo exige por eso.

---

## 8. Movimientos de datos del runtime

### 8.1 Bind mount (volúmenes)

`mount --bind /host /cont` hace que `/cont` referencie el mismo inodo que
`/host`. Cambios en uno se ven en el otro (son el mismo objeto). Es lo que
implementa `--volume /host:/cont`.

### 8.2 `mergeEnv`

Combina el entorno heredado del padre (`os.Environ()`) con las variables
pasadas por `--env`. **`--env` tiene prioridad**: si repetís una clave
(ej. `--env PATH=/bin`), pisa la heredada. Lógica de `mergeEnv` en
`container.go:123`.

### 8.3 `forwardSignals`

PID 1 del namespace no recibe señales por default. `forwardSignals`
goroutine en `container.go:97` atrapa `SIGINT`/`SIGTERM` con
`signal.Notify` y las reenvía al proceso init. Si no muere en `killGrace`
(3 s), fuerza `SIGKILL`. Mismo modelo que `docker stop`.

---

## 9. Flujo de ejecución completo

Desde `minidocker --rootfs R --memory M --cpu C --env K=V --volume /h:/c run /bin/sh`.

1. **`main()`** (padre): parsea flags, valida (`config.Parse*`).
2. **`isInit()`** retorna `false` (no tiene `MINIDOCKER_INIT=1`), va a
   `runContainer`.
3. **`runContainer`** construye `cfg` y `container.New(cfg).Run()`.
4. **`Container.Run`** (`container.go:32`):
   - Arma `initArgs = [--rootfs R --volume /h:/c init /bin/sh]`.
   - `exec.Command("/proc/self/exe", initArgs...)` con `SysProcAttr.Cloneflags`
     (UTS+PID+MNT) y `Env = mergeEnv(os.Environ()+MINIDOCKER_INIT=1,
     cfg.Env)`.
   - Si hay `--memory`/`--cpu`: `cgroup.New` + `SetMemoryLimit` +
     `SetCPULimit` (`defer Cleanup`).
   - `cmd.Start()` → obtiene `cmd.Process.Pid`.
   - `cg.AddProcess(pid)` → el hijo queda sujeto a los límites.
   - `forwardSignals(cmd)` (`defer stop`).
   - `cmd.Wait()` bloquea hasta que el hijo termina.
5. **Hilo del hijo** arranca en su propio namespace:
   - `main()` ve `MINIDOCKER_INIT=1` → `isInit()` true → `initContainer`.
   - `parseVolumes` con `--volume` (reenviado por el padre).
   - `container.ExecInit(cfg)` = `setupContainer` + `execContainer`.
6. **`setupContainer`** (`setup_linux.go:121`):
   - `sethostname("minidocker")` (UTS)
   - `makeMountsPrivate()` (MS_REC|MS_PRIVATE sobre `/`)
   - `mountVolumes(cfg)` (bind mounts **antes** del pivot)
   - `prepareRootfs` + `pivotRootfs` (fallback chroot)
   - `mountProc` (después del root change)
7. **`execContainer`** (`exec_linux.go:10`): `syscall.Exec("/bin/sh",
   ["/bin/sh"], env)` — reemplaza la imagen del proceso init por la de
   `/bin/sh`. El PID sigue siendo 1 del namespace.
8. El usuario interactúa con `/bin/sh` dentro del contenedor. Al salir,
   `cmd.Wait` vuelve en el padre, `defer stop()` cierra `forwardSignals`,
   `defer cg.Cleanup()` mata y borra el cgroup. Fin.

---

## 10. Glosario

| Término | Definición de una línea |
|---|---|
| **namespace** | Vista parcial del sistema que el kernel muestra a un proceso (UTS, PID, MNT, NET, USER, IPC). |
| **`CLONE_NEW*`** | Flags de `clone(2)` que piden namespaces nuevos para el proceso hijo. |
| **cgroup v2** | Jerarquía unificada en `/sys/fs/cgroup` para limitar recursos de un grupo de procesos. |
| **`memory.max`** | Límite de memoria del cgroup. Excederlo dispara OOM kill del kernel. |
| **`cpu.max`** | Límite de CPU: `"quota period"` (microsegundos); `50000 100000` = 0.5 CPU. |
| **`cgroup.procs`** | Archivo con los PIDs miembros del cgroup. Escribir un PID lo mueve ahí. |
| **`cgroup.kill`** | Archivo que mata todo el subárbol del cgroup con escribir `1`. |
| **`/proc/self/exe`** | Symlink mágico del kernel al binario en ejecución. Base del patrón re-exec. |
| **re-exec** | Técnica donde el padre ejecuta una copia de sí mismo como hijo (para que el hijo nazca con namespaces). |
| **`pivot_root(2)`** | Syscall que intercambia el root del proceso y desmonta el viejo. |
| **`chroot(2)`** | Syscall que cambia la raíz aparente del proceso. Menos seguro que `pivot_root`. |
| **bind mount** | Montar una ruta del host en otra ruta (implementa `--volume`). |
| **`MS_REC\|MS_PRIVATE`** | Marca el árbol de montajes como privado: los mounts nuevos no se propagan al padre. |
| **OOM kill** | Out-Of-Memory kill: el kernel mata el proceso que supera `memory.max`. |
| **PID 1** | Primer proceso de un PID namespace. No recibe señales por default (no tiene handler del kernel). |
| **`killGrace`** | Período de gracia (3 s) antes de forzar SIGKILL. Mismo modelo que `docker stop`. |
| **`EBUSY`** | errno que devuelve `rmdir` si el cgroup aún tiene procesos (hay que reintentar). |
| **`syscall.Exec`** | Reemplaza la imagen del proceso actual (no crea uno nuevo). Lo usa el init tras el setup. |
| **`SysProcAttr`** | Struct de Go donde se setea `Cloneflags` para pedir namespaces al crear el hijo. |
| **`signal.Notify`** | Función de Go que replica señales del proceso en un channel, accesible desde goroutines. |

---

## 11. Ver también

- [`docs/ejecucion-y-pruebas.md`](./ejecucion-y-pruebas.md) — cómo instalar
  el runtime en Linux/WSL2, ejecutar los 4 hitos y validar la rúbrica.
- [`docs/anexo-a-bitacora.md`](./anexo-a-bitacora.md) — bitácora de
  decisiones de diseño por hito (control de autoría rúbrica 2.3).
- [`docs/tareas/`](./tareas/) — briefs para los integrantes del grupo.
- [`guia minidocker.md`](../guia%20minidocker.md) — guía original del
  docente (no commiteada, en `.gitignore`).