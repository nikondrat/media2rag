# Deployment — v1 Design

## Bash Setup Tool

```bash
./setup.sh
```

**Что делает:**
1. Определяет ОС (macOS / Linux)
2. Проверяет зависимости
3. Спрашивает что ставить
4. Устанавливает недостающее

### Проверка зависимостей

```bash
check_command() {
    if command -v $1 &>/dev/null; then
        echo "✓ $1 installed"
        return 0
    else
        echo "✗ $1 not found"
        return 1
    fi
}

# Проверяем всё
check_command go
check_command npx
check_command qdrant
check_command ollama
```

### Интерактивный выбор

```bash
echo "=== media2rag Setup ==="
echo ""

# Спрашиваем режим
echo "Выбери режим:"
echo "1) Local — Ollama + Qdrant + media2rag (нужен мощный CPU/GPU)"
echo "2) Server — Qdrant + media2rag (LLM через API, OpenRouter)"
read -p "Режим [1/2]: " mode

if [ "$mode" = "1" ]; then
    echo "→ Режим: local"
    install_ollama
    install_qdrant
    install_rdrr
elif [ "$mode" = "2" ]; then
    echo "→ Режим: server"
    install_qdrant
    install_rdrr
    echo "⚠ Для server режима нужен OPENROUTER_API_KEY"
    read -p "Введи API ключ: " api_key
    echo "OPENROUTER_API_KEY=$api_key" >> .env
fi

# Собираем и устанавливаем media2rag
echo "→ Собираем media2rag..."
go build -o /usr/local/bin/media2rag ./cmd/media2rag
echo "✓ media2rag установлен"
```

### Автоустановка

```bash
install_ollama() {
    if ! check_command ollama; then
        echo "→ Устанавливаем Ollama..."
        if [ "$(uname)" = "Darwin" ]; then
            brew install ollama
        else
            curl -fsSL https://ollama.com/install.sh | sh
        fi
        ollama serve &
        sleep 5
    fi
}

install_qdrant() {
    if ! check_command qdrant; then
        echo "→ Устанавливаем Qdrant..."
        if [ "$(uname)" = "Darwin" ]; then
            brew install qdrant
        else
            # Docker или бинарник
            if command -v docker &>/dev/null; then
                docker run -d -p 6333:6333 -p 6334:6334 \
                  -v /var/lib/qdrant:/qdrant/storage \
                  qdrant/qdrant
            else
                curl -L https://github.com/qdrant/qdrant/releases/latest/download/qdrant-x86_64-unknown-linux-musl.tar.gz | tar -xz
                mv qdrant /usr/local/bin/
            fi
        fi
    fi
}

install_rdrr() {
    if ! check_command npx; then
        echo "→ Устанавливаем Node.js..."
        if [ "$(uname)" = "Darwin" ]; then
            brew install node
        else
            curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
            apt-get install -y nodejs
        fi
    fi
    npm install -g rdrr
}
```

## systemd (server mode)

```ini
# /etc/systemd/system/media2rag.service
[Unit]
Description=media2rag RAG server
After=network.target

[Service]
ExecStart=/usr/local/bin/media2rag serve --config /etc/media2rag/config.yaml
Restart=always
RestartSec=5
User=media2rag
EnvironmentFile=/etc/media2rag/.env

[Install]
WantedBy=multi-user.target
```

## Обновление

```bash
./setup.sh --update
# Скачивает последний бинарник, перезапускает сервис
```

## Docker (опционально)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o media2rag ./cmd/media2rag

FROM alpine:latest
RUN apk add --no-cache nodejs npm
RUN npm install -g rdrr
COPY --from=builder /app/media2rag /usr/local/bin/
EXPOSE 8542
CMD ["media2rag", "serve"]
```

```bash
docker build -t media2rag .
docker run -d -p 8542:8542 \
  -v ~/.media2rag:/root/.media2rag \
  -e OPENROUTER_API_KEY=sk-... \
  media2rag
```

## Что нужно для каждого режима

| Зависимость | Local | Server |
|-------------|-------|--------|
| Go (build) | ✓ | ✓ |
| Ollama | ✓ | ✗ |
| Qdrant | ✓ | ✓ |
| Node.js + rdrr | ✓ | ✓ |
| OPENROUTER_API_KEY | опционально | ✓ |
| GPU/CPU мощный | ✓ | ✗ |
