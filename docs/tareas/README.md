# Tareas para los integrantes del grupo

Tres briefs auto-contenidos, listos para pasarse a un LLM (Claude, GPT,
Gemini, etc.) y que ejecute la tarea sin necesidad de más contexto.

## Reglas comunes a los tres

- **Branch por tarea.** Cada integrante abre su rama desde `main` y abre PR
  al terminar. No se commitea directo a `main`.
- **Conventional commits**, sin `Co-Authored-By` ni atribución a la IA.
- **No usar emojis** en código, docs ni commits.
- **`.gitattributes` ya fuerza LF** en `*.go`/`*.md`/`*.sh` — respetalo.
- **Verificación obligatoria antes de abrir PR:**
  ```bash
  go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...
  ```

## Reparto

| Integrante | Tarea | Tipo | Obligatoria | Archivo |
|---|---|---|---|---|
| 1 | Anexo A — Bitácora de decisiones | Documentación | ✅ Sí (rúbrica 2.3) | `integrante-1-anexo-a-bitacora.md` |
| 2 | Anexo B — Declaración de uso de IA | Documentación | ✅ Sí (rúbrica 2.3) | `integrante-2-anexo-b-declaracion-ia.md` |
| 3 | Hito 5 — Networking (CLONE_NEWNET + loopback, opcional veth) | Código Go + tests + docs | ⬜ Opcional (bonus 25%) | `integrante-3-hito-5-networking.md` |

## Orden recomendado de ejecución

1. **Integrante 1 (Anexo A)** y **Integrante 2 (Anexo B)** pueden trabajar
   en paralelo — son independientes.
2. **Integrante 3 (Hito 5)** puede arrancar en paralelo, pero su PR debe
   mergearse después de los Anexos (no depende técnicamente, pero
   conviene cerrar lo obligatorio antes del bonus).

## Cómo usar cada brief

1. El integrante lee su `.md`.
2. Copia el contenido completo del `.md` y se lo pasa a su LLM elegito
   (Claude, GPT, Gemini, opencode, Cursor, etc.).
3. El LLM ejecuta la tarea siguiendo el brief. El integrante **revisa** y
   **comprende** todo lo que el LLM produce (la rúbrica verifica autoría).
4. El integrante abre rama + PR con los cambios.

> **Recordatorio rúbrica (Sección 2.3):** el control de mayor peso es que
> el estudiante pueda **explicar y modificar en vivo** el código que
> entrega. Usar IA está permitido; pegar sin comprender, no. Declarar el
> uso honestamente en el Anexo B NO penaliza; ocultarlo, sí.

## Pendiente del líder (repo owner)

Antes de asignar las tareas, confirmá que tu VM tiene red y que `main`
está pusheado a GitHub:
```bash
git push origin main   # sync del commit 096d773 si no se subió aún
git log --oneline -3   # verificar que origin/main == local
```