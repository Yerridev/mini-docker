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
