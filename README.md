# Augustberries

E-commerce проект, построенный по принципам микросервисной архитектуры.

## Технологический стек

- **Backend**: Go (Gin, GORM)
- **Databases**: PostgreSQL, MongoDB
- **Cache**: Redis
- **Message Broker**: Apache Kafka
- **Monitoring**: Prometheus, Grafana
- **Logging**: ELK Stack (Elasticsearch, Logstash, Kibana)

## Микросервисы

| Сервис | Порт | Описание |
|--------|------|----------|
| auth-service | 8080 | Аутентификация и авторизация |
| catalog-service | 8081 | Управление товарами и категориями |
| orders-service | 8082 | Обработка заказов |
| reviews-service | 8083 | Управление отзывами |
| background-worker | 8085 | Фоновая обработка заказов и курсов валют |

## Быстрый старт

```bash
# Скопируйте файл конфигурации
cp .env.example .env

# Запустите все сервисы
docker-compose up -d
```

## Конфигурация

Все переменные окружения вынесены в `.env` файл. Пример конфигурации находится в `.env.example`.

## API Endpoints

### Auth Service (порт 8080)

**Публичные эндпоинты:**
- `POST /auth/register` - Регистрация
- `POST /auth/login` - Вход
- `POST /auth/refresh` - Обновление токенов
- `POST /auth/validate` - Валидация токена

**Защищенные эндпоинты:**
- `GET /auth/me` - Информация о текущем пользователе
- `POST /auth/logout` - Выход

**Административные эндпоинты (только admin):**
- `GET /admin/roles` - Список ролей
- `GET /admin/roles/:id` - Получить роль
- `POST /admin/roles` - Создать роль
- `PUT /admin/roles/:id` - Обновить роль
- `DELETE /admin/roles/:id` - Удалить роль
- `GET /admin/roles/:id/permissions` - Разрешения роли
- `POST /admin/roles/:id/permissions` - Назначить разрешения
- `DELETE /admin/roles/:id/permissions` - Удалить разрешения
- `GET /admin/permissions` - Список разрешений
- `POST /admin/permissions` - Создать разрешение
- `DELETE /admin/permissions/:id` - Удалить разрешение
