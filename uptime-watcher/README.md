[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-22041afd0340ce965d47ae6ef1cefeee28c7c493a6346c4f15d667ab976d596c.svg)](https://classroom.github.com/a/OC59jqlQ)

# Monitor de Servicios (ejemplo)

Este ejemplo implementa un monitor estilo "uptime" escrito 100% en Go. Integra múltiples paradigmas:

- **Imperativo**: el scheduler (`internal/scheduler`) orquesta tiempos, reintentos y manejo de señales.
- **Concurrente**: cada chequeo se ejecuta en una goroutine, coordinada mediante canales.
- **Funcional**: el pipeline de agregación (`internal/store`) usa funciones puras para calcular uptime y transformar resultados.

## Estructura

```
ejemplo/
├── cmd/monitor/main.go          # punto de entrada, banderas y wiring
├── config/targets.json          # configuración (sin datos hardcodeados)
├── internal/api                 # API REST
├── internal/check               # chequeos HTTP/TCP
├── internal/config              # carga de configuración
├── internal/scheduler           # scheduler concurrente
├── internal/store               # estado en memoria + estadísticas
└── internal/ui                  # frontend HTML simple con html/template
```

## Ejecución

Desde la carpeta `ejemplo/`:

```bash
cd ejemplo
GOCACHE=$(pwd)/.gocache go run ./cmd/monitor
```

Banderas útiles:

- `-config` Ruta a un archivo JSON con targets (por defecto `config/targets.json`).
- `-addr` Dirección para exponer la API/frontend (por defecto `:8080`).

La aplicación expone:

- Frontend HTML en `GET /`
- `GET /api/status` snapshot de estados
- `GET /api/targets` lista de servicios
- `GET /api/history?id=<id>&limit=<n>` histórico reciente
- `POST /api/refresh?id=<id>` fuerza un chequeo inmediato
- `GET /healthz` health-check de la app

## Configuración de targets

`config/targets.json` define los servicios a monitorear (HTTP o TCP). Ejemplo:

```json
{
  "targets": [
    {
      "id": "example-http",
      "name": "Servicio HTTP de ejemplo",
      "kind": "http",
      "url": "https://example.org/",
      "frequency": "30s",
      "timeout": "5s"
    }
  ]
}
```

Puedes añadir más entradas sin recompilar; basta reiniciar el monitor.

## Validación

Se verificó la compilación con:

```bash
GOCACHE=$(pwd)/.gocache go test ./...
```

(No hay pruebas unitarias aún; el comando asegura que todos los paquetes compilan.)
