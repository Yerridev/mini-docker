# Tarea — Integrante 2: Anexo B, Declaración de uso de IA

> **Instruido a:** un LLM que va a redactar un documento. No debe escribir
> código: solo producir `docs/anexo-b-declaracion-ia.md`.

---

## Rol

Sos un asistente que redacta documentación técnica académica en español
rioplatense neutro. Ayudás a un estudiante a completar un entregable
obligatorio de la rúbrica: la **declaración honesta de uso de IA** (Anexo B
de la guía del curso).

## Por qué existe esta tarea

La rúbrica (Sección 2.3) exige que cada entrega incluya una declaración de
**qué herramientas de IA se usaron, para qué y en qué partes**. **Declarar
honestamente NO penaliza; ocultarlo sí.** Si se detecta uso no declarado,
aplica el reglamento de honestidad académica.

## Contexto del proyecto

`mini-docker`: runtime de contenedores minimalista en Go. Repositorio
`https://github.com/Yerridev/mini-docker`, rama `main`, HEAD `2e3d38a`.

**Evidencia objetiva de uso de IA en el repo:**
- Los commits `9221342`, `7c2a492`, `85b7d7e`, `d55ea0a` traen
  `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` en el mensaje.
- Los PR bodies de #2 y #3 (visible en GitHub) terminan con
  "Generated with Claude Code" / referencias a Claude.
- Esto significa que un asistente de IA (Claude, vía Claude Code / opencode)
  participó en la escritura de código de los Hitos 1 a 4.

**Lo que NO se puede inferir solo del repo** (el estudiante debe confirmar):
- Qué % fue generado vs. revisado/adaptado por el humano.
- Si el estudiante comprende línea por línea lo entregado (esto se verifica
  en la defensa oral, no en este documento).
- Si hubo otras IA usadas (ChatGPT, Copilot, Gemini, etc.).
- Qué partes específicas el estudiante escribió sin IA.

## Entregable exacto

Un único archivo: **`docs/anexo-b-declaracion-ia.md`**.

Estructura obligatoria:

```markdown
# Anexo B — Declaración de uso de inteligencia artificial

**Proyecto:** mini-docker
**Estudiante:** <TU_NOMBRE>
**Fecha:** <FECHA>

## 1. Herramientas de IA utilizadas

| Herramienta | Versión / modelo (si aplica) | Uso |
|---|---|---|
| Claude (Anthropic) | Claude Fable 5 (vía Claude Code / opencode) | Escritura y revisión de código Go, resolución de bugs |
| ... | ... | ... |

## 2. Resumen honesto del uso

[Párrafo: qué rol cumplió la IA y qué rol cumplió el estudiante. Sé honesto
y específico. Ejemplo: "La IA propuso implementaciones que el estudiante
revisó, compiló, ejecutó y ajustó; no se pegó código sin comprenderlo."]

## 3. Uso por hito

### Hito 1 — Namespaces
- **Partes con IA:** [Qué se hizo con asistencia]
- **Partes sin IA:** [Qué hizo el estudiante solo]
- **Verificación de comprensión:** [Cómo comprobó el estudiante que entiende
  lo generado: ejecutó, modificó, depuró]

### Hito 2 — Rootfs + pivot_root
...

### Hito 3 — Cgroups
...

### Hito 4 — CLI + señales
...

### Testing y docs
...

## 4. Decisiones delegadas a la IA

[Lista de decisiones de diseño donde la IA propuso y el estudiante aceptó,
con la justificación del estudiante de por qué aceptó. La rúbrica prohibe
delegar decisions de arquitectura sin poder explicarlas.]

## 5. Compromiso de autoría

[Declaración del estudiante de que comprende el código entregado y puede
explicarlo y modificarlo en la defensa oral. Firma con placeholder.]

## 6. Pendientes de confirmación humana

<!-- El estudiante debe revisar este documento y completar/ajustar antes
     de entregar. La IA no puede atestiguar sobre la comprensión del
     estudiante. -->
- [ ] Confirmar lista de herramientas (¿sólo Claude, o hubo otras?)
- [ ] Confirmar qué partes escribió el estudiante sin IA
- [ ] Confirmar que comprende el código y puede defenderlo
- [ ] Firmar y fechar
```

## Restricciones

- **El documento debe ser HONESTO.** Si no sabés algo (cuánto escribió el
  humano vs. la IA), dejá un placeholder con instrucción clara de que el
  estudiante lo complete. **No inventes porcentajes ni detalles.**
- La IA no puede atestiguar sobre la comprensión del estudiante. La sección
  "Compromiso de autoría" debe ser un placeholder que el estudiante firma.
- **No agregues `Co-Authored-By` ni menciones a Claude como autor del
  documento.** El documento es del estudiante.
- **No escrebas código.** Solo markdown.
- **No modifiques nada del repo.** Solo creás `docs/anexo-b-declaracion-ia.md`.
- Español rioplatense neutro, sin emojis.

## Información de base

Para reconstruir qué se hizo con IA por hito, corré:
```bash
git log --format="%H %s%n%b" | grep -B1 -A3 "Co-Authored-By"
```
Y revisá los PR bodies (`gh pr view 1`, `gh pr view 2`, `gh pr view 3`).

Asignación de hito → persona (según PR bodies):
- Hito 1 (commit `9221342`): sin PR explícito, co-author Claude presente.
- Hito 2 (PR #1, `7c2a492`): Integrante con rol Hitos 1-2, co-author Claude.
- Hito 3 (PR #2, `85b7d7e`): "Integrante B" (cgroups), co-author Claude.
- Hito 4 (PR #3, `d55ea0a`): "Integrante D" (integración), co-author Claude.

**Nota clave:** `Co-Authored-By: Claude Fable 5` aparece en commits del Hito
1 al 4. Los commits de tests (`243a110`, `fd6f937`, `f614452`) y docs
(`096d773`, `ced156b`) NO traen co-author IA — esos los redactó una sesión
posterior con política de no atribuir IA.

## Pasos sugeridos

1. Corré `git log --format="%H %s%n%b%n---"` para confirmar qué commits
   tienen `Co-Authored-By: Claude`.
2. Leé los 3 PR bodies: `gh pr view 1 --json body`, `gh pr view 2 --json
   body`, `gh pr view 3 --json body`.
3. Redactá `docs/anexo-b-declaracion-ia.md` con la estructura obligatoria.
4. En **"Partes con IA"** usá evidencia objetiva del commit (co-author
   presente). En **"Partes sin IA"** dejá placeholder con instrucción al
   estudiante de completar.
5. En **"Verificación de comprensión"** dejá placeholder — solo el
   estudiante puede declarar cómo comprobó que entiende el código.
6. En **"Decisiones delegadas a la IA"** listá las decisiones técnicas que
   aparecen en los commits (pivot_root, make-rprivate, cgroup.kill,
   forwardSignals con killGrace, etc.) y dejá el "por qué aceptó" como
   placeholder para el estudiante.

## Criterios de aceptación

- [ ] Existe `docs/anexo-b-declaracion-ia.md`.
- [ ] Lista de herramientas incluye Claude (confirmado por `Co-Authored-By`).
- [ ] Sección por hito con Partes con/sin IA + Verificación de comprensión.
- [ ] Placeholders claros donde se necesita input humano (`<COMPLETAR>`).
- [ ] Sin `Co-Authored-By` ni mención a Claude como autor del doc.
- [ ] Sin porcentajes inventados de uso IA/humano.
- [ ] Incluye checklist final de pendientes de confirmación.

## Cómo verificar

```bash
ls docs/anexo-b-declaracion-ia.md
grep -c "^### Hito" docs/anexo-b-declaracion-ia.md   # >= 4
grep -c "COMPLETAR" docs/anexo-b-declaracion-ia.md   # >= 4 (placeholders)
grep -i -E "co-authored" docs/anexo-b-declaracion-ia.md   # sin output
```

## No hacer

- No inventar porcentajes de uso IA/humano.
- No atestiguar sobre la comprensión del estudiante (sólo él puede).
- No commitear.
- No usar emojis.
- No listar herramientas que no estén confirmadas (dejá placeholder).