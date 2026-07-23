# Mini-Docker — Comandos de prueba (VM)

Lista rápida de **todos los comandos** para probar el runtime completo (Hitos
1-5) en una VM Linux. Copiá y pegá. Cada bloque se autoexplica.

> Requisito previo: root, Go ≥ 1.21, cgroups v2, filesystem nativo (no NTFS).
> Si todavía no clonaste el repo o preparaste el rootfs, mirá la sección 0.

---

## 0. Setup (una sola vez)

```bash
# Clonar y compilar
cd ~
git clone https://github.com/Yerridev/mini-docker
cd mini-docker
go build -o minidocker ./cmd/minidocker

# Descargar rootfs Alpine 3.21 (idempotente)
chmod +x scripts/setup-rootfs.sh
./scripts/setup-rootfs.sh
ls rootfs/bin/sh      # debe existir

# Verificación de conformidad rúbrica (Sección 3.1)
go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...
# ↑ todo debe dar vacío / "ok" — si algo falla, no seguir

# Bit de ejecución al binario
chmod +x minidocker
```

---

## 1. Hito 1 — Namespaces (UTS, PID, MNT)

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```

Dentro del contenedor:
```sh
hostname                              # → minidocker  (UTS aislado)
ps aux                                # → solo PID 1 = /bin/sh  (PID aislado)
mount | head -5                        # → mounts propios, no los del host
exit
```

**Otra terminal (host)** durante la corrida:
```bash
ps aux | grep minidocker              # → ves dos procesos (padre + init)
hostname                               # → sigue siendo el del host, no "minidocker"
```

---

## 2. Hito 2 — Rootfs propio + /proc

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```

Dentro:
```sh
ls /                                  # → bin etc home ... usr var (Alpine real)
cat /etc/os-release                   # → Alpine 3.21.x
ls /.oldroot 2>/dev/null && echo FAIL # → "FAIL" = mal; nada = bien (pivot_root limpió)
mount | grep proc                      # → proc en /proc, propio del contenedor
exit
```

---

## 3. Hito 3 — Cgroups (memoria y CPU)

### 3.1 Memoria — forzar OOM kill

```bash
sudo ./minidocker --rootfs rootfs --memory 64m run /bin/sh
```

Dentro:
```sh
awk 'BEGIN{x="x"; while(1){x=x x; if(length(x)>200000000) exit}}'
# → kernel: "Memory cgroup out of memory: Killed process (awk)"
```

**Otra terminal (host)** mientras corre:
```bash
cat /sys/fs/cgroup/minidocker/c-*/memory.max    # → 67108864 (64 MiB)
```

> El check de `memory.max` se hace **desde el host**: tras `pivot_root` el
> contenedor no ve `/sys/fs/cgroup` del host.

### 3.2 CPU — limitar y comparar

Con límite (0.2 CPU = 20%):
```bash
sudo ./minidocker --rootfs rootfs --cpu 0.2 run /bin/sh
```
Dentro:
```sh
time sh -c 'i=0; while [ $i -lt 60000000 ]; do i=$((i+1)); done'
# → real  ~18s
```

Sin límite (comparativa):
```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Dentro:
```sh
time sh -c 'i=0; while [ $i -lt 60000000 ]; do i=$((i+1)); done'
# → real  ~2.3s    (~8x más rápido que con --cpu 0.2)
```

---

## 4. Hito 4 — CLI (env, volúmenes, señales)

### 4.1 Variables de entorno

```bash
sudo ./minidocker --rootfs rootfs --env FOO=bar --env PATH=/bin run /bin/sh
```
Dentro:
```sh
echo $FOO                             # → bar
printenv PATH                         # → /bin  (pisó el PATH heredado)
exit
```

### 4.2 Volúmenes bidireccionales

Preparar en el host:
```bash
echo "desde-host" > /tmp/archivo.txt
```

Correr:
```bash
sudo ./minidocker --rootfs rootfs --volume /tmp:/data run /bin/sh
```
Dentro:
```sh
cat /data/archivo.txt                 # → desde-host  (lectura host→contenedor)
echo "desde-container" > /data/otro.txt
exit
```

Verificar en host:
```bash
cat /tmp/otro.txt                     # → desde-container  (escritura cont→host)
```

### 4.3 Señales (Ctrl-C)

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Pulsá **Ctrl-C** → el contenedor termina en ~3s (grace period de `forwardSignals`).

> `error: exit status 130` al salir es **esperado** (130 = 128 + SIGINT). No es un bug.

---

## 5. Hito 5 — Networking (loopback, opcional)

Por default el flag `--net loopback` viene activo. Para verlo:

```bash
sudo ./minidocker --rootfs rootfs run /bin/sh
```
Dentro:
```sh
ip addr 2>/dev/null || cat /proc/net/dev   # → "lo" existe y está UP
ping -c 1 127.0.0.1                         # → responde (loopback ok)
exit
```

### 5.1 Sin red (`--net none`)

```bash
sudo ./minidocker --rootfs rootfs --net none run /bin/sh
```
Dentro:
```sh
cat /proc/net/dev                          # → "lo" existe pero DOWN (sin UP)
ping -c 1 127.0.0.1 2>&1 | head -2         # → "Network is unreachable" o similar
exit
```

### 5.2 Aislamiento de red (contenedor no ve al host)

Con `--net loopback` (default):
```bash
# En el HOST, buscá tu IP:
ip addr | grep "inet " | grep -v 127.0.0.1
# → algo como 192.168.x.x o 10.0.0.x

sudo ./minidocker --rootfs rootfs run /bin/sh
```
Dentro:
```sh
ping -c 1 <IP_DEL_HOST>                    # → falla (network unreachable)
# El contenedor SOLO tiene loopback, no puede alcanzar al host ni a internet
exit
```

### 5.3 Hostname personalizado (nuevo flag Hito 5)

```bash
sudo ./minidocker --rootfs rootfs --hostname mi-contenedor run /bin/sh
```
Dentro:
```sh
hostname                                  # → mi-contenedor  (no "minidocker")
exit
```

---

## 6. Integración completa (todo junto)

Un comando que ejerce **los 5 hitos** a la vez:

```bash
echo "desde-host" > /tmp/archivo.txt

sudo ./minidocker --rootfs rootfs \
    --memory 128m \
    --cpu 1.0 \
    --env FOO=bar \
    --env PATH=/bin \
    --volume /tmp:/data \
    --hostname demo \
    --net loopback \
    run /bin/sh
```

Dentro:
```sh
hostname                                  # → demo          (H1 + flag H5)
ps aux                                    # → solo PID 1    (H1)
ls /                                      # → Alpine        (H2)
echo $FOO                                 # → bar           (H4 env)
cat /data/archivo.txt                     # → desde-host    (H4 volume)
ip addr 2>/dev/null || cat /proc/net/dev  # → lo UP         (H5)
ping -c 1 127.0.0.1                       # → responde      (H5)
exit
```

Verificación de límites desde el host (en otra terminal durante la corrida):
```bash
cat /sys/fs/cgroup/minidocker/c-*/memory.max    # → 134217728 (128 MiB)
cat /sys/fs/cgroup/minidocker/c-*/cpu.max      # → "100000 100000" (1 CPU)
```

---

## 7. Limpieza (si algo se cuelga)

```bash
# Verificar cgroups colgados
ls /sys/fs/cgroup/minidocker/

# Limpiar cgroups colgados
sudo rm -rf /sys/fs/cgroup/minidocker/c-*

# Matar procesos minidocker zombie
ps aux | grep minidocker
sudo kill -9 <pid>

# Rastrillo final (solo si fue muy mal)
sudo rm -rf /sys/fs/cgroup/minidocker/
```

---

## 8. Tests automatizados (rúbrica)

```bash
# Sin detector de races (rápido)
go test ./...

# Con detector de races — OBLIGATORIO (rúbrica Sección 3.1)
go test -race -count=1 ./...

# Cobertura por paquete
go test -cover ./internal/config ./internal/cgroup ./internal/container ./internal/netns

# Correr un paquete a la fois, verbose
go test -v -race ./internal/container/

# Correr un solo test
go test -run TestFormatCPUMax -v ./internal/cgroup/
```

Resultado esperado de `go test -race -count=1 ./...` en Linux:
```
?       minidocker/cmd/minidocker       [no test files]
ok      minidocker/internal/cgroup      1.x s
ok      minidocker/internal/config      1.x s
ok      minidocker/internal/container   1.x s
?       minidocker/internal/namespace   [no test files]
ok      minidocker/internal/netns       1.x s
```

---

## 9. Troubleshooting rápido

| Síntoma | Fix |
|---|---|
| `sudo: unable to resolve host ...` | `echo "127.0.0.1 $(hostname)" >> /etc/hosts` |
| `permission denied: ./scripts/setup-rootfs.sh` | `chmod +x scripts/setup-rootfs.sh` |
| `syntax error: unexpected word (expecting ")")` con `time` | usar `time sh -c '...'` (busybox no traga subshell tras `time`) |
| OOM no se dispara con `yes \| head -c 30M` | usar `awk 'BEGIN{x="x"; while(1){x=x x; if(length(x)>200000000) exit}}'` |
| `cat /sys/fs/cgroup/...` falla **adentro** del contenedor | verificar desde **el HOST** en otra terminal |
| `pivot_root no disponible (...) usando chroot` | **No es error**, fallback automático. Verificá igual el aislamiento. |
| binarios Alpine `: not found` | rootfs descomprimido en NTFS — reposicionar a `~/mini-docker` ext4 nativo |

---

## 10. Resumen de flags

```bash
./minidocker [flags] run <comando> [args...]
```

| Flag | Default | Valores | Hito |
|---|---|---|---|
| `--rootfs <path>` | `./rootfs` | directorio con Alpine | 1-2 |
| `--memory <n>` | (sin límite) | `64m`, `1g`, bytes | 3 |
| `--cpu <n>` | `0` (sin límite) | `0.5`, `2.0` (núcleos) | 3 |
| `--env K=V` | (vacío) | repetible | 4 |
| `--volume /h:/c` | (vacío) | repetible | 4 |
| `--hostname <name>` | `minidocker` | string | 5 (flag custom) |
| `--net <mode>` | `loopback` | `loopback`, `none`, `veth` | 5 |

Para el modo `veth` (conectividad host↔contenedor) mirá
`docs/hito-5-networking.md` o el código en `internal/netns/netns_linux.go`.