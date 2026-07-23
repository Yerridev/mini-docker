# Mini-Docker — Exposición para la defensa oral

Guion para presentar el proyecto en la defensa (15-20 min). Estructurado en
bloques de tiempo con **frases dichas**, **demo en vivo** y **preguntas
anticipadas**. Pensado para leerse de corrido o usarse como guion.

> **Filosofía:** CONCEPTOS > CÓDIGO. La rúbrica valora que puedas explicar
> por qué elegiste cada técnica y que puedas modificar el código en vivo.
> No recites líneas: explicá decisiones.

---

## 0. Antes de empezar (5 min previos)

Llevá abiertas dos terminales en la VM Kali:

- **Terminal 1**: lista para correr el runtime.
- **Terminal 2**: para verificar cgroups desde el host durante la demo.

Preparar el rootfs y compilar (si no está ya):
```bash
cd ~/mini-docker
go build -o minidocker ./cmd/minidocker
./scripts/setup-rootfs.sh    # idempotente

# Verificar que todo compila y los tests pasan
go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...
```

Asegurate de que el output anterior sea **todo verde** antes de arrancar la
exposición. Si algo falla, no arrancar — arreglalo primero.

---

## 1. Introducción (1 minuto)

> **Decir:**
>
> "Mini-Docker es un runtime de contenedores minimalista escrito en Go.
> No es una máquina virtual: es un proceso normal de Linux al que el kernel
> le presenta una vista filtrada del sistema mediante namespaces y le limita
> recursos con cgroups. El objetivo del proyecto es desarmar la caja negra
> de Docker y entender qué hace el kernel por debajo.
>
> Implementé los 5 hitos: aislamiento con namespaces (UTS, PID, MNT),
> rootfs propio con `pivot_root`, límites de recursos con cgroups v2, un CLI
> con variables de entorno, volúmenes y reenvío de señales, y por último
> network namespace con loopback."

Ubicación en el repo:
```bash
cat README.md | head -50      # o abrí el README en el editor
ls internal/                  # muestra la estructura por paquetes
```

---

## 2. Arquitectura general (2 minutos)

> **Decir:**
>
> "La arquitectura sigue el patrón **re-exec**, que es justo lo que hace
> `runc` por debajo de Docker. El proceso padre ejecuta una copia de sí
> mismo como hijo usando `/proc/self/exe`, pasándole `MINIDOCKER_INIT=1`
> en el entorno. El hijo detecta esa variable y entra en el modo init:
> hace el setup de namespaces, rootfs, /proc y red, y al final reemplaza
> su imagen con `syscall.Exec` por el comando del usuario.
>
> ¿Por qué re-exec? Porque las flags `CLONE_NEW*` solo se pueden activar
> **al nacer** del proceso — no se pueden encender después sobre un
> proceso ya corriendo."

**Diagrama textual (mostrar la terminal):**

```
PADRE (minidocker run)
  ├─ parse flags
  ├─ cgroup.New + SetMemoryLimit + SetCPULimit
  ├─ exec.Command("/proc/self/exe", "--rootfs", ..., "init", "/bin/sh")
  │     Cloneflags = NEWUTS|NEWPID|NEWMNT|NEWNET
  │     Env = MINIDOCKER_INIT=1
  │
  └─ hijo (minidocker-init)
        ├─ sethostname ("minidocker")
        ├─ makeMountsPrivate (MS_REC|MS_PRIVATE)
        ├─ mountVolumes (--volume host:cont)
        ├─ prepareRootfs (bind sobre sí)
        ├─ pivotRootfs (fallback chroot)
        ├─ mountProc
        ├─ netns.Setup (--net loopback → lo up)
        └─ syscall.Exec("/bin/sh")     ← reemplaza la imagen, PID sigue 1
```

Código relevante:
```bash
# Re-exec pattern
cat cmd/minidocker/main.go | sed -n '40,50p'     # bifurcación isInit
cat internal/container/container.go | sed -n '32,55p'  # exec.Command del hijo

# syscall.Exec
cat internal/container/exec_linux.go
```

---

## 3. Hito por hito (8-10 minutos)

La idea de esta sección es **decir + demostrar** en paralelo. Por cada
hito: 30-45 segundos de explicación + 30-60 segundos de demo.

### 3.1 Hito 1 — Namespaces (UTS, PID, MNT)

> **Decir:**
>
> "El hijo nace con `CLONE_NEWUTS|NEWPID|NEWMNT`. UTS aísla el hostname,
> PID hace que el init sea PID 1, MNT le da un árbol de montajes propio.
> No es virtualización: el kernel del host es el mismo, solo filtra qué
> ve el proceso."

**Demo:**
```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Dentro (decir mientras ejecutás):
```sh
hostname                              # "minidocker" (no el del host)
ps aux                                # solo PID 1, el /bin/sh
exit
```

### 3.2 Hito 2 — Rootfs propio con `pivot_root`

> **Decir:**
>
> "Usé `pivot_root(2)` en vez de `chroot(2)` porque `pivot_root` no solo
> cambia la raíz aparente, sino que intercambia el root viejo del host
> por el nuevo y lo desmonta — no queda vía de escape. Si `pivot_root`
> falla por filesystem (NFS, tmpfs especial), cae automáticamente a
> `chroot` con aviso. Además, antes de pivot marqué todo el árbol de
> montajes como privado con `MS_REC|MS_PRIVATE`: esto es crítico en
> systemd porque si `/` está como `MS_SHARED`, los mounts del contenedor
> se fugan al host y corrompen `/proc` (síntoma: la primera corrida
> anda, las siguientes fallan con `fork/exec`)."

**Demo:**
```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Dentro:
```sh
ls /                                  # Alpine real
cat /etc/os-release                   # Alpine 3.21.x
ls /.oldroot 2>/dev/null && echo FAIL # nada = bien (no hay escape al host)
mount | grep proc                      # proc propio
exit
```

### 3.3 Hito 3 — cgroups v2

> **Decir:**
>
> "Creo un cgroup en `/sys/fs/cgroup/minidocker/<id>`, delego `+memory +cpu`
> en el `subtree_control` del padre (cgroups v2 lo exige), escribo
> `memory.max` y `cpu.max = 'quota period'` en microsegundos, y muevo
> el PID del contenedor con `cgroup.procs`. Si el proceso supera el
> límite de memoria, el kernel lo mata con OOM. Para limpiar uso
> `cgroup.kill = '1'` (kernel 5.14+) que mata todo el subárbol de una
> escritura, con `rmdir` y reintento ante `EBUSY`."

**Demo memoria (OOM):**
```bash
sudo ./minidocker --rootfs rootfs --memory 64m run /bin/sh
```
Dentro:
```sh
awk 'BEGIN{x="x"; while(1){x=x x; if(length(x)>200000000) exit}}'
# → kernel: "Memory cgroup out of memory: Killed process (awk)"
```

**En la otra terminal (host)** mientras corre:
```bash
cat /sys/fs/cgroup/minidocker/c-*/memory.max    # 67108864 (64 MiB)
```

**Demo CPU** — comparar con y sin límite:
```bash
sudo ./minidocker --rootfs rootfs --cpu 0.2 run /bin/sh
# dentro:
time sh -c 'i=0; while [ $i -lt 60000000 ]; do i=$((i+1)); done'
# → ~18s

exit
sudo ./minidocker --rootfs rootfs run /bin/sh
# dentro:
time sh -c 'i=0; while [ $i -lt 60000000 ]; do i=$((i+1)); done'
# → ~2.3s    (~8x más rápido — el límite funciona)
exit
```

### 3.4 Hito 4 — CLI completo

> **Decir:**
>
> "Los flags `--env` y `--volume` son repetibles como en Docker. Para
> variables de entorno, `mergeEnv` da prioridad a `--env` sobre las
> heredadas — por eso puedo sobrescribir `PATH`. Los bind mounts se hacen
> **antes** del `pivot_root` porque el origen vive en el host y
> desaparece cuando desmonto el root viejo. Y para señales: el init del
> contenedor es PID 1 de su namespace, y PID 1 **no recibe SIGTERM por
> default**. Por eso tengo `forwardSignals`, una goroutine que reenvía
> SIGINT/SIGTERM al hijo y, si no responde en 3 segundos, fuerza SIGKILL
> — mismo modelo que `docker stop`."

**Demo env + volumen:**
```bash
# En el host
echo "desde-host" > /tmp/archivo.txt

sudo ./minidocker --rootfs rootfs \
    --env FOO=bar --env PATH=/bin \
    --volume /tmp:/data \
    run /bin/sh
```
Dentro:
```sh
echo $FOO                             # bar
printenv PATH                         # /bin  (pisado)
cat /data/archivo.txt                 # desde-host
echo "desde-container" > /data/otro.txt
exit
```
En el host:
```bash
cat /tmp/otro.txt                     # "desde-container" (bidireccional)
```

**Demo señales:**
```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
# Ctrl-C → termina en ~3s
```

### 3.5 Hito 5 — Networking (opcional)

> **Decir:**
>
> "El último hito añade el namespace de red (`CLONE_NEWNET`) con loopback.
> Dentro del namespace NET nuevo, la interfaz `lo` existe pero está DOWN
> — hay que levantarla explícitamente con un ioctl `SIOCSIFFLAGS` o con
> `ip link set lo up` como fallback. El flag `--net` controla el modo,
> por default es `loopback`. Con `--net none` no se levanta nada.
> Bonus: `--hostname` permite personalizar el hostname del contenedor en
> vez del default `minidocker`."

**Demo básico:**
```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Dentro:
```sh
cat /proc/net/dev                     # lo existe y está UP
ping -c 1 127.0.0.1                   # responde
exit
```

**Demo hostname + aislamiento de red:**
```bash
# Host: ver tu IP
ip addr | grep "inet " | grep -v 127.0.0.1
# → 192.168.x.x o 10.0.0.x

sudo ./minidocker --rootfs rootfs --hostname demo run /bin/sh
```
Dentro:
```sh
hostname                              # demo
ping -c 1 <IP_DEL_HOST>               # falla (network unreachable)
exit
```

---

## 4. Tests (1 minuto)

> **Decir:**
>
> "La suite de pruebas cubre los parsers (`ParseMemory`, `ParseEnv`,
> `ParseVolumes` con 18 casos), utilidades de cgroup (`FormatCPUMax`,
> `Contains`), `mergeEnv` con y sin override, y `forwardSignals` con el
> detector de carreras. Los tests de señales usan el patrón helper-process
> — re-ejecutan el binario de tests como proceso hijo con
> `GO_HELPER_PROCESS=1` — para no depender de `/bin/sleep` que no existe
> en todos los SOs. Y los tests Linux-only están marcados con
> `//go:build linux`."

**Demo:**
```bash
go test -race -count=1 ./...
go test -cover ./internal/config ./internal/cgroup ./internal/container ./internal/netns
```

> Resultado: `go test -race` en verde. Coverage: config 97%, cgroup 50%,
> container sube en Linux (corren signals_linux_test.go), netns sube en
> Linux (corren netns_linux_test.go).

---

## 5. Decisiones de diseño (2 minutos)

> **Decir:**
>
> "Algunas decisiones relevantes y por qué las tomé:
>
> 1. **`pivot_root` antes que `chroot`**: más seguro. `chroot` cambia la
>    raíz aparente pero el root viejo sigue montado y accesible. `pivot_root`
>    intercambia y desmonta. Fallback a `chroot` solo si `pivot_root` falla
>    por filesystem.
>
> 2. **`MS_REC|MS_PRIVATE` sobre `/` antes de cualquier mount**: sin esto,
>    en systemd (`/ = MS_SHARED`), los mounts del contenedor se propagan al
>    host y corrompen `/proc` (síntoma que costó aislar: 'la 1era corrida
>    anda, las siguientes fallan con `fork/exec`).
>
> 3. **`cgroup.kill '1'` con reintento `EBUSY`**: el kernel tarda milisegundos
>    en vaciar el cgroup tras matar los procesos. Reintentar `rmdir` 100
>    veces con sleep de 10ms evita dejar residuos.
>
> 4. **Re-exec pattern (`/proc/self/exe`)**: las flags `CLONE_NEW*` solo
>    aplican al nacer del proceso — no se pueden activar después. Por eso
>    el padre ejecuta una copia de sí mismo como hijo, no hay otra forma
>    idiomática en Go.
>
> 5. **Separación por build tags** (`*_linux.go` + `*_other.go`): permite
>    `go build ./...` y `go test ./...` en Windows/macOS con stubs no-op.
>    Mantiene el dominio portable y la implementación Linux-only separada."

---

## 6. Preguntas anticipadas

El docente puede preguntar cualquiera de estas. Las respuestas son de 1-2
líneas, no te las aprendas — **entendé el por qué**.

### ¿Por qué `pivot_root` y no solo `chroot`?
> `chroot` solo cambia la raíz aparente; el root viejo sigue montado y es
> accesible con técnicas conocidas (escape de chroot). `pivot_root`
> intercambia el root y desmonta el viejo — no queda vía de escape.

### ¿Qué pasa si el PID 1 del namespace cuelga?
> El init no recibe SIGTERM por default (no tiene handler del kernel).
> Eso es justo por lo que `forwardSignals` existe — reenvía SIGINT/SIGTERM
> al hijo y, si no responde en `killGrace` (3s), fuerza SIGKILL. El
> contenedor muere y el `defer cg.Cleanup()` borra el cgroup.

### ¿Por qué hay que reenviar señales?
> Ctrl-C llega al padre (minidocker) en la terminal — es ese el proceso que
> recibe SIGINT del terminal. Si el init estuviera en foreground podría
> recibirla directamente, pero como está detrás de un `exec.Command`, el
> padre debe reenviar.

### ¿Qué es `MS_REC|MS_PRIVATE` sobre `/` y por qué importa en systemd?
> Marca todo el árbol de montajes como privado: los mounts nuevos no se
> propagan al padre. En systemd, `/` está como `MS_SHARED` por defecto, así
> que sin esto los mounts del contenedor se fugarían al host y corromperían
> `/proc` del host (rompiendo `fork/exec /proc/self/exe` en corridas
> subsiguientes).

### ¿Cómo se eliminan cgroups colgados?
> En código, `Cleanup` hace `cgroup.kill = '1'` (kernel 5.14+) y `rmdir`
> con reintento ante `EBUSY`. A mano: `sudo rm -rf
> /sys/fs/cgroup/minidocker/c-*`.

### ¿Cómo se levanta `lo` dentro del namespace NET?
> Existe pero está DOWN por default. Hay que ponerlo UP con un ioctl
> `SIOCSIFFLAGS` (setear flags `IFF_UP|IFF_RUNNING`) o con fallback
> `ip link set lo up`. Sin eso, `ping 127.0.0.1` falla.

### ¿Qué diferencia `cmd.Start()` de `cmd.Run()`?
> `Run = Start + Wait`. El runtime usa `Start` para obtener el PID y
> moverlo al cgroup **antes** de esperar a que termine. Con `Run`
> perderías ese hook.

### ¿Qué hace `fmt.Errorf` con `%w`?
> Envuelve un error existente agregando contexto, preservando la cadena
> para `errors.Is`/`errors.As` en capas superiores. `%v` solo agrega texto
> y corta la cadena.

### ¿Por qué hay archivos `*_linux.go` y `*_other.go`?
> Los syscalls de namespaces/cgroups solo existen en Linux. Con build tags
> `//go:build linux` y `//go:build !linux`, en Linux compila la
> implementación real y en Windows/macOS compila un stub no-op, manteniendo
> `go build ./...` portable.

### ¿Por qué `/proc/self/exe` y no un path al binario?
> `/proc/self/exe` es un symlink mágico del kernel al binario en
> ejecución. Permite al padre relanzarse a sí mismo como hijo sin
> importar cómo se haya invocado o desde dónde — es el patrón estándar
> de `runc`.

### ¿Qué es `syscall.Exec` y en qué se diferencia de `exec.Command`?
> `syscall.Exec` **reemplaza** la imagen del proceso actual (el PID no
> cambia). `exec.Command` **crea** un proceso nuevo. El runtime usa los
> dos: el padre hace `exec.Command` para crear el hijo, el hijo hace
> `syscall.Exec` para reemplazarse por `/bin/sh`.

---

## 7. Modificación en vivo (5-7 minutos)

El docente va a pedirte un cambio chico. Una buena opción que muestra
comprensión del flujo **padre → hijo → setup** es añadir un flag
`--.hostname` (ya está implementado en Hito 5, así que conseguí otro
similar).

Otras modificaciones posibles (no todas pedibles, pero practicable):

- **`--user U:G`** — mapear UID/GID dentro del contenedor con
  `CLONE_NEWUSER`. Hay que añadir `uid_map`/`gid_map` en `/proc/<pid>/`.
- **`--hostname <name>`** (si no estuviera) — cambiar el hardcoded
  `"minidocker"` en `setup_linux.go` por una variable. Pasar del flag →
  `config.Config` → reenviar al hijo → usar en `sethostname`.
- **`--ulimit N`** — limitar número de procesos del contenedor con
  `pids.max` en el cgroup (similar a `memory.max`, solo controlador
  distinto).
- **`SIGUSR1` para dump de estado** — añadir handoff en
  `forwardSignals` que ante `SIGUSR1` imprima los PIDs hijos en el
  cgroup. Requiere `signal.Notify` con la señal extra.

**Ejemplo planteado** (si te piden `--ulimit N`):

1. `internal/config/config.go`: añadir `MaxPIDs int64`.
2. `cmd/minidocker/main.go`: flag `--ulimit <n>` (int), mapea a
   `cfg.MaxPIDs`.
3. `internal/cgroup/cgroup_linux.go`: nuevo método
   `SetPIDsLimit(n int64)` que escribe `pids.max` con valor `n`.
4. `internal/container/container.go`: llamarlo junto a
   `SetMemoryLimit`/`SetCPULimit` (colgando del `if
   c.Config.MaxPIDs > 0`).
5. Test: verificar que el archivo se crea con el contenido esperado
   (no hace falta forzar fork-bomb en test; basta check de formato).

Tiempo objetivo: 5-7 min. Si lo hacés en 10 con tests, te sobra.

---

## 8. Cierre (30 segundos)

> **Decir:**
>
> "En resumen, mini-docker es un runtime completo que aísla procesos con
> scripts del kernel Linux — namespaces y cgroups — y los expone
> idiomáticamente desde Go. Implementé los 5 hitos: namespaces, rootfs
> con pivot_root, cgroups v2, CLI con env/volúmenes/señales, y networking
> con loopback. La suite de tests pasa con `-race` y el código compila
> limpio. Usé asistida de IA para acelerar partes (la declaración está en
> el Anexo B), pero todas las decisiones de diseño son mías y las puedo
> defender. Gracias."

---

## 9. Checklist final (antes de presentarte)

- [ ] VM Kali arrancada, root, red conectada.
- [ ] `cd ~/mini-docker && git pull origin main`.
- [ ] `go build -o minidocker ./cmd/minidocker`.
- [ ] `./scripts/setup-rootfs.sh` (idempotente).
- [ ] `go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...` todo verde.
- [ ] Terminal 1 lista para runtime; **Terminal 2 abierta** para ver cgroups.
- [ ] `echo "desde-host" > /tmp/archivo.txt` listo para demo de volumen.
- [ ] `ip addr` anotada en un papel (tu IP del host) para demo de aislamiento.
- [ ] Anexo A (bitácora) commiteado y accesible.
- [ ] Anexo B (declaración IA) commiteado (si lo tenés).
- [ ] `docs/comandos-prueba.md` abierto en el editor por si Continental.
- [ ] Reposar el agua. Respirar hondo.

---

## 10. Si se te complicate (salidas de emergencia)

| Síntoma durante la demo | Qué decir / qué hacer |
|---|---|
| Falla `pivot_root` (chroot fallback) | "Se cae al fallback de chroot, que es lo esperado en filesystems como NFS. El aislamiento sigue funcionando, solo menos estricto." |
| `fork/exec /proc/self/exe: no such file` | Detené, decí: "Esto pasa si /proc del host se corrompió. Reinicio el shell y reintentamos — no es un bug del runtime." |
| OOM no dispara | "Hay que usar algo que ALOQUE memoria, no streaming por pipe." Y pasá al awk. |
| `ping 127.0.0.1` falla | Verificá que `--net` no esté en `none`. Si está en `loopback`, decí: "El ioctl no levantó `lo`, muestro el fallback con `ip link set lo up`." |
| Docente pregunta algo que no sabés | Decí: "Lo investigo, no quiero inventar." Es mejor que confesar especulando. |

> **Principio rector:** la rúbrica no premia la perfección, premia la
> autoría demostrable. Si no sabés algo, admitilo. Si sabés la mitad,
> explicá la mitad y reconocé el límite. Eso pesa más que la respuesta
> completa repetida de memoria.