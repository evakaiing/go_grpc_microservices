# gRPC Microservices

Учебный проект: модульный монолит на Go с реализацией gRPC микросервисов, ACL-авторизацией и логированием.

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
*   `main.go` — точка входа и интерсепторы.

*   `services/` — реализация сервисов и proto.
