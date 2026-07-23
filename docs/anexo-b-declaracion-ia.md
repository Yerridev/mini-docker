# Anexo B - Declaracion de uso de inteligencia artificial

**Proyecto:** mini-docker  
**Estudiante:** <TU_NOMBRE>  
**Fecha:** <FECHA>

## 0. Autores detectados en Git y trazabilidad

Esta tabla resume los autores que aparecen en el historial Git visible de este clon y la relacion de cada commit con el trabajo del proyecto.

| Commit | Autor Git | Parte del proyecto | Relacion con IA |
|---|---|---|---|
| `9221342` | Yerri `<cramirezabondan@uss.edu.pe>` | Hito 1 - namespaces UTS, PID y MNT | `Co-Authored-By: Claude Fable 5` |
| `7c2a492` | Jhon `<jhon26798@gmail.com>` | Hito 2 - rootfs propio con `pivot_root` y fallback `chroot` | `Co-Authored-By: Claude Fable 5` |
| `3d373af` | Jhon | Integracion visible de Hitos 3 y 4 - `cgroups v2`, CLI, env, volumenes y senales | El brief del repositorio indica coautoria de Claude en los cambios base de H3/H4 |
| `e837dc2` | Keysi Jeanpierre Bardales Vasquez `<bvasquezkeysije@gmail.com>` | Hito 5 - networking (`CLONE_NEWNET` + loopback) |
| `89e9d8d` | JESLYN `<jes.hidrogo26@gmail.com>` | Bitacora de decisiones - Hito 1 | Sin coautor IA en el mensaje |
| `93f6f12` | JESLYN `<jes.hidrogo26@gmail.com>` | Bitacora de decisiones - Hito 2 | Sin coautor IA en el mensaje |
| `b8d2127` | JESLYN `<jes.hidrogo26@gmail.com>` | Bitacora de decisiones - Hito 3 | Sin coautor IA en el mensaje |
| `ee8ed8a` | JESLYN `<jes.hidrogo26@gmail.com>` | Bitacora de decisiones - Hito 4 | Sin coautor IA en el mensaje |
| `720fe48` | JESLYN `<jes.hidrogo26@gmail.com>` | Decisiones transversales | Sin coautor IA en el mensaje |
| `755817a` | Yerri `<cramirezabondan@uss.edu.pe>` | Briefs de tareas para integrantes | Sin coautor IA en el mensaje |
| `096d773` | Yerri `<cramirezabondan@uss.edu.pe>` | Guia de ejecucion, pruebas y defensa oral | Sin coautor IA en el mensaje |
| `626d30f` | Yerri `<cramirezabondan@uss.edu.pe>` | Docs de comandos de prueba y actualizaciones de ejecucion | Sin coautor IA en el mensaje |
| `43ae121` | Yerri `<cramirezabondan@uss.edu.pe>` | Docs de conceptos de Go aplicados al runtime | Sin coautor IA en el mensaje |
| `243a110` | Yerri `<cramirezabondan@uss.edu.pe>` | Tests de cgroup | Sin coautor IA en el mensaje |
| `fd6f937` | Yerri `<cramirezabondan@uss.edu.pe>` | Tests de config | Sin coautor IA en el mensaje |
| `f614452` | Yerri `<cramirezabondan@uss.edu.pe>` | Tests de container | Sin coautor IA en el mensaje |
| `7b660bb` | Yerri `<cramirezabondan@uss.edu.pe>` | Ajustes de documentacion para Hito 3 | Sin coautor IA en el mensaje |
| `ced156b` | Yerri `<cramirezabondan@uss.edu.pe>` | Forzar LF y gitignore de guia docente | Sin coautor IA en el mensaje |

## 1. Herramientas de IA utilizadas

| Herramienta | Version / modelo | Uso |
|---|---|---|
| Claude (Anthropic) | Claude Fable 5 (via Claude Code / opencode) | Asistencia en la escritura y revision tecnica de los Hitos 1 a 4 |


## 2. Resumen honesto del uso

La IA se utilizo como apoyo para proponer implementaciones, explicar decisiones tecnicas y revisar redaccion tecnica. El estudiante reviso el codigo, compilo, ejecuto pruebas y ajusto lo necesario antes de entregar. La autoria final y la capacidad de explicar cada cambio permanecen en el estudiante. Este documento solo declara lo que puede sostenerse con evidencia del historial Git y con lo que el estudiante confirme de forma personal.

## 3. Uso por hito

### Hito 1 - Namespaces
**Autor Git detectado:** Yerri (`9221342`).  
**Partes con IA:** aislamiento de namespaces UTS, PID y MNT; preparacion del flujo de re-ejecucion.  
 
**Verificacion de comprension:** <COMPLETAR: explicar como se probo y que se modifico manualmente>.

### Hito 2 - Rootfs + `pivot_root`
**Autor Git detectado:** Jhon (`7c2a492`).  
**Partes con IA:** implementacion de `pivot_root`, fallback con `chroot`, remonte como `MS_REC|MS_PRIVATE`, montaje de `/proc` despues del cambio de raiz y ajuste del script de `rootfs`.  
 
**Verificacion de comprension:** <COMPLETAR: explicar como se probo `ls /`, `ps`, `mount` y el aislamiento del host>.

### Hito 3 - `cgroups`
**Autor Git detectado:** Jhon (`3d373af`).  
**Partes con IA:** definicion de limites `memory.max` y `cpu.max`.  

**Verificacion de comprension:** <COMPLETAR: explicar memoria, CPU y limpieza del cgroup>.

### Hito 4 - CLI + senales
**Autor Git detectado:** Jhon (`3d373af`).  
 
**Verificacion de comprension:** <COMPLETAR: explicar como se valido el manejo de senales y volumenes>.

### Hito 5 - Networking
**Autor Git detectado:** Keysi (`e837dc2`).    

**Verificacion de comprension:** <COMPLETAR: explicar como se probo el aislamiento de red y la interfaz `lo`>.

### Testing y documentacion
**Autores Git detectados:** Yerri, JESLYN y Keysi.  

**Verificacion de comprension:** <COMPLETAR: indicar que pruebas ejecuto el estudiante y que ajusto por su cuenta>.

## 4. Decisiones delegadas a la IA

| Decision tecnica | Commit / autor Git | Motivo tecnico | Justificacion del estudiante |
|---|---|---|---|
| Usar `pivot_root` con fallback a `chroot` | `7c2a492` / Jhon | Aisla mejor la raiz real del proceso y evita depender del root del host | <COMPLETAR> |
| Montar `/proc` despues del cambio de raiz | `7c2a492` / Jhon | Mantiene la vista correcta de procesos dentro del contenedor | <COMPLETAR> |
| Remontar `/` como `MS_REC|MS_PRIVATE` | `7c2a492` / Jhon | Evita fugas de montajes al host | <COMPLETAR> |
| Limpiar cgroups con `cgroup.kill` | `3d373af` / Jhon | Asegura limpieza de procesos colgados dentro del cgroup | <COMPLETAR> |
| Reenviar `SIGINT` y `SIGTERM` con gracia antes de `SIGKILL` | `3d373af` / Jhon | Permite cierre ordenado del contenedor | <COMPLETAR> |
| Dar prioridad a `--env` sobre variables heredadas | `3d373af` / Jhon | Evita sobrescritura incorrecta de configuracion | <COMPLETAR> |
| Soportar `--volume` como bind mount | `3d373af` / Jhon | Permite persistencia y acceso a archivos del host | <COMPLETAR> |

## 5. Compromiso de autoria

Declaro que comprendo el codigo entregado, puedo explicarlo en defensa oral y puedo modificarlo sin depender de una copia textual de la IA. Revise lo que la IA propuso, ejecute las pruebas necesarias y corrija lo que no funciono. Firma: <FIRMA_DEL_ESTUDIANTE>.
