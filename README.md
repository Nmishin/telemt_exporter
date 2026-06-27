# telemt-exporter

Prometheus-экспортер для [telemt](https://github.com/telemt/telemt) / telemt-ui.

## Метрики

| Метрика | Тип | Описание |
|---|---|---|
| `telemt_user_traffic_up_bytes_total` | Counter | Байт отправлено пользователем |
| `telemt_user_traffic_down_bytes_total` | Counter | Байт получено пользователем |
| `telemt_user_active_connections` | Gauge | Активные соединения |
| `telemt_scrape_up` | Gauge | 1 = последний scrape успешен |

Все метрики по трафику имеют лейблы `user_id` и `username`.

## Запуск

### Собрать и запустить локально

```bash
go mod tidy
go build -o telemt-exporter .
./telemt-exporter -url http://localhost:54321 -token YOUR_TOKEN
```

### Docker Compose

```bash
TELEMT_TOKEN=your_token docker compose up -d
```

Метрики доступны на `http://localhost:9101/metrics`.

## Настройка

| Флаг | По умолчанию | Описание |
|---|---|---|
| `-url` | `http://localhost:54321` | Базовый URL telemt |
| `-token` | `` | Bearer-токен для API |
| `-listen` | `:9101` | Адрес для экспорта метрик |

## Адаптация под твой API

В `main.go` нужно подогнать две вещи:

1. **Структура `User`** — поля должны совпадать с JSON от твоего telemt
2. **Endpoint** в `fetchUsers()` — сейчас `/api/users`, поменяй если у тебя другой путь

Чтобы посмотреть что отдаёт API:
```bash
curl -H "Authorization: Bearer TOKEN" http://localhost:54321/api/users | jq .
```

## Grafana

Полезные PromQL-запросы:

```promql
# Скорость загрузки по пользователям (байт/сек за последние 5 мин)
rate(telemt_user_traffic_down_bytes_total[5m])

# Топ-5 по суммарному трафику
topk(5, telemt_user_traffic_up_bytes_total + telemt_user_traffic_down_bytes_total)

# Активные соединения сейчас
telemt_user_active_connections
```
