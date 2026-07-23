# Mini-Docker — Guía de instalación, ejecución y pruebas

Guía completa para instalar, ejecutar y verificar el runtime `minidocker`.
Incluye los conceptos mínimos para entender cada comando, las pruebas por
hito (con el resultado esperado) y una grilla para la defensa oral.

> Entorno de referencia: Kali Linux y Ubuntu 24.04 (VM o WSL2), kernel
> moderno, cgroups v2, Go ≥ 1.21. Probado en ambos.

---

## 1. Requisitos

| Requisito | Por qué |
|---|---|
| **Linux** | Los namespaces y cgroups son syscalls del kernel Linux. No funcionan nativamente en Windows ni macOS. |
| **Go ≥ 1.21** | El runtime está escrito en Go. Usa `syscall.SysProcAttr.Cloneflags` para pedir los namespaces. |
| **root o sudo** | `CLONE_NEW*` y escribir en `/sys/fs/cgroup` requieren `CAP_SYS_ADMIN`. |
| **cgroups v2** | Jerarquía unificada en `/sys/fs/cgroup`. Kernel ≥ 5.14 típico. |
| **Filesystem nativo de Linux** | El rootfs Alpine tiene symlinks (busybox) que NTFS/vboxsf rompen. **Nunca** uses `/mnt/c` ni shared folders. |

Verificá rápido:
```bash
uname -r                           # kernel
stat /sys/fs/cgroup/cgroup.controllers    # cgroups v2: existe → OK
go version                         # >= go1.21
```

---

## 2. Instalación

```bash
# 1) Clonar el repo
cd ~                               # siempre filesystem nativo (~)
git clone https://github.com/Yerridev/mini-docker
cd mini-docker

# 2) Dar permiso de ejecución al script de rootfs
chmod +x scripts/setup-rootfs.sh

# 3) Compilar el binario
go build -o minidocker ./cmd/minidocker
ls -la minidocker                  # binario Linux x86_64

# 4) Descargar y extraer el rootfs Alpine 3.21 (idempotente)
./scripts/setup-rootfs.sh
ls rootfs/bin/sh                   # debe existir
```

El script es idempotente: si `rootfs/bin/sh` ya existe, sale sin hacer nada.

---

## 3. Verificación de conformidad (rúbrica Sección 3.1)

```bash
go build ./...                     # 0 errores/advertencias
go vet ./...                       # 0 observaciones
gofmt -l .                         # debe dar vacío
go test -race -count=1 ./...       # suite de pruebas con detector de carreras
```

**¿Por qué `-race` es obligatorio?** La rúbrica lo exige cuando hay concurrencia.
`forwardSignals` usa una goroutine + channels + `time.After` para reenviar
señales y forzar SIGKILL tras 3s de gracia. Ese es el escenario clásico donde
el detector de carreras del runtime de Go aporta valor real.

Resultado esperado en Linux:
```
?       minidocker/cmd/minidocker       [no test files]
ok      minidocker/internal/cgroup      1.0xx s
ok      minidocker/internal/config      1.0xx s
ok      minidocker/internal/container   1.0xx s
ok      minidocker/internal/netns       0.0xx s
?       minidocker/internal/namespace   [no test files]

Nota: `signals_linux_test.go` y `netns_linux_test.go` tienen `//go:build linux`
— solo corren en Linux. El primero valida la concurrencia de `forwardSignals`
con `-race`; el segundo valida que `lo` se levanta en un namespace NET nuevo.

---

## 4. Conceptos clave (conciso)

### 4.1 Namespaces

Un namespace es una **vista parcial** del sistema que el kernel le presenta
a un proceso. No es virtualización (sin hipervisor): es el kernel filtrando
qué ve el proceso.

| Namespace | Aísla | Flag de `clone(2)` |
|---|---|---|
| UTS | hostname | `CLONE_NEWUTS` |
| PID | numeración de procesos (el init es PID 1) | `CLONE_NEWPID` |
| MNT | árbol de montajes | `CLONE_NEWMNT` |
| NET | interfaces de red, rutas | `CLONE_NEWNET` |

En el código: `internal/namespace/namespace_linux.go` setea `SysProcAttr.Cloneflags`.

### 4.2 Re-exec pattern (`/proc/self/exe`)

El padre ejecuta `/proc/self/exe` (su propio binario) como **hijo**, con
`MINIDOCKER_INIT=1` en el entorno. El hijo detecta esa variable y entra en
`initContainer` en vez de `runContainer`. ¿Por qué? Porque las flags
`CLONE_NEW*` solo aplican **al nacer** del proceso — no se pueden activar
después. El hijo ya dentro del namespace hace el setup (hostname, mounts,
pivot_root) y al final `syscall.Exec` **reemplaza su imagen** por el comando
del usuario (`/bin/sh`).

Es el patrón estándar que usa `runc` y Docker.

### 4.3 `pivot_root` vs `chroot`

- `chroot` cambia la raíz **aparente**, pero el root viejo sigue accesible
  (escape clásico). Menos seguro.
- `pivot_root(2)` **intercambia** el root viejo por el nuevo y desmonta el
  viejo. No queda vía de escape. Nivel sobresaliente de la rúbrica.

El código intenta `pivot_root` y cae a `chroot` si falla (p. ej. NFS).

### 4.4 cgroups v2

Jerarquía unificada colgando de `/sys/fs/cgroup`. Limitar recursos = escribir
archivos:

| Archivo | Contenido | Efecto |
|---|---|---|
| `cgroup.subtree_control` | `+memory +cpu` | Delegar controladores al padre |
| `memory.max` | bytes | El kernel OOM-kill si se excede |
| `cpu.max` | `"quota period"` (µs) | `50000 100000` = 0.5 CPU |
| `cgroup.procs` | PID | Mueve un proceso al cgroup |
| `cgroup.kill` | `1` | Mata todo el subárbol de una escritura |

Limpiar = `cgroup.kill="1"` + `rmdir` con reintento ante `EBUSY`.

### 4.5 `forwardSignals` (Hito 4)

El init del contenedor es **PID 1 de su namespace**. En Linux, PID 1 **no
recibe SIGTERM por defecto** (no tiene handler instalado como cualquier
proceso). Por eso si mandás Ctrl-C al padre, hay que **reenviar** la señal al
hijo. Si no responde en `killGrace` (3s), se fuerza SIGKILL. Es lo que hace
`docker stop`.

---

## 5. Pruebas por hito

Cada prueba tiene:
- **comando** (bloque bash)
- **resultado esperado** (bloque de salida)
- **por qué** (1-2 líneas)

> Nota: `sudo: unable to resolve host ...` puede aparecer. **Es inofensivo**
> (sudo quejándose de tu hostname). Para silenciarlo:
> `echo "127.0.0.1 $(hostname)" >> /etc/hosts`.

### Hito 1 — namespaces (UTS, PID, MNT)

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```

Adentro del contenedor:
```sh
hostname
```
```
minidocker
```
**Por qué:** `CLONE_NEWUTS` aísla el hostname. `sethostname("minidocker")`
cambió el nombre dentro del namespace sin tocar el host.

```sh
ps aux
```
```
PID   USER     COMMAND
    1 root     /bin/sh
```
**Por qué:** `CLONE_NEWPID` hace que el init sea PID 1 del namespace. `ps`
solo ve procesos nacidos acá.

Salís con `exit`.

### Hito 2 — rootfs propio + `/proc`

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```

Adentro:
```sh
ls /
```
```
bin  dev  etc  home  lib  media  mnt  opt  proc  root  run  sbin  srv  sys  tmp  usr  var
```
**Por qué:** `pivot_root` intercambió el root viejo por el rootfs de Alpine.
Ves Alpine real, no tu `$HOME`.

```sh
cat /etc/os-release
```
```
NAME="Alpine Linux"
VERSION_ID=3.21.0
...
```

```sh
ls /.oldroot
```
```
ls: /.oldroot: No such file or directory
```
**Por qué:** tras `pivot_root` el root viejo se desmonta y se elimina
`/.oldroot`. Si lo viera, habría una vía de escape al host — debe fallar.

### Hito 3 — cgroups

#### Memoria (OOM visible)

```bash
sudo ./minidocker --rootfs rootfs --memory 64m run /bin/sh
```

Adentro:
```sh
yes | head -c 30000000 >/dev/null
```
```
Memory cgroup out of memory: Killed process (tail ...)
```
**Por qué:** `memory.max` quedó en 67108864 bytes (64 MiB). El `yes` llena
memoria; al pasar el límite, el kernel OOM-kill mata el proceso.

En otra terminal (mientras el contenedor corre):
```bash
cat /sys/fs/cgroup/minidocker/c-*/memory.max
```
```
67108864
```
**Por qué:** confirás que el cgroup se creó con el límite pedido.

#### CPU

```bash
sudo ./minidocker --rootfs rootfs --cpu 0.2 run /bin/sh
```

Adentro:
```sh
time (i=0; while [ $i -lt 60000000 ]; do i=$((i+1)); done)
```
```
real    ~18s
```
Compará sin `--cpu` (en otra corrida):
```
real    ~2.3s
```
**Por qué:** `cpu.max` quedó `"20000 100000"` (= 0.2 CPU). El kernel
acota el tiempo de ejecución efectivo a 20% del período de 100 ms. El bucle
puro de CPU se vuelve ~8× más lento.

### Hito 4 — CLI completo

#### Variables de entorno

```bash
sudo ./minidocker --rootfs rootfs --env FOO=bar --env PATH=/bin run /bin/sh
```

Adentro:
```sh
echo $FOO
```
```
bar
```
```sh
printenv PATH
```
```
/bin
```
**Por qué:** `--env` inyecta variables en el entorno del proceso init.
`mergeEnv` da prioridad a `--env` sobre las heredadas, por eso `PATH=/bin`
pisó el PATH del host.

#### Volúmenes bidireccionales

Primero, **en el host** (afuera del contenedor):
```bash
echo "desde-host" > /tmp/archivo.txt
```

Después arrancá el contenedor:
```bash
sudo ./minidocker --rootfs rootfs --volume /tmp:/data run /bin/sh
```

Adentro:
```sh
cat /data/archivo.txt
```
```
desde-host
```
```sh
echo "desde-container" > /data/otro.txt
exit
```

Afuera, verificás:
```bash
cat /tmp/otro.txt
```
```
desde-container
```
**Por qué:** `mountVolumes` hace un `bind mount` **antes del `pivot_root`**
(el origen vive en el host y desaparece al desmontar el root viejo). El bind
mount es bidireccional: lo que escribís en `/data/otro.txt` termina en
`/tmp/otro.txt` del host.

#### Señales (Ctrl-C)

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Pulsá `Ctrl-C` en la terminal del padre.
```
^C
/ # exit
```
**Por qué:** `forwardSignals` atrapó SIGINT (lo que produce Ctrl-C) y lo
reenvió al init del contenedor. Como el init no manejaba SIGTERM/SIGINT, tras
3s de gracia se forzó SIGKILL. Mismo mecanismo que `docker stop`.

> Es **normal** ver `error: exit status 130` al final. 130 = 128 + 2 (muerte
> por SIGINT). Técnicamente correcto (el hijo terminó por señal), aunque en
> producción Docker no lo trataría como error.

### Hito 5 — Red aislada (loopback)

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```

Adentro:
```sh
ip addr 2>/dev/null || cat /proc/net/dev
```
```
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 ...
```
**Por qué:** `CLONE_NEWNET` crea un namespace NET propio y `netns.Setup`
levanta `lo` con `SIOCSIFFLAGS`.

```sh
ping -c 1 127.0.0.1
```
```
PING 127.0.0.1 (127.0.0.1): 56 data bytes
64 bytes from 127.0.0.1: icmp_seq=0 ttl=64 time=0.050 ms
```
**Por qué:** `lo` está UP y enrutado dentro del namespace.

```sh
ping -c 1 <IP_del_host>
```
```
ping: sendto: Network is unreachable
```
**Por qué:** el contenedor no comparte interfaces ni rutas con el host.

```bash
sudo ./minidocker --rootfs rootfs --net none run /bin/sh
```
Adentro:
```sh
cat /proc/net/dev
```
```
Inter-|   Receive ...
 face |bytes    packets ...
    lo:       0       0 ...
```
**Por qué:** `lo` existe (el kernel siempre la crea) pero no está UP; con
`--net none` no se levanta.

---

## 6. Troubleshooting

| Síntoma | Causa | fix |
|---|---|---|
| `sudo: unable to resolve host macOS11` | hostname del host no en `/etc/hosts` | `echo "127.0.0.1 $(hostname)" >> /etc/hosts` |
| `permission denied: ./scripts/setup-rootfs.sh` | script sin bit de ejecución | `chmod +x scripts/setup-rootfs.sh` |
| `[minidocker] pivot_root no disponible (...) usando chroot` | filesystem no soporta `pivot_root` (NFS, tmpfs especial) | **No es error** — fallback automático a `chroot`. Verificá igual el aislamiento. |
| `cgroup: ... no such file or directory` | cgroup colgado de una corrida interrumpida | `sudo rm -rf /sys/fs/cgroup/minidocker/c-*` |
| `fork/exec ... no such file` en la 2da corrida | `/proc` del host se corrompió por falta de `make-rprivate` (ya arreglado en main) | verificá que `setup_linux.go` tenga `makeMountsPrivate` |
| binarios Alpine con `: not found` | rootfs descomprimido en NTFS/shared folder (symlinks rotos) | reposicionar a `~/mini-docker` en filesystem ext4 nativo |

Limpieza manual si quedó algo:
```bash
ls /sys/fs/cgroup/minidocker/          # confirma residuo
sudo rm -rf /sys/fs/cgroup/minidocker/c-*
ps aux | grep minidocker               # zombie
sudo kill -9 <pid>
```

---

## 7. Cobertura de tests

```bash
go test -cover ./internal/config ./internal/cgroup ./internal/container
```

| Paquete | Cobertura | Notas |
|---|---|---|
| `internal/config` | **97%** | `ParseMemory`/`ParseEnv`/`ParseVolumes`/`ParseMode` son funciones puras, fácil de cubrir. |
| `internal/cgroup` | 50% | Unit tests de `FormatCPUMax`/`Contains`/guard `SetMemoryLimit(0)`. El resto toca `/sys` (requiere Linux + root, fuera de reach de unit tests sin privileges). |
| `internal/container` | 17.6% en Windows / **sube en Linux** | Los tests de `forwardSignals` están en `signals_linux_test.go` con `//go:build linux`. En Linux corren y la cobertura sube. |
| `internal/netns` | ~80% en Linux | `ParseMode`/`String` son portables. `setLinkUp` se testea en un namespace NET nuevo con helper-process.

**¿Por qué `-race` solo aporta en Linux para este proyecto?** La única
concurrencia real está en `forwardSignals` (goroutine + channels + signal).
Los tests de esa función están marcados `//go:build linux` porque usan
`exec.Command(os.Args[0], ...)` (helper-process pattern de Go) y señales
Linux.

---

## 8. Defensa oral — grilla de explicación (15-20 min)

> Estructura sugerida: 30-45s por hito + 3 min de Q&A. Total ~12 min, dejá
> margen para la modificación en vivo.

### Por hito

**Hito 1 — Namespaces (UTS, PID, MNT)**
> "El proceso hijo nace con `CLONE_NEWUTS|NEWPID|NEWMNT`. Por eso el hostname
> es `minidocker` y no el del host, `ps aux` solo ve PID 1, y los montajes
> nuevos no se ven afuera. Los namespaces no son virtualización: el kernel le
> muestra al proceso una vista filtrada del sistema."

**Hito 2 — rootfs + `/proc`**
> "Uso `pivot_root(2)` antes que `chroot` porque `pivot_root` desmonta el root
> viejo por completo — no queda `/.oldroot` accesible, sin vía de escape.
> Después de cambiar la raíz monto `/proc` propio dentro del namespace MNT,
> para que `ps` vea solo los procesos del contenedor."

**Hito 3 — cgroups**
> "Creo un cgroup en `/sys/fs/cgroup/minidocker/<id>`, delego `+memory +cpu`
> en el padre, escribo `memory.max` y `cpu.max="quota period"`, y muevo el
> PID del contenedor a `cgroup.procs`. Si excede memoria, el kernel lo OOM-
> kill. Al salir hago `cgroup.kill="1"` + `rmdir` con reintento ante EBUSY."

**Hito 4 — CLI + señales**
> "Los flags `--env` y `--volume` son repetibles como en Docker. `mergeEnv`
> da prioridad a `--env` sobre las heredadas. Los bind mounts se hacen antes
> del `pivot_root` porque el origen vive en el host. Para señales: el init
> del namespace es PID 1 y no recibe SIGTERM por default; por eso tengo
> `forwardSignals` que reenvía SIGINT/SIGTERM al hijo y, si no muere en 3s,
> fuerza SIGKILL — mismo modelo que `docker stop`."

**Hito 5 — Red aislada**
> "Añadí `CLONE_NEWNET` al hijo para que tenga su propia pila de red. Dentro
> del namespace NET el kernel crea un `lo` propio pero está DOWN; lo levanto
> con `SIOCSIFFLAGS` antes de ejecutar el comando del usuario. Con `--net
> none` no se levanta, así que el contenedor queda sin loopback activo."

### Preguntas anticipadas

**¿Por qué `pivot_root` y no solo `chroot`?**
> `chroot` solo cambia la raíz aparente; el root viejo sigue montado y es
> accesible con técnicas conocidas (escape). `pivot_root` intercambia el
> root y desmonta el viejo, sin vía de escape.

**¿Qué pasa si el PID 1 del namespace cuelga?**
> El init no recibe SIGTERM por default (no tiene handler del kernel). Por
> eso `forwardSignals` lo reenvía y, si no responde en 3s (`killGrace`),
> fuerza SIGKILL. El contenedor muere y el `defer cg.Cleanup()` borra el
> cgroup.

**¿Por qué hay que reenviar señales?**
> Ctrl-C llega al **padre** (minidocker) en la terminal. El padre es el
> proceso que recibe SIGINT del terminal. Si el init estuviera en foreground
> podría, pero como está detrás de un exec.Command, el padre debe reenviar.

**¿Qué es `MS_REC|MS_PRIVATE` sobre `/` y por qué importa en systemd?**
> Marca todo el árbol de montajes como privado. En systemd, `/` está como
> `MS_SHARED` por defecto, así que cualquier mount dentro del contenedor
> se propagaría al host y corrompería `/proc` (síntoma: "1era corrida
> funciona, las siguientes fallan con `fork/exec`").

**¿Cómo se eliminan cgroups colgados?**
> `sudo rm -rf /sys/fs/cgroup/minidocker/c-*`. En código, `Cleanup` hace
> `cgroup.kill="1"` (kernel 5.14+) y `rmdir` con retry ante `EBUSY` por el
> delay del kernel al vaciar el cgroup.

### Modificación en vivo sugerida (control de autoría)

Agregar un flag `--hostname <name>` para personalizar el hostname del
contenedor (en vez de hardcoded "minidocker"). Pasos:
1. `cmd/minidocker/main.go`: declarar `hostname := flag.String("hostname", "minidocker", "...")`.
2. Agregar `Hostname string` a `config.Config`.
3. `container.go` pasarlo en `initArgs` (junto a `--rootfs`/`--volume`).
4. `main.go initContainer`: leer el flag y setear `cfg.Hostname`.
5. `setup_linux.go sethostname`: usar `cfg.Hostname` en vez de `"minidocker"` hardcodeado.

Mide comprensión del flujo padre→hijo→setup. Tiempo objetivo: 5-7 min.

---

## 9. Glosario

| Término | Definición de una línea |
|---|---|
| **namespace** | Vista parcial del sistema que el kernel muestra a un proceso (UTS, PID, MNT, NET, USER). |
| **`CLONE_NEW*`** | Flags de `clone(2)` que piden namespaces nuevos para el proceso hijo. |
| **`/proc/self/exe`** | Symlink mágico del kernel al binario en ejecución. Base del patrón re-exec. |
| **re-exec** | Técnica donde el padre ejecuta una copia de sí mismo como hijo (para que el hijo nazca con namespaces). |
| **`pivot_root(2)`** | Syscall que intercambia el root del proceso y desmonta el viejo. Más seguro que `chroot`. |
| **`chroot(2)`** | Syscall que cambia la raíz aparente del proceso. Menos seguro: el root viejo sigue montado. |
| **bind mount** | Montar una ruta del host en otra ruta (usado por `--volume`). |
| **`MS_REC\|MS_PRIVATE`** | Marca el árbol de montajes como privado: los mounts nuevos no se propagan al padre. |
| **cgroup v2** | Jerarquía unificada en `/sys/fs/cgroup` para limitar y medir recursos. |
| **`memory.max`** | Límite de memoria del cgroup. Excederlo dispara OOM kill. |
| **`cpu.max`** | Límite de CPU: `"quota period"` (microsegundos). `50000 100000` = 0.5 CPU. |
| **`cgroup.kill`** | Archivo que mata todo el subárbol del cgroup con una escritura (`1`). |
| **OOM kill** | Out-Of-Memory kill: el kernel mata el proceso que supera `memory.max`. |
| **PID 1** | Primer proceso de un PID namespace. No recibe señales por default (no tiene handler del kernel). |
| **`CLONE_NEWNET`** | Flag de `clone(2)` que crea un namespace de red propio para el hijo. |
| **`SIOCSIFFLAGS`** | ioctl para cambiar los flags de una interfaz de red (usado para poner `lo` UP). |
| **`veth`** | Par de interfaces virtuales que actúan como un cable entre dos namespaces de red. |
| **`forwardSignals`** | Función del runtime que reenvía SIGINT/SIGTERM al PID 1 del contenedor y fuerza SIGKILL tras gracia. |
| **`killGrace`** | Período de gracia (3 s) antes de SIGKILL. Mismo modelo que `docker stop`. |
| **`EBUSY`** | errno que devuelve `rmdir` si el cgroup aún tiene procesos. Motivo del reintento en `Cleanup`. |
| **`syscall.Exec`** | Reemplaza la imagen del proceso actual (no crea uno nuevo). Lo usa el init tras el setup. |
| **`SysProcAttr`** | Struct de Go donde se setea `Cloneflags` para pedir namespaces al crear el hijo. |