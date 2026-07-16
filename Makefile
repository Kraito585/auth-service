.PHONY: all setup check build clean

APP_NAME = auth_service
BIN_DIR = bin

# === МАГИЯ ОПРЕДЕЛЕНИЯ ОС ===
ifeq ($(OS),Windows_NT)
	# Настройки для Windows
	EXT = .exe
	RM = if exist $(BIN_DIR) rmdir /s /q $(BIN_DIR)
	MKDIR = if not exist $(BIN_DIR) mkdir $(BIN_DIR)
	FIX_PATH = $(subst /,\,$1)
else
	# Настройки для Linux / macOS
	EXT =
	RM = rm -rf $(BIN_DIR)
	MKDIR = mkdir -p $(BIN_DIR)
	FIX_PATH = $1
endif
# ==============================

all: setup check build

setup:
	@echo "📥 Установка зависимостей..."
	go mod tidy
	go mod download

check:
	@echo "🔍 Проверка кода..."
	go vet ./...

build:
	@echo "🔨 Сборка проекта..."
	$(MKDIR)
	go build -o $(call FIX_PATH,$(BIN_DIR)/$(APP_NAME)$(EXT)) ./cmd/api/main.go
	@echo "✅ Бинарник готов: $(BIN_DIR)/$(APP_NAME)$(EXT)"

clean:
	@echo "🧹 Очистка..."
	$(RM)