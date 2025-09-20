# Многоэтапная сборка для оптимизации размера образа
FROM golang:1.23-alpine AS builder

# Устанавливаем необходимые пакеты
RUN apk add --no-cache git ca-certificates tzdata

# Создаем пользователя для безопасности
RUN adduser -D -g '' appuser

# Устанавливаем рабочую директорию
WORKDIR /build

# Копируем go.mod для кэширования зависимостей
COPY go.mod ./

# Загружаем зависимости (go.sum создастся автоматически если нужно)
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o server ./cmd/server

# Финальный образ
FROM scratch

# Копируем сертификаты и временную зону
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Копируем пользователя
COPY --from=builder /etc/passwd /etc/passwd

# Копируем собранное приложение
COPY --from=builder /build/server /server

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8080

# Устанавливаем переменные окружения по умолчанию
ENV HTTP_ADDR=:8080
ENV KRAKEN_URL=https://api.kraken.com
ENV CACHE_TTL=60s
ENV HTTP_TIMEOUT=3s

# Запускаем приложение
ENTRYPOINT ["/server"]
