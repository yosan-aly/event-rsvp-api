# event-rsvp-api

Backend de [event-rsvp](https://github.com/yosan-aly/event-rsvp): confirmación de
asistencia para eventos (bodas, cumpleaños, celebraciones). Los invitados confirman su
asistencia (nombre y cuántas personas van en total) y los organizadores ven el listado
con los totales desde la misma página, con un código de acceso.

## Stack

- **Go 1.25** · [Fiber v2](https://gofiber.io) · [pgx v5](https://github.com/jackc/pgx)
- **PostgreSQL 17** ([Neon](https://neon.tech) en producción)
- **Google Cloud Run** (escala a 0) · Cloud Build · Secret Manager
- Logs JSON con `log/slog`, compatibles con Cloud Logging
- Documentación OpenAPI con [swag](https://github.com/swaggo/swag)

## Arquitectura

Hexagonal en un solo servicio: el dominio no conoce ni HTTP ni la base de datos.

```
api/rest/                             # Especificación OpenAPI generada (swag)
cmd/main.go                           # Arranque: config → BD → migraciones → HTTP
scripts/                              # open-api.sh, run.sh, test.sh
internal/
├── attendance/
│   ├── domain/                       # Confirmation, validación, errores, puerto Repository
│   ├── application/service.go        # Casos de uso: confirmar, actualizar, resumen, eliminar
│   └── infrastructure/
│       ├── postgres/                 # Adaptador de salida (pgx)
│       └── rest/                     # Adaptador de entrada (Fiber): handlers y DTOs
└── platform/
    ├── config/                       # Variables de entorno
    ├── database/                     # Pool, migraciones embebidas y dbtest
    ├── httperr/                      # Formato común de errores
    ├── logger/                       # slog → formato Cloud Logging
    └── server/                       # Middlewares, rutas, autenticación de admin
```

## API

Base: `/api/v1`. Los errores siempre tienen la forma
`{"error": {"code": "...", "message": "..."}}`, y `message` se puede mostrar al invitado.

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| GET | `/health` | público | Estado del servicio |
| POST | `/api/v1/attendance-confirmations` | público | Confirma asistencia. Body: `{"name": "Ana", "attendees": 3}`; devuelve el `id` |
| PUT | `/api/v1/attendance-confirmations/{id}` | público, con el `id` | Actualiza nombre o asistentes |
| GET | `/api/v1/admin/attendance-confirmations` | admin | Confirmaciones con `totalConfirmations` y `totalAttendees` |
| DELETE | `/api/v1/admin/attendance-confirmations/{id}` | admin | Elimina una confirmación (por ejemplo, un duplicado) |

- `attendees` incluye a quien confirma y va de 1 a 7. Los nombres pueden repetirse.
- El `id` que devuelve la confirmación es un UUID que solo conoce el navegador del invitado:
  es lo que le permite actualizar su confirmación sin necesidad de login.
- Las rutas de admin requieren el header `X-Admin-Token`; en la página es el "código de
  acceso" de la pestaña de invitados. Si `ADMIN_TOKEN` no está definido, responden 404.

### Swagger y Postman

```bash
sh scripts/open-api.sh        # regenera api/rest tras cambiar endpoints o DTOs
```

- **Swagger UI:** con `SERVICE_WITH_OPEN_API=true` (activo en `.dev.env`):
  <http://localhost:7080/swagger/index.html>. En producción está desactivado.
- **Postman:** *Import* → `api/rest/swagger.json`. Hay ejemplos en [`requests.http`](requests.http).

## Desarrollo local

Requisitos: Go 1.25 y PostgreSQL (por defecto en `localhost:5433`).

```bash
# 1. Crear la base (una vez); el schema lo crea el servicio
psql -h localhost -p 5433 -U postgres -c "CREATE DATABASE event_rsvp"

# 2. Ejecutar (lee .dev.env; para sobrescribir valores crea un .env)
go run ./cmd                  # http://localhost:7080

# 3. Tests
go test ./...                 # unitarios
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5433/event_rsvp?sslmode=disable" \
  go test -count=1 ./...      # integración, en el schema event_rsvp_test
```

Las migraciones se aplican solas al arrancar, en orden y una sola vez, dentro del schema
de `DB_SCHEMAS` (que se crea si no existe). Un advisory lock por schema evita que dos
instancias migren a la vez.

## Configuración

| Variable | Requerida | Descripción |
|---|---|---|
| `DB_ADDR` | sí | `host:puerto` de PostgreSQL |
| `DB_DATABASE` | sí | Nombre de la base |
| `DB_SCHEMAS` | no | Schema del servicio; se crea si no existe (por defecto `public`) |
| `DB_USER` / `DB_PASSWORD` | sí | Credenciales |
| `DB_SSL_MODE` | no | `verify-full` en producción (Neon) |
| `ALLOWED_ORIGINS` | sí | Orígenes CORS separados por comas |
| `ADMIN_TOKEN` | no | Código de acceso al listado de invitados |
| `WRITE_RATE_LIMIT` | no | Confirmaciones por minuto por IP (por defecto 10) |
| `SERVICE_PORT` / `PORT` | no | Puerto HTTP (por defecto 8080) |
| `SERVICE_WITH_OPEN_API` | no | `true` publica Swagger UI en `/swagger` |
| `LOG_LEVEL` | no | `debug`, `info`, `warn` o `error` |

## Despliegue

```bash
gcloud builds submit --config=cloudbuild.yaml --substitutions="_TAG=$(git rev-parse --short HEAD)"
```

Construye la imagen, la publica en Artifact Registry y la despliega en Cloud Run.
`DB_PASSWORD` y `ADMIN_TOKEN` se leen de Secret Manager.
