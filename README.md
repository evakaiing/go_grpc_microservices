# gRPC Microservices

gRPC-сервер с микросервисной архитектурой интерфейсов (но монолитной реализацией).

## Описание
В одном процессе запускаются два сервиса:
1.  **Biz**: Бизнес-логика (`Check`, `Add`, `Test`).
2.  **Admin**: Мониторинг и логи (`Logging`, `Statistics`).

**Особенности:**
*   **gRPC & Protobuf**: Контракты API.
*   **Interceptors**: Валидация Unary и Stream запросов.
*   **ACL**: Проверка прав по метаданным `consumer`.
*   **Logging**: Стрим событий в реальном времени.
*   **Statistics**: Агрегация метрик вызовов.

## Запуск

1.  **Клон:**
    ```
    git clone https://github.com/your-user/go-grpc-hw.git
    cd go-grpc-hw/microservice
    ```

2.  **Тесты:**
    ```
    go test -v -race ./...
    ```

## Makefile

### Запуск
`make run`

### Тестирование
`make test`


## Конфиг ACL
Пример прав доступа (JSON):

```
{
"biz_user": ["/pbBiz.Biz/Check"],
"biz_admin": ["/pbBiz.Biz/*"],
"logger": ["/pbAdmin.Admin/Logging"]
}
```

## Структура
```microservice/
├── api/ # Proto-файлы
│ ├── admin.proto
│ └── biz.proto
├── cmd/
│ └── server/
│ ├── main.go # Точка входа
│ └── service_test.go # Интеграционные тесты
├── internal/
│ ├── app/
│ │ └── server.go # Сборка gRPC-сервера
│ └── service/
│ ├── admin.go # Реализация Admin-сервиса
│ └── biz.go # Реализация Biz-сервиса
├── pkg/
│ └── api/ # Сгенерированный код
│ ├── admin/
│ └── biz/
├── Makefile 
├── go.mod
└── README.md
```
