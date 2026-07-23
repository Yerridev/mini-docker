# Mini-Docker — Conceptos de Go aplicados al runtime

Mapa rápido de **qué conceptos de Go** aparecen en `mini-docker` y **dónde se
aplican**. Pensado para repasar el lenguaje a la luz del proyecto.

Cada sección es **una tabla de 3 columnas**:
`Concepto Go | Qué es (1 línea) | Dónde: file:line`

> Para una explicación teórica más amplia de containers/namespaces/cgroups,
> ver [`docs/conceptos-base.md`](./conceptos-base.md). Para ejecutar el
> runtime, ver [`docs/ejecucion-y-pruebas.md`](./ejecucion-y-pruebas.md).

---

## 1. Paquetes y organización

| Concepto Go | Qué es | Dónde |
|---|---|---|
| `internal/` | Paquetes privados al módulo (no se pueden importar desde afuera) | `internal/config/`, `internal/container/`, `internal/cgroup/`, `internal/namespace/` |
| `cmd/bin/` | Convención: aquí viven los entrypoints del módulo | `cmd/minidocker/main.go` |
| `package main` | Paquete compilable a binario ejecutable | `cmd/minidocker/main.go:1` |
| `//go:build linux` | Restrictor de build: solo compila en Linux | `internal/container/setup_linux.go:1`, `internal/cgroup/cgroup_linux.go`, `internal/namespace/namespace_linux.go:1`, `signals_linux_test.go:1` |
| `//go:build !linux` | Stub no-op para Windows/macOS (compila pero no hace nada) | `internal/container/setup_other.go:1`, `internal/cgroup/cgroup_other.go:1`, `internal/namespace/namespace_other.go:1`, `exec_other.go:1` |
| Pareja `xxx_linux.go` + `xxx_other.go` | Patrón para portabilidad: implementación real + stub vacío | `setup_linux.go` / `setup_other.go`, `cgroup_linux.go` / `cgroup_other.go` |
| `go.mod` | Declara el módulo, ruta y versión de Go | `go.mod` — `module minidocker` |

**Gotcha:** los dos archivos de cada pareja implementan los **mismos nombres de
funciones** pero solo uno compila según el GOOS. Si agregás una función nueva
a `xxx_linux.go`, también declarás el stub en `xxx_other.go` o `go build`
falla en Windows/macOS.

---

## 2. Errores como valores

| Concepto Go | Qué es | Dónde |
|---|---|---|
| Múltiples valores de retorno | `(val, err)` — idiomático en Go para señalizar errores en el flujo | `config.ParseMemory → (int64, error)`, `cgroup.New → (*Manager, error)` |
| `fmt.Errorf("...: %w", err)` | Envuelve un error existente agregando contexto, preservando la cadena para `errors.Is` | 31 usos; ej: `setup_linux.go:13`, `cgroup_linux.go:29`, `config.go:58` |
| `os.IsNotExist(err)` | Sentinel check sobre el error devuelto por `os.Stat`/`os.Remove` | `cgroup_linux.go:112` — `if err == nil || os.IsNotExist(err)` |
| `return nil` | "No pasó nada" — también un valor válido | `FormatCPUMax` no errora; `Contains` devuelve bool; setters con guard de cero |
| Error fatal en main | `fmt.Fprintf(os.Stderr, "error: %v\n", err); os.Exit(1)` en el entrypoint | `cmd/minidocker/main.go:71` — `fatal(err)` |
| Sin `panic` | El runtime nunca usa `panic` para control de flujo; los errores son valores | — |

**Gotcha:** `%w` no es lo mismo que `%v`. `%w` envuelve (permite
`errors.Is`/`errors.As` en capas superiores). `%v` solo agrega texto y
**corta la cadena**. El proyecto usa `%w` consistentemente.

**Nota:** el runtime **no** usa `errors.Is`/`errors.As` (todavía) porque
hasta ahora no necesita discriminar tipos de error más allá de `os.IsNotExist`.
Tampoco usa `context.Context` — son gaps de los hitos 1-4; Hito 5 o
refactors futuros probablemente lo introduzcan para timeouts.

---

## 3. Interfaces y tipos

| Concepto Go | Qué es | Dónde |
|---|---|---|
| Implementación implícita de interfaces | No declarás `implements`; si tus métodos satisfacen la firma, ya la cumplís | `cmd/minidocker/main.go:21-24` — `stringSlice` cumple `flag.Value` (String + Set) sin declararlo |
| Métodos con receiver por puntero | `func (m *Manager) ...` — puede mutar el estado del Manager | `cgroup_linux.go:51` — `func (m *Manager) SetMemoryLimit` |
| Métodos con receiver por valor | `func (m *Manager) Path()` — puede usar valores por copia | `cgroup.go:25` — `Path()` |
| Campos exportados | Mayúscula = visible fuera del paquete | `config.Config.Rootfs`, `Volume.Source`, `Manager.Path` |
| Campos no exportados | minúscula = privado al paquete | `Manager.path`, `Manager.parent`, `Manager.id` |
| Struct tagged | Comentarios por hito sobre los campos | `internal/config/config.go:9-22` |
| Constructor explícito | Función `NewX(...)` que devuelve el tipo inicializado (valor cero no utilizable) | `cgroup.New(id)` en `cgroup_linux.go:29`, `Container.New(cfg)` en `container.go:28` |

**Gotcha de `Manager`**: el valor cero `Manager{}` **no es utilizable** (no
tiene path). Por eso hay que llamar `cgroup.New(id)`. Es la convención de Go
para tipos que requieren setup.

---

## 4. Concurrencia

| Concepto Go | Qué es | Dónde |
|---|---|---|
| Goroutine | Función que corre concurrentemente (`go func() {...}()`) | `container.go:102` — `forwardSignals` |
| Channel tipado | Tubería tipada para sincronizar goroutines sin locks | `container.go:98` — `chan os.Signal`; `container.go:100` — `chan struct{}` |
| `make(chan T, n)` | Canal con buffer de capacidad n | `container.go:98` — buffer 1 para no perder señales; `signals_linux_test.go:62` — `make(chan error, 1)` |
| `signal.Notify(ch, sigs...)` | Replica señales del proceso en un channel accesible desde goroutines | `container.go:99` — `signal.Notify(ch, os.Interrupt, syscall.SIGTERM)` |
| `select` | Multiplexación: el primer caso listo gana | `container.go:103-113` — espera señal, done o timeout |
| `time.After(d)` | Devuelve un channel que se cierra tras `d` — útil para timeouts en select | `container.go:108` — `time.After(killGrace)` |
| `close(ch)` | Cierra un channel — receptores reciben valor cero + flag `ok=false` | `container.go:116` — `close(done)` en el `stop()` retornado |
| `signal.Stop(ch)` | Desregistra la suscripción de señales (idealmente antes de salir) | `container.go:117` |
| `sync.WaitGroup` | Barrera reutilizable para esperar a N goroutines | `signals_linux_test.go:77` — 10 goroutines start→stop con `wg.Add/Done/Wait` |
| `go test -race` | Flag que instrumenta el binario para detectar data races | rúbrica Sección 3.1; verificado en `go test -race -count=1 ./...` |

**Gotcha (killGrace pattern):** el `select` con `time.After(killGrace)`
implementa "si no recibí `done` en 3s, mato el proceso". Es exactamente el
modelo de `docker stop` y uno de los puntos más pedagógicos del proyecto.

---

## 5. Ciclo de vida con `defer`

| Concepto Go | Qué es | Dónde |
|---|---|---|
| `defer` | Difiere la llamada al return de la función, LIFO | `container.go:66` — `defer cg.Cleanup()` |
| LIFO | Last-In First-Out: los defers se ejecutan en orden inverso | `container.go:90` se defiere `stop` antes que `cmd.Wait()` retorne; se ejecuta después |
| `defer f()` con cierre (`closure`) | Captura variables — útil para cleanup con parámetros | `signals_linux_test.go:48` — `defer func() { _ = cmd.Process.Kill() }()` |
| Función retornada `func()` | Patrón: `forwardSignals(cmd)` devuelve un `stop()` para ser `defer`eado | `container.go:115-118` |
| Garantía "pase lo que pase" | `defer` corre incluso con panic o early return | `container.go:66` — si `SetMemoryLimit` falla y retorna, `Cleanup` igual corre |

**Gotcha:** `defer` se usa comunmente para `Close()`/`Unlock()`/`Cleanup()`. En
este proyecto solo hay 3 `defer` reales (no en tests): los dos de
`container.go` (Cleanup + stop) y el `defer stop()` implícito en el test.
Es lo mínimo y suficiente.

---

## 6. E/S y procesos

| Concepto Go | Qué es | Dónde |
|---|---|---|
| `flag.String/Float64/Bool` | Flags escalares con valor default + descripción | `cmd/minidocker/main.go:34-36` |
| `flag.Var(v, name, desc)` | Flag custom: registrás un tipo que cumpla `flag.Value` | `cmd/minidocker/main.go:37-38` — `flag.Var(&envFlags, "env", ...)` |
| `flag.Parse()` | Lee `os.Args` y llena los flags registrados | `cmd/minidocker/main.go:39` |
| `flag.Args()` | Argumentos posicionales restantes post-parse | `cmd/minidocker/main.go:79, 104` |
| Stringer (`String() string`) | Satisfaction de `flag.Value` — repr del flag en help | `cmd/minidocker/main.go:21` |
| `exec.Command(name, args...)` | Crea un `*Cmd` para ejecutar un proceso | `container.go:43` — `exec.Command("/proc/self/exe", initArgs...)` |
| `cmd.Start()` vs `cmd.Run()` | Start = arranca y devuelve control (te da el PID); Run = Start + Wait | `container.go:77` usa Start para obtener `cmd.Process.Pid` y moverlo al cgroup antes de esperar |
| `cmd.SysProcAttr` | Hook donde se pasan flags de clone (namespaces) | `container.go:50` — `cmd.SysProcAttr = namespace.SysProcAttr()` |
| `cmd.Wait()` | Bloquea hasta que el proceso termina y devuelve su error | `container.go:92` |
| `os.Environ()` | Slice `["K=V", ...]` con el entorno del proceso actual | `container.go:53`, `exec_linux.go:10` |
| `os.Getpid()` | Devuelve el PID del proceso actual | `container.go:146` — `containerID()` |
| `os.Getenv("K")` | Lee una variable de entorno sin error | `cmd/minidocker/main.go:68` — `isInit()` |
| `os.Exit(n)` | Termina el proceso inmediatamente (no corre defers) | `cmd/minidocker/main.go:73` — `fatal` solo en entrypoint, no en dominio |
| `/proc/self/exe` | Symlink del kernel al binario en ejecución — permite re-exec | `container.go:43` |

**Gotcha:** `os.Exit` no corre `defer`. Por eso `fatal()` solo se usa en
`main.go` (el entrypoint), nunca en el dominio (`internal/...`). El dominio
siempre devuelve `error` para que main decida si salir.

---

## 7. `syscall` y bajo nivel

| Concepto Go | Qué es | Dónde |
|---|---|---|
| `syscall.SysProcAttr` | Struct de configuración passed a `cmd.SysProcAttr`; tiene `Cloneflags` | `namespace_linux.go:11` |
| `Cloneflags` | Bits OR de `CLONE_NEW*` para pedir namespaces | `namespace_linux.go:13` — `flagNEWUTS | flagNEWPID | flagNEWMNT` |
| `syscall.Exec(cmd, args, env)` | Reemplaza la imagen del proceso actual (PID no cambia, no crea uno nuevo) | `exec_linux.go:10` |
| `syscall.Mount(src, dst, fstype, flags, data)` | Cuelga un filesystem | `setup_linux.go:23, 42, 96, 114` |
| `syscall.PivotRoot(new, put)` | Intercambia el root y deja el viejo en `put` | `setup_linux.go:72` |
| `syscall.Unmount(path, flags)` | Desmonta (con `MNT_DETACH` = lazy) | `setup_linux.go:79` |
| `syscall.Chroot(path)` | Cambia la raíz aparente del proceso | `setup_linux.go:55` |
| `syscall.Sethostname([]byte)` | Cambia hostname (UTS namespace) | `setup_linux.go:12` |
| `syscall.Kill(pid, sig)` | Envía una señal a un PID | `cgroup_linux.go:102` (fallback de `killAll`) |
| `syscall.SIGTERM`, `os.Interrupt` (SIGINT) | Señales reenviadas al init del contenedor | `container.go:99` |
| Separación `_linux.go` | Los syscalls de namespaces solo existen en Linux; se separan por build tag | `namespace_linux.go:1`, `setup_linux.go:1`, `cgroup_linux.go:1` |

**Gotcha:** `syscall.Exec` **no es** `exec.Command`. `Exec` reemplaza el
proceso actual (mantiene el PID). `exec.Command` **crea** un proceso nuevo.
El runtime usa los dos, en momentos distintos: el padre usa `exec.Command`
para crear el hijo; el hijo usa `syscall.Exec` para reemplazarse por
`/bin/sh`.

---

## 8. Testing

| Concepto Go | Qué es | Dónde |
|---|---|---|
| Table-driven tests | Tabla de casos (struct anónimo + slice) recorrida en un loop | `config_test.go:18` — `TestParseMemory` con 18 casos; `cgroup_test.go:12` — `TestFormatCPUMax` con 6 casos |
| `t.Run(name, func(t *T))` | Subtest aislado: cada caso se reporta por separado | `config_test.go:37` |
| `t.Fatalf(format, ...)` | Marca el subtest como fallido y para acá | toda la suite |
| `t.Errorf(format, ...)` | Marca el subtest como fallido y sigue | `signals_linux_test.go:88` |
| `t.Helper()` | Marca un test helper (no figura como el origen del FAIL) | `mergeenv_test.go:73` — `envToMap` helper |
| Helper-process pattern | Re-ejecutá tu binario de tests como proceso hijo (`os.Args[0]`) con env var de check | `signals_linux_test.go:36, 43` — `GO_HELPER_PROCESS=1` |
| `exec.Command(os.Args[0], "-test.run=...", ...)` | Arranca un subtest como proceso | `signals_linux_test.go:43` |
| Build tags en tests | Algunos tests solo corren en Linux (`//go:build linux`) | `signals_linux_test.go:1` |
| `go test -cover` | Reporta cobertura por paquete | 97% config, 50% cgroup, 17.6% container (sube en Linux) |
| `go test -race` | Detector de data races durante la ejecución del test | rúbrica Sección 3.1, `forwardSignals` el principal target |
| `go test -count=1` | Desactiva cache de resultados | recomendado cuando probás cambios de comportamiento |

**Gotcha (helper-process):** es el patrón oficial de Go para testear
comportamiento de procesos. **No usar** `/bin/sleep` en Windows/macOS (no
existe), por eso `signals_linux_test.go` re-ejecuta el propio binario de
tests con `GO_HELPER_PROCESS=1` — el mismo subtest detecta la env var y
actúa como helper dormido.

---

## 9. Convenciones del proyecto

| Concepto | Qué es | Dónde |
|---|---|---|
| Conventional commits | `feat:`, `fix:`, `test:`, `docs:`, `chore:` | todo `git log` |
| Scope opcional | `feat(cgroup):`, `test(container):` | `git log --oneline` |
| Work-unit commits | Tests en el mismo commit que el código que verifican (no commits "add tests") | commits `243a110`, `fd6f937`, `f614452` |
| `gofmt -l .` | Validador de formato: debe dar vacío | rúbrica Sección 3.1 |
| `.gitattributes` con `eol=lf` | Garantiza LF en `*.go`/`*.mod`/`*.sum`/`*.sh` en cualquier plataforma | `.gitattributes` — sin esto `gofmt -l` falla en Windows (CRLF) |
| `go vet ./...` | Análisis estático además de `gofmt` | rúbrica Sección 3.1 |
| Comentarios en español | Los comentarios del equipo van en español (rioplatense neutro) | todos los archivos `.go` |
| Sin `Co-Authored-By` | Política del repo: no atribuir authorship a la IA en los mensajes de commit | `AGENTS.md` del usuario |
| Sin emojis | Tanto en código como en docs | todo el repo |

---

## 10. Auto-check — 10 preguntas para repasar

Antes de la defensa oral, intentá responder de memoria. Las respuestas son
1 línea.

| # | Pregunta | Respuesta |
|---|---|---|
| 1 | ¿Qué hace `syscall.Exec` y en qué se diferencia de `exec.Command`? | `Exec` reemplaza la imagen del proceso actual (PID no cambia); `exec.Command` crea un proceso nuevo. |
| 2 | ¿Por qué el padre usa `cmd.Start()` y no `cmd.Run()`? | Start devuelve el control de inmediato → obtener el PID y moverlo al cgroup antes de esperar. |
| 3 | ¿Por qué `forwardSignals` necesita `time.After(killGrace)` en el select? | Para forzar SIGKILL si el init (PID 1 de su namespace) no responde en 3s — PID 1 no recibe SIGTERM por default. |
| 4 | ¿Qué diferencia `%w` de `%v` en `fmt.Errorf`? | `%w` envuelve el error (preserva la cadena para `errors.Is`); `%v` solo agrega texto y corta la cadena. |
| 5 | ¿Por qué hay archivos `*_linux.go` y `*_other.go`? | Para que `go build ./...` funcione en cualquier SO: en Linux compile la implementación real, en Windows/macOS compile el stub no-op. |
| 6 | ¿Qué es `/proc/self/exe` y por qué el padre lo ejecuta? | Symlink del kernel al binario actual; el padre lo ejecuta como hijo para que el hijo nazca con namespaces (CLONE_NEW* solo aplica al crear). |
| 7 | ¿Por qué `t.Helper()` en `envToMap`? | Marca la función como helper de test: si falla, el FAIL apunta al caller, no al helper. |
| 8 | ¿Qué hace `defer cg.Cleanup()` y por qué es `defer`? | Difiere la limpieza del cgroup al return de Run — garantía de limpieza aún con panic o early return. |
| 9 | ¿Para qué sirve el patrón helper-process en tests? | Re-ejecutar `os.Args[0]` como proceso hijo con env var de check — permite testear señales (`SIGTERM`, etc.) sin depender de `/bin/sleep` que no existe en todos los SO. |
| 10 | ¿Por qué `go test -race` es obligatorio en este proyecto? | Hay concurrencia real en `forwardSignals` (goroutine + channels); `-race` detecta data races que el test normal no ve. La rúbrica lo exige. |

Si no podés responder una de memoria → revisitá la sección correspondiente
y el `file:line` señalado.

---

## 11. Ver también

- [`docs/conceptos-base.md`](./conceptos-base.md) — conceptos teóricos del
  runtime (namespaces, cgroups v2, pivot_root, re-exec).
- [`docs/ejecucion-y-pruebas.md`](./ejecucion-y-pruebas.md) — cómo instalar,
  ejecutar los 4 hitos y validar la rúbrica.
- [`docs/anexo-a-bitacora.md`](./anexo-a-bitacora.md) — bitácora de
  decisiones de diseño por hito.
- [`docs/tareas/`](./tareas/) — briefs para los integrantes del grupo.