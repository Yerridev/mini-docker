# Anexo A — Bitácora de decisiones de diseño

**Proyecto:** mini-docker
**Estudiante:** `<TU_NOMBRE>`
**Fecha:** `<FECHA>`

## Cómo leer esta bitácora

Este documento registra las decisiones de diseño más relevantes del proyecto
mini-docker, un runtime de contenedores minimalista en Go. Cada sección
corresponde a un hito de desarrollo y contiene las alternativas evaluadas,
la decisión final y su justificación técnica.

---

## Hito 1 — Aislamiento con namespaces (UTS, PID, MNT)

### Decisión 1.1 — Crear el proceso hijo con `SysProcAttr.Cloneflags`

**Contexto:** El patrón re-exec requiere que el padre lance un hijo dentro de
namespaces aislados. En Go, `os/exec` no expone flags de namespaces
directamente; hay que usar `syscall.SysProcAttr` para setear `Cloneflags`.

**Alternativas consideradas:**
- `syscall.Clone` directamente (bajo nivel, control total).
- `SysProcAttr.Cloneflags` con `exec.Command` (integración nativa, pipes
  automáticos).

**Decisión tomada:** `SysProcAttr.Cloneflags` con `exec.Command("/proc/self/exe")`.

**Justificación:** `exec.Command` maneja stdin/stdout/stderr y `.Wait()`
automáticamente. El patrón re-exec (`/proc/self/exe`) permite que el
mismo binario sirva como padre y como init del contenedor.

### Decisión 1.2 — Solo UTS, PID y MNT (sin NET ni USER)

**Contexto:** Linux ofrece siete namespaces. Para un runtime minimalista
había que decidir cuáles incluir.

**Alternativas consideradas:**
- Los 7 namespaces completos.
- Solo PID y MNT (mínimo estricto).
- UTS + PID + MNT (balance aislamiento/complejidad).

**Decisión tomada:** UTS + PID + MNT.

**Justificación:** UTS permite hostname independiente. PID aísla el árbol
de procesos (el hijo es PID 1). MNT es requisito para `pivot_root`. NET y
USER agregan complejidad significativa sin impacto pedagógico proporcional.

---

## Hito 2 — Rootfs propio (pivot_root / chroot)

### Decisión 2.1 — `pivot_root` primario con fallback a `chroot`

**Contexto:** El contenedor necesita su propio filesystem raíz. Las opciones
son `chroot(2)` (simple pero con fugas) y `pivot_root(2)` (sin vías de escape).

**Alternativas consideradas:**
- Solo `chroot` (trivial, pero root del host queda accesible).
- Solo `pivot_root` (seguro, pero puede fallar en NFS/rootfs inválidos).
- `pivot_root` con fallback a `chroot`.

**Decisión tomada:** `pivot_root` primario con fallback a `chroot`.

**Justificación:** `pivot_root` desmonta el root viejo (`/.oldroot` se
unmount y elimina), sin vías de escape. El fallback cubre filesystems que
no soportan `pivot_root`, manteniendo funcionalidad sin romper Hito 1.

### Decisión 2.2 — `mount --make-rprivate /` antes de `pivot_root`

**Contexto:** En hosts con systemd, `/` tiene propagación `MS_SHARED`. Sin
cambiar a `MS_PRIVATE`, los montajes del contenedor se propagan al host.

**Alternativas consideradas:**
- No hacer nada (asumir host con propagación privada).
- `MS_PRIVATE` solo en el rootfs.
- `MS_REC|MS_PRIVATE` en `/` (recursivo, todo el árbol).

**Decisión tomada:** `MS_REC|MS_PRIVATE` en `/` antes de cualquier mount.

**Justificación:** `MS_REC` es crucial: sin recursivo, sub-mounts de
systemd mantienen `shared`. Esto causa el bug "primera corrida funciona,
las siguientes fallan" porque `/proc` del host se corrompe.

### Decisión 2.3 — `/proc` se monta DESPUÉS del cambio de raíz

**Contexto:** `/proc` debe reflejar el PID namespace del contenedor, no el
del host. Si se monta antes de `pivot_root`, muestra procesos del host.

**Alternativas consideradas:**
- Montar antes de `pivot_root` (incorrecto).
- Montar después del cambio de raíz.

**Decisión tomada:** Montar `/proc` después de `pivot_root`.

**Justificación:** Después del cambio, el proceso vive en su nuevo rootfs.
`/proc` refleja solo procesos del contenedor; `ps aux` muestra `/bin/sh`
como PID 1, no el PID real del host.

### Decisión 2.4 — Script `setup-rootfs.sh` con Alpine minirootfs

**Contexto:** Testing y demostración requieren un rootfs mínimo con `sh`,
`ls`, `ps`.

**Alternativas consideradas:**
- Rootfs manual (tedioso).
- Distro completa (demasiado grande).
- Alpine minirootfs (~3MB, busybox).

**Decisión tomada:** Alpine minirootfs 3.21 via `setup-rootfs.sh`.

**Justificación:** Alpine minirootfs es estándar para contenedores
minimalistas. Busybox incluye herramientas necesarias. El script automatiza
descarga y extracción.

### Decisión 2.5 — Bugfix: reenviar `--rootfs` al proceso init

**Contexto:** El hijo heredaba rootfs por defecto (`./rootfs`). Si el
usuario especificaba otro path, el hijo no lo recibía.

**Alternativas consideradas:**
- Variable de entorno para pasar rootfs.
- Reenviar `--rootfs` como argumento CLI.

**Decisión tomada:** Reenviar `--rootfs` como flag, normalizado a ruta
absoluta en el padre.

**Justificación:** La normalización es crítica: el hijo cambia de cwd
durante `pivot_root`, y un path relativo dejaría de funcionar.

---

## Hito 3 — Cgroups v2 (memoria y CPU)

### Decisión 3.1 — Paquete `internal/cgroup/` con split por SO

**Contexto:** Cgroups son específicos de Linux. El código debe compilarse
solo en Linux con stubs en otras plataformas.

**Alternativas consideradas:**
- Todo en un solo archivo con `//go:build linux`.
- Split en `cgroup.go` (común) + `cgroup_linux.go` + `cgroup_other.go`.

**Decisión tomada:** Split en tres archivos.

**Justificación:** Funciones como `FormatCPUMax` y `Contains` no dependen
de Linux; testearlas en Windows/CI es valioso. Cumple Sección 3.1 (`go
test -race` verde en todas las plataformas).

### Decisión 3.2 — Delegar controladores en `cgroup.subtree_control`

**Contexto:** En cgroups v2, un controlador solo puede usarse si está
habilitado en `cgroup.subtree_control` del padre.

**Alternativas consideradas:**
- Asumir controladores habilitados (rompe en systemd).
- Habilitar solo en el padre.
- Delegación recursiva: raíz → nivel intermedio → hijo.

**Decisión tomada:** Delegación recursiva: `delegateControllers` en raíz
y nivel intermedio `minidocker/`.

**Justificación:** Systemd puede no tener `memory`/`cpu` habilitados. La
delegación en dos niveles garantiza que el cgroup hijo funcione sin
importar la configuración del host.

### Decisión 3.3 — `cgroup.kill "1"` con fallback SIGKILL manual

**Contexto:** Al eliminar un cgroup, los procesos deben morir primero.
Kernels 5.14+ tienen `cgroup.kill`; antiguos requieren SIGKILL manual.

**Alternativas consideradas:**
- Solo SIGKILL manual (lento, funciona siempre).
- Solo `cgroup.kill` (rápido, kernels < 5.14 no).
- `cgroup.kill` con fallback SIGKILL.

**Decisión tomada:** `cgroup.kill` primario con fallback SIGKILL.

**Justificación:** `cgroup.kill` es atómico y rápido. El fallback cubre
kernels antiguos. `removeWithRetry` reintenta `rmdir` ante `EBUSY` (el
kernel tarda ms en vaciar el cgroup).

### Decisión 3.4 — Fix de portabilidad: `MS_PRIVATE` antes de `/proc`

**Contexto:** Corrección del Hito 2 que impactó Hito 3. Sin `MS_PRIVATE`,
el mount de `/proc` se propaga al host en systemd.

**Alternativas consideradas:**
- Documentar "solo funciona sin systemd".
- Aplicar `MS_PRIVATE` siempre.

**Decisión tomada:** `MS_PRIVATE` siempre, antes de montar `/proc`.

**Justificación:** Fix crítico de portabilidad. El síntoma ("primera
corrida funciona, siguientes fallan") es difícil de diagnosticar. `MS_PRIVATE`
universal previene el problema sin efectos colaterales.

### Decisión 3.5 — Flags `--memory` y `--cpu` con parsing flexible

**Contexto:** Límites de recursos con sintaxis amigable y compatible con
Docker.

**Alternativas consideradas:**
- Solo bytes y cores decimales (mínimo).
- Sufijos (`k`, `m`, `g`) + cores decimales.

**Decisión tomada:** `--memory` acepta sufijos y bytes crudos; `--cpu`
acepta núcleos decimales (`0.5` = 50% de un core).

**Justificación:** Sufijos son convención Docker. Parsing extraído a
`config.ParseMemory` para testeabilidad (18 casos cubren edge cases).
