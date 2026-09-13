# Freelance Job Agent — RFC & PRD

## Estado
Borrador v1 — listo para implementación

---

# PARTE 1: PRD (Product Requirements Document)

## 1. Resumen
Sistema automatizado que monitorea ofertas de trabajo freelance/remoto en múltiples fuentes, las evalúa contra el perfil profesional del usuario usando un LLM, y notifica por Telegram las ofertas más relevantes.

## 2. Problema a resolver
Revisar manualmente múltiples plataformas de empleo freelance/remoto es lento y las ofertas relevantes se mezclan con muchas que no lo son. Se necesita un proceso que filtre y priorice automáticamente.

## 3. Usuario objetivo
Backend engineer senior (5-8 años de experiencia), perfil:
- Stack: Go, Kotlin/Spring Boot, históricamente C#
- Dominio: sistemas distribuidos, alta transaccionalidad, mensajería (Kafka, SQS/SNS)
- Fintech: pagos, transferencias, cash-in, remesas, ISO 20022/SWIFT
- Cloud: AWS (EC2, S3, DynamoDB), Postgres, Oracle, Redis
- Busca roles remotos, hands-on con código, sin restricción geográfica que excluya LATAM

## 4. Objetivos
- Reducir el tiempo de búsqueda manual de oportunidades
- No perder ofertas relevantes por volumen o mala redacción
- Minimizar falsos positivos (evitar spam de notificaciones irrelevantes)
- Costo operativo mínimo (infra + LLM)

## 5. Fuera de alcance (esta primera etapa)
- Scraping directo de sitios web (LinkedIn, Workana, u otros)
- Torre.co y GetOnBoard (quedan para una segunda etapa, evaluando si ofrecen mail/API antes de considerar scraping)
- Postulación automática a las ofertas
- Interfaz web/dashboard (solo Telegram como canal de salida)

## 6. Requerimientos funcionales

### 6.1 Ingesta de ofertas
- **LinkedIn**: lectura de alertas de empleo configuradas por el usuario, recibidas por mail, vía IMAP
- **Workana**: lectura de alertas de proyectos por mail, vía IMAP
- **Remotive**: consumo de su API pública, con filtro de ofertas LATAM/remoto

### 6.2 Filtro de reglas (pre-LLM)
Antes de invocar al LLM, descartar ofertas obvias por:
- Restricción geográfica explícita que excluya LATAM (ej. "US only", "EU only")
- Modalidad no remota (on-site, híbrido exigido)
- Ausencia total de match de stack técnico

### 6.3 Scoring con LLM
- Modelo: Claude Haiku (vía Anthropic API)
- Input: texto de la oferta + criterios de perfil del usuario
- Output esperado (texto plano, no JSON):
  ```
  RESUMEN: [2-3 líneas]
  PUNTUACIÓN: [1-100]
  ```
- Parseo con regex sobre los delimitadores `RESUMEN:` / `PUNTUACIÓN:`

### 6.4 Umbral y notificación
- Umbral de aviso: puntuación >= 65
- Canal de notificación: bot de Telegram
- Contenido del mensaje: resumen (2-3 líneas) + puntuación + (idealmente) link/fuente de la oferta

### 6.5 Frecuencia de ejecución
- Corrida cada 1 hora

## 7. Requerimientos no funcionales
- Costo de infraestructura: EC2 `t4g.micro` (ARM64, free tier)
- Costo de LLM: estimado <$2/mes en el escenario de mayor volumen (~50 ofertas/día evaluadas)
- Autenticación de mail: IMAP + App Password (no OAuth2, por simplicidad en esta etapa)
- Tolerancia a fallos: si el parser de un mail falla (cambio de template de LinkedIn/Workana), debe loguearse el error sin frenar el resto del pipeline

## 8. Métricas de éxito (sugeridas, a validar)
- % de ofertas notificadas que el usuario considera relevantes
- Ofertas relevantes que el filtro de reglas descartó por error (falsos negativos)
- Costo mensual real vs. estimado

## 9. Riesgos conocidos
- Cambios en el template HTML de los mails de LinkedIn/Workana rompen el parser
- LinkedIn podría cambiar la política de alertas por mail
- Prompt de scoring puede requerir iteración para calibrar mejor la escala 1-100

---

# PARTE 2: RFC (Request for Comments) — Diseño Técnico

## 1. Arquitectura general

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  LinkedIn   │   │   Workana    │   │  Remotive    │
│  (IMAP/mail)│   │  (IMAP/mail) │   │  (API REST)  │
└──────┬──────┘   └──────┬───────┘   └──────┬───────┘
       │                 │                   │
       └────────┬────────┴───────────────────┘
                 ▼
        ┌─────────────────┐
        │  Normalizador    │  (unifica formato: título, descripción, link, fuente)
        └────────┬─────────┘
                 ▼
        ┌─────────────────┐
        │  Filtro de reglas│  (stack, geografía, modalidad)
        └────────┬─────────┘
                 ▼
        ┌─────────────────┐
        │  Scoring (Haiku) │  (resumen + puntuación 1-100)
        └────────┬─────────┘
                 ▼
        ┌─────────────────┐
        │  Umbral >= 65?   │
        └────────┬─────────┘
                 ▼ (sí)
        ┌─────────────────┐
        │  Notificación    │  (Telegram bot)
        └─────────────────┘
```

Ejecución orquestada por un cron (o scheduler equivalente) cada 1 hora sobre una instancia EC2 `t4g.micro` (ARM64, free tier).

## 2. Componentes

### 2.1 Conectores de ingesta

**IMAP genérico (compartido por LinkedIn y Workana)**
- Conexión vía IMAP + App Password
- Filtro por remitente conocido (ej. `jobs-noreply@linkedin.com`, remitente de notificaciones de Workana)
- Marcar mails procesados (leído, o mover a carpeta) para no reprocesar

**Parser LinkedIn**
- El mail de alerta trae múltiples ofertas en un solo mensaje
- Parsear HTML para separar cada oferta individual (título, empresa, link, snippet de descripción)
- Debe tolerar variaciones de formato sin crashear (try/catch + logging del HTML crudo si falla)

**Parser Workana**
- Formato de mail más simple/estable
- Extraer título, presupuesto/tipo de proyecto, descripción, link

**Cliente Remotive**
- Llamada a la API pública
- Filtrar por remoto + LATAM (según los parámetros que ofrezca la API)

### 2.2 Normalizador
Modelo de datos común para todas las fuentes:
```
{
  "fuente": "linkedin" | "workana" | "remotive",
  "titulo": string,
  "descripcion": string,
  "link": string,
  "fecha_publicacion": datetime,
  "id_unico": string  // para deduplicar
}
```

### 2.3 Filtro de reglas
Reglas simples basadas en keywords/regex sobre `titulo` + `descripcion`:
- Descarta si matchea restricción geográfica excluyente
- Descarta si matchea modalidad no remota
- Descarta si no hay ningún match de stack técnico relevante

### 2.4 Scoring (integración Claude API)
- Modelo: `claude-haiku-4-5` (confirmar string de modelo vigente al momento de implementar)
- Request vía API REST estándar de Anthropic (`/v1/messages`)
- Prompt de sistema con el perfil del usuario (ver Anexo A)
- Parseo de respuesta con regex sobre `RESUMEN:` / `PUNTUACIÓN:`
- Manejo de errores: si el parseo falla o la API no responde, loguear y continuar (no bloquear el pipeline por una oferta)

### 2.5 Notificador Telegram
- Bot de Telegram ya creado (o a crear con BotFather)
- Envío vía Bot API (`sendMessage`)
- Formato del mensaje: resumen + puntuación + link + fuente

### 2.6 Persistencia (a definir en implementación)
- Necesario para deduplicar ofertas ya procesadas entre corridas
- Opciones: archivo local simple (JSON/SQLite) dado el volumen bajo, corriendo en la misma EC2

## 3. Costos estimados
- **Infra**: $0 (EC2 `t4g.micro`, ARM64, free tier)
- **LLM (Haiku)**: ~$0.50-$2/mes según volumen (ver cálculo de la conversación de origen)
- **Telegram Bot API**: $0 (gratuito)

## 4. Plan de implementación sugerido
1. Conector IMAP genérico + parser Workana (formato más simple, valida el pipeline primero)
2. Parser LinkedIn (más complejo por HTML variable)
3. Cliente Remotive (API, más directo)
4. Normalizador + filtro de reglas
5. Integración Claude API (scoring)
6. Notificador Telegram
7. Orquestación (cron/scheduler) + deploy en EC2
8. Persistencia para deduplicación

## 5. Preguntas abiertas
- ¿Persistencia simple (archivo/SQLite) es suficiente o conviene algo más robusto desde el inicio?
- ¿El mensaje de Telegram debe incluir el link directo a la oferta? (recomendado, no confirmado aún)
- ¿Qué pasa si Remotive cambia su API o límites de uso?

---

## Anexo A: Prompt de scoring (Claude Haiku)

```
Eres un asistente que evalúa ofertas de trabajo freelance/remoto para un backend engineer senior con este perfil:

- 5-8 años de experiencia, foco en backend (Go, Kotlin/Spring Boot, C# en el pasado)
- Sistemas distribuidos, alta transaccionalidad, mensajería (Kafka, SQS/SNS)
- Fintech: pagos, transferencias, cash-in, remesas, normas ISO 20022/SWIFT
- Cloud: AWS (EC2, S3, DynamoDB), bases relacionales y NoSQL (Postgres, Oracle, Redis)
- Experiencia liderando equipos técnicos, pero busca roles hands-on con código
- Preferencia: modalidad remota, sin restricción geográfica que excluya LATAM

Analiza la siguiente oferta de trabajo y respondé en este formato exacto:

RESUMEN: [2-3 líneas describiendo de qué se trata la oferta y por qué encaja o no con el perfil]
PUNTUACIÓN: [número del 1 al 100]

Guía de referencia para la puntuación:
0-20 = No relevante (stack o dominio totalmente distinto)
21-40 = Poco relevante (algún match menor)
41-64 = Relevante pero con dudas (buen match de stack, pero falla en modalidad/geografía/seniority)
65-84 = Muy relevante (match fuerte en stack + modalidad + geografía)
85-100 = Ideal (match fuerte + fintech o dominio similar a su experiencia)

Oferta:
{texto_de_la_oferta}
```
