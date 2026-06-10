# anticheat-native

Нативный JVMTI-агент античита. Загружается в JVM Minecraft через `-agentpath` и:

- доказывает своё присутствие Java-агенту через flag-файл (`-agentpath:lib=<flagfile>`);
- ставит `ClassFileLoadHook` и инспектирует имена загружаемых классов (включая
  bootstrap-классы, недоступные Java-инструментации) на маркеры читов;
- определяет отладчик (`TracerPid` на Linux / `IsDebuggerPresent` на Windows).

Anti late-attach обеспечивается JVM-флагом `-XX:+DisableAttachMechanism`, который
добавляет лаунчер рядом с `-agentpath`.

## Сборка

### Linux (.so)

```bash
JAVA_HOME=/path/to/jdk ./build.sh
# → backend/data/libanticheat.so
```

### Windows (.dll) и кроссплатформенно — через CMake

Собирать нужно на целевой ОС (или кросс-тулчейном). Требуется установленный JDK
(переменная `JAVA_HOME` с заголовками `include/jvmti.h`).

```bash
cmake -B build
cmake --build build --config Release
# Linux:   build/libanticheat.so
# Windows: build/Release/anticheat.dll  (MSVC) или build/anticheat.dll (MinGW)
```

Готовые библиотеки кладутся в `backend/data/` и раздаются бэкендом по
`GET /api/anticheat/native/{linux|windows}` (пути задаются через
`ANTICHEAT_NATIVE_LINUX` / `ANTICHEAT_NATIVE_WIN`).

## Ограничения (честно)

Агент работает в user-space: нет ring0/драйверов. flag-файл и инжект агентов
спуфятся пропатченным лаунчером. Это поднимает планку, но не делает обход
невозможным без доверенного железа. Подпись бинарников и обфускация — M6.
