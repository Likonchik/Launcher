package bot

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Лимит загрузки файла ботом в Telegram (~50 MiB); исходный PJMLauncher.exe часто чуть больше.
const (
	telegramBotMaxUploadBytes      int64 = 50 * 1024 * 1024
	telegramPreferGzipIfLargerThan int64 = 48 * 1024 * 1024
)

// prepareLauncherForTelegram готовит путь к документу для sendDocument, имя вложения и cleanup.
// Если .exe слишком большой для лимита Telegram, во временный файл пишется gzip (имя …exe.gz).
func prepareLauncherForTelegram(absExe string) (sendPath, attachName string, cleanup func(), usedGzip bool, err error) {
	absExe = filepath.Clean(absExe)
	st, err := os.Stat(absExe)
	if err != nil {
		return "", "", nil, false, err
	}
	if !st.Mode().IsRegular() {
		return "", "", nil, false, fmt.Errorf("не обычный файл: %s", absExe)
	}
	base := filepath.Base(absExe)
	noop := func() {}
	if st.Size() <= telegramPreferGzipIfLargerThan {
		return absExe, base, noop, false, nil
	}

	wf, err := os.CreateTemp("", "launcher-tg-*.gz")
	if err != nil {
		return "", "", nil, false, err
	}
	tmpPath := wf.Name()
	cleanupFn := func() { _ = os.Remove(tmpPath) }

	src, err := os.Open(absExe)
	if err != nil {
		_ = wf.Close()
		cleanupFn()
		return "", "", nil, false, err
	}
	gw, err := gzip.NewWriterLevel(wf, gzip.BestCompression)
	if err != nil {
		_ = src.Close()
		_ = wf.Close()
		cleanupFn()
		return "", "", nil, false, err
	}
	if _, err := io.Copy(gw, src); err != nil {
		_ = gw.Close()
		_ = src.Close()
		_ = wf.Close()
		cleanupFn()
		return "", "", nil, false, err
	}
	if err := gw.Close(); err != nil {
		_ = src.Close()
		_ = wf.Close()
		cleanupFn()
		return "", "", nil, false, err
	}
	if err := src.Close(); err != nil {
		_ = wf.Close()
		cleanupFn()
		return "", "", nil, false, err
	}
	if err := wf.Close(); err != nil {
		cleanupFn()
		return "", "", nil, false, err
	}

	gzSt, err := os.Stat(tmpPath)
	if err != nil {
		cleanupFn()
		return "", "", nil, false, err
	}
	if gzSt.Size() > telegramBotMaxUploadBytes {
		cleanupFn()
		return "", "", nil, false, fmt.Errorf("сжатый лаунчер всё ещё больше лимита Telegram (~50 МБ)")
	}

	attach := base + ".gz"
	return tmpPath, attach, cleanupFn, true, nil
}
