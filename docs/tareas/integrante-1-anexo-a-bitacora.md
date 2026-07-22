# Tarea — Integrante 1: Anexo A, Bitácora de decisiones

> **Instruido a:** un LLM que va a redactar un documento. No debe escribir
> código: solo producir `docs/anexo-a-bitacora.md`.

---

## Rol

Sos un asistente que redacta documentación técnica académica en español
(Rioplatense neutro). Estás ayudando a un estudiante a completar un
entregable obligatorio de la rúbrica de un taller de Go: la **bitácora de
decisiones de diseño** (Anexo A de la guía del curso).

## Por qué existe esta tarea

La rúbrica (Sección 2.3, "Controles de verificación de autoría") exige un
documento breve, por hito, que registre las decisiones de diseño relevantes
y su justificación. **Debe reflejar el razonamiento propio del estudiante.**
Es el control de mayor peso: sin esto, la nota se capped en "En desarrollo"
sin importar la calidad del código.

## Contexto del proyecto

`mini-docker` es un runtime de contenedores minimalista en Go que ejecuta
un comando dentro de namespaces Linux aislados (UTS, PID, MNT) con rootfs
propio (`pivot_root`/`chroot`), `/proc` independiente, límites de recursos
vía cgroups v2 y un CLI con `--env`, `--volume` y reenvío de señales.

Repositorio: `https://github.com/Yerridev/mini-docker`
Rama: `main` (HEAD actual: `2e3d38a`).

Arquitectura: patrón **re-exec** — el padre ejecuta `/proc/self/exe` como
hijo con `MINIDOCKER_INIT=1` y flags `CLONE_NEWUTS|NEWPID|NEWMNT` vía
`syscall.SysProcAttr.Cloneflags`. El hijo detecta la env var, entra en
`initContainer`, hace el setup (hostname, mounts private, bind volumes,
`pivot_root`, `/proc`) y al final `syscall.Exec` reemplaza su imagen por el
comando del usuario.

## Entregable exacto

Un único archivo: **`docs/anexo-a-bitacora.md`**.

Estructura obligatoria (una sección por hito + introducción):

```markdown
# Anexo A — Bitácora de decisiones de diseño

**Proyecto:** mini-docker
**Estudiante:** [nombre del estudiante — DEJA PLACEHOLDER `<TU_NOMBRE>`]
**Fecha:** [fecha de entrega — placeholder `<FECHA>`]

## Cómo leer esta bitácora

[Párrafo breve: motivo del documento y que refleja decisiones propias.]

## Hito 1 — Aislamiento con namespaces (UTS, PID, MNT)

### Decisión 1.1 — [...]
**Contexto:** ...
**Alternativas consideradas:** ...
**Decisión tomada:** ...
**Justificación:** ...

### Decisión 1.2 — [...]

## Hito 2 — Rootfs propio (pivot_root / chroot)
...

## Hito 3 — Cgroups v2 (memoria y CPU)
...

## Hito 4 — CLI (--env, --volume, señales)
...

## Decisiones transversales
[ Las que no son de un hito concreto: formato de commits, .gitattributes LF,
  estructura por paquetes internal/*, etc. ]
```

## Restricciones

- **No inventes decisiones que no estén respaldadas por los commits o PR
  bodies.** Toda decisión debe poder trazarse a un commit.
- **No agregues Co-Authored-By ni mención a Claude/IA** en el documento. Es
  un documento del estudiante.
- **No escrebas código.** Solo markdown.
- **No modifiques nada del repo.** Solo creás `docs/anexo-a-bitacora.md`.
- Usa español rioplatense neutro (voseo, sin coloquialismos pesados).
- Máximo ~400 líneas. Conciso pero completo.

## Información de base (commits y decisiones a documentar)

Corré `git log --format="%H%n%s%n%b%n---"` para leer todos los mensajes.
Estas son las decisiones clave que **deben** aparecer (extraídas de los
commits/PR bodies reales):

### Hito 1 (commit `9221342`)
- Crear hijo con `CLONE_NEWUTS|NEWPID|NEWMNT` vía `SysProcAttr.Cloneflags`.
- Elección de esos 3 namespaces (no NET ni USER en este hito).

### Hito 2 (commit `7c2a492`, PR #1)
- `pivot_root(2)` como primario con **fallback a `chroot(2)** si falla.
- `mount --make-rprivate /` (`MS_REC|MS_PRIVATE`) ANTES de pivot_root: evita
  que mounts del contenedor se propaguen al host en systemd.
- `/proc` se monta DESPUÉS del cambio de raíz (no antes) para que refleje el
  PID namespace dentro del nuevo rootfs.
- `scripts/setup-rootfs.sh` descarga Alpine minirootfs 3.21 (URL corregida;
  la de `edge/` del README original no existía).
- Bugfix Hito 1: el padre ahora **reenvía `--rootfs` al proceso init**
  (antes el hijo usaba el default) y normaliza a ruta absoluta.

### Hito 3 (commit `85b7d7e`, PR #2)
- Nuevo paquete `internal/cgroup/` con split por SO:
  `cgroup.go` (común) + `cgroup_linux.go` + stub `cgroup_other.go`.
- Manager delega controladores `memory`/`cpu` en `cgroup.subtree_control`
  del padre antes de crear el cgroup hijo (cgroups v2 lo exige).
- `cgroup.kill "1"` (kernel 5.14+) para limpieza con fallback SIGKILL por PID.
- `removeWithRetry` reintenta `rmdir` ante `EBUSY` (el kernel tarda ms en vaciar).
- **Fix de portabilidad crítico**: `mount --make-rprivate /` antes de
  montar `/proc`. Sin esto, en hosts con systemd (`/ = MS_SHARED`) el mount
  de `/proc` del contenedor se propaga al host y corrompe su `/proc`
  (síntoma: "la 1ª corrida funciona, las siguientes fallan con fork/exec").
- Flags `--memory` (`100m`/`1g`/bytes) y `--cpu` (núcleos, ej. `0.5`).

### Hito 4 (commit `d55ea0a`, PR #3)
- `--env KEY=VALUE` y `--volume /host:/contenedor`, ambos **repetibles** (como Docker).
- `mergeEnv` da **prioridad a `--env` sobre variables heredadas** (permite
  override, p. ej. `PATH`).
- `mountVolumes` hace bind mounts **ANTES del `pivot_root`** (el origen es
  del host y desaparece al desmontar el root viejo).
- `forwardSignals` reenvía `SIGINT`/`SIGTERM` al contenedor y fuerza
  `SIGKILL` tras `killGrace = 3s` (PID 1 no recibe SIGTERM por default).

### Decisiones transversales (commits `ced156b`, `243a110`, `fd6f937`, `f614452`, `096d773`)
- `.gitattributes` fuerza `eol=lf` en `*.go`/`*.mod`/`*.sum`/`*.sh`:
  garantiza conformidad con `gofmt` en Windows (sin CRLF) y scripts shell
  válidos en busybox.
- `.gitignore` excluye `guia minidocker.md` (material del docente con
  controles anti-autoría Sección 2.3) y `rootfs/*`.
- Suite de tests: `FormatCPUMax`/`Contains`/`ParseMemory`/`ParseEnv`/
  `ParseVolumes` extraídas como funciones exportadas en archivos comunes
  (sin build tag) para ser testeables en cualquier SO.
- `signals_linux_test.go` con `//go:build linux` y patrón helper-process.
- Commits como work-units (tests + código juntos, no por tipo de archivo).

## Pasos sugeridos

1. Corré `git log --format="%H %s%n%b%n---"` para confirmar el historial.
2. Leé los archivos clave para entender las decisiones reales:
   `internal/namespace/namespace_linux.go`,
   `internal/container/setup_linux.go`,
   `internal/container/container.go`,
   `internal/cgroup/cgroup_linux.go`,
   `cmd/minidocker/main.go`,
   `.gitattributes`, `.gitignore`.
3. Redactá `docs/anexo-a-bitacora.md` siguiendo la estructura obligatoria.
4. Cada decisión debe tener: **Contexto / Alternativas / Decisión /
   Justificación** (4 subtítulos o 4 párrafos cortos).
5. Dejá placeholders `<TU_NOMBRE>` y `<FECHA>` donde corresponda.

## Criterios de aceptación

- [ ] Existe `docs/anexo-a-bitacora.md`.
- [ ] Cubre los 4 hitos + decisiones transversales.
- [ ] Cada decisión tiene Contexto / Alternativas / Decisión / Justificación.
- [ ] Todas las decisiones del listado de arriba están documentadas.
- [ ] No hay menciones a IA ni `Co-Authored-By`.
- [ ] Menos de 400 líneas, español rioplatense neutro.
- [ ] Placeholders `<TU_NOMBRE>` y `<FECHA>` presentes.

## Cómo verificar

```bash
ls docs/anexo-a-bitacora.md
wc -l docs/anexo-a-bitacora.md   # < 400
grep -i -E "claude|anthropic|co-authored" docs/anexo-a-bitacora.md   # sin output
grep -c "^## Hito" docs/anexo-a-bitacora.md   # >= 4
```

## No hacer

- No modificar código fuente.
- No commitear (`git add`/`git commit` es tarea del estudiante).
- No inventar decisiones que no estén en el historial.
- No usar emojis.