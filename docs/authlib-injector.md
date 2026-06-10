# Authlib: вход только из лаунчера

Цель — чтобы на сервер могли зайти **только** игроки, запустившие игру через
лаунчер Project Minecraft. Реализовано по схеме GML (authlib-injector +
собственный Yggdrasil-сервер).

## Как это работает

```
Лаунчер ──(JWT)──▶ POST /api/yggdrasil/launcher-session ──▶ accessToken + uuid
   │                                                            (хранится в бэкенде)
   ▼ запускает игру с -javaagent:<служебный jar>=<URL>/api/v1/integrations/authlib/minecraft
Игра ──(join, accessToken)──▶ POST /sessionserver/.../join ──▶ серверId ↔ профиль
   ▼ подключается к серверу
Сервер ──(hasJoined)──▶ GET /sessionserver/.../hasJoined ──▶ 200 (есть) | 204 (нет → kick)
```

Пират/сторонний клиент не может получить валидный `accessToken` (его выдаёт
только наш бэкенд после входа через лаунчер), поэтому `join` не проходит, а
`hasJoined` возвращает 204 — сервер отклоняет подключение.

**Важно:** enforcement работает только если сервер запущен в `online-mode=true`
и с тем же authlib-injector, что и клиент.

## 1. Бэкенд (готово в коде)

Yggdrasil-API смонтирован на `/api/yggdrasil`. Переменные окружения:

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `PUBLIC_BASE_URL` | `http://127.0.0.1:8080` | Публичный URL бэкенда. Должен совпадать с тем, что видят и лаунчер, и сервер. Используется для `skinDomains`/меты. |
| `YGGDRASIL_KEY_PATH` | `data/yggdrasil_key.pem` | Файл RSA-ключа (генерируется при первом старте, не коммитить). |
| `YGGDRASIL_SERVER_NAME` | `Project Minecraft` | Имя в мете. |
| `AUTHLIB_INJECTOR_PATH` | `data/authlib-injector.jar` | Jar, который backend отдаёт лаунчеру по `/api/yggdrasil/authlib-injector.jar`. |

URL, который указывается в javaagent на клиенте и сервере:
**`<PUBLIC_BASE_URL>/api/v1/integrations/authlib/minecraft`**.
`/api/yggdrasil` остаётся доступен как основной внутренний путь того же API.

Проверка, что API живой:

```bash
curl https://pjm.likonchik.xyz/api/yggdrasil/
# → {"meta":{...},"skinDomains":[...],"signaturePublickey":"-----BEGIN PUBLIC KEY-----..."}
```

## 2. Клиент (готово в коде)

Лаунчер перед запуском:
1. меняет JWT на игровую сессию (`/api/yggdrasil/launcher-session`);
2. передаёт игре `accessToken` и `uuid` из этой сессии;
3. гарантирует наличие `authlib-injector.jar` в служебной папке лаунчера
   (вне `files/`, чтобы cleanup профиля его не удалял);
4. при необходимости скачивает jar с backend:
   `/api/yggdrasil/authlib-injector.jar`;
5. добавляет `-javaagent:<путь>=<PUBLIC_BASE_URL>/api/v1/integrations/authlib/minecraft`.

**Что нужно сделать:** положить jar на backend и указать `AUTHLIB_INJECTOR_PATH`
или использовать дефолт `backend/data/authlib-injector.jar`. В файлы профиля
jar добавлять не нужно.

## 3. Игровой сервер (нужно настроить)

1. Скачать готовый jar инжектора (релиз
   [Gml.Authlib.Injector](https://github.com/Gml-Launcher/Gml.Authlib.Injector)
   или оригинальный `authlib-injector`), положить рядом с `server.jar`.

2. В `server.properties`:
   ```properties
   online-mode=true
   ```

3. Запуск сервера с агентом (GML-совместимый путь — стоковый jar, drop-in):
   ```bash
   java \
     -javaagent:authlib-injector-1.2.5.jar=https://launcher.likonchik.xyz/api/v1/integrations/authlib/minecraft \
     -Xmx4G -jar server.jar nogui
   ```

   Бэкенд отдаёт один и тот же Yggdrasil API на двух путях:
   `/api/yggdrasil` и `/api/v1/integrations/authlib/minecraft` (GML-совместимый).
   Клиент-лаунчер использует второй — указывай на сервере тот же.

   Для Forge/NeoForge добавить агент в `user_jvm_args.txt` или в строку запуска
   аналогично. Для Pterodactyl/панелей — дописать `-javaagent:...` в поле
   «дополнительные JVM-аргументы» (Startup), `online-mode=true` в properties.

4. Проверить в логах сервера при старте:
   ```
   [authlib-injector] Authentication server: Project Minecraft
   [authlib-injector] ... API root: https://pjm.likonchik.xyz/api/yggdrasil
   ```

После этого: запуск из лаунчера → вход проходит; любой сторонний клиент с тем же
ником → kick «Failed to verify username» (hasJoined вернул 204).

## Эндпоинты Yggdrasil (для справки)

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/yggdrasil/` | Мета + публичный ключ |
| POST | `/api/yggdrasil/launcher-session` | (JWT) выпуск игровой сессии лаунчеру |
| POST | `/api/yggdrasil/authserver/{authenticate,refresh,validate,invalidate,signout}` | Совместимость с authserver |
| POST | `/api/yggdrasil/sessionserver/session/minecraft/join` | Клиент фиксирует вход |
| GET | `/api/yggdrasil/sessionserver/session/minecraft/hasJoined` | Сервер проверяет вход |
| GET | `/api/yggdrasil/sessionserver/session/minecraft/profile/{uuid}` | Профиль по UUID |
| POST | `/api/yggdrasil/api/profiles/minecraft` | Резолв ников → UUID |

## Защита от подделки и обхода

- **Случайный токен на каждый запуск.** `accessToken` — 16 байт из `crypto/rand`,
  хранится только на бэкенде. Сфабриковать его нельзя: на `join` валиден лишь
  тот токен, что мы реально выдали.
- **Правка флагов запуска бесполезна.** Доступ решает сервер через `hasJoined`,
  а не клиент. Уберёшь `-javaagent` или подменишь аргументы — не будет валидной
  сессии → kick. Обойти лаунчер правкой команды нельзя.
- **Анти-replay:** `hasJoined` потребляет join-запись (повторный/перехваченный
  запрос → 204).
- **Sliding TTL:** каждый `join` продлевает токен — переподключения в течение
  игровой сессии работают.
- **Гашение при выходе:** лаунчер инвалидирует токен после закрытия игры, поэтому
  скопированную команду запуска нельзя переиспользовать позже.

Чего эта схема **не** даёт: запретить вход валидным аккаунтом через сторонний
клиент. authlib-injector передаёт серверу только ник/serverId/IP — для жёсткой
привязки «только наш лаунчер» нужен серверный плагин (отдельная задача).

## Ограничения текущей версии

- **Скины не отдаются** — игроки видят дефолтный Steve/Alex. Каркас подписи
  текстур (RSA) уже есть, добавить источник скинов можно позже.
- Сессии и join-записи хранятся **в памяти** бэкенда (TTL: сессия 15 мин, join
  60 сек). При рестарте бэкенда активные сессии сбрасываются — игроку нужно
  перезапустить игру из лаунчера. Для нескольких инстансов бэкенда понадобится
  общий стор (Redis/БД).
