# =============================================================================
# Stage 1 — builder
# Компилирует бинарный файл внутри официального образа Go.
# go mod tidy генерирует go.sum автоматически, если его ещё нет.
# =============================================================================
FROM golang:1.25-alpine AS builder

# Нужен gcc для cgo-зависимостей (lib/pq не требует, но оставляем на всякий случай)
RUN apk add --no-cache git

WORKDIR /app

# Копируем весь исходный код
COPY . .

# Генерируем/обновляем go.sum и скачиваем зависимости
RUN go mod tidy

# Собираем статически связанный бинарник
# -ldflags="-s -w" убирает отладочную информацию → меньший размер образа
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api

# =============================================================================
# Stage 2 — runtime
# Минимальный образ: только бинарник + системные сертификаты + временные зоны.
# =============================================================================
FROM alpine:3.19

# libreoffice is used by internal/preview to convert uploaded .docx files to
# PDF on upload, so the iframe can render them inline without triggering a
# browser download. Cyrillic fonts (DejaVu/Liberation) are bundled via
# ttf-dejavu / ttf-liberation; without them text becomes squares.
RUN apk add --no-cache ca-certificates tzdata libreoffice ttf-dejavu ttf-liberation

WORKDIR /app

COPY --from=builder /bin/api ./api

EXPOSE 8080

ENTRYPOINT ["./api"]
