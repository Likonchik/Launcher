package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"launcher-backend/internal/botconfig"
)

// opsFormatDigest дублирует поведение Rust ops.rs
func opsFormatDigest(client *http.Client, cfg *botconfig.Config) string {
	var parts []string
	if cfg.VPSOpsHealthURL != "" {
		if client == nil {
			client = http.DefaultClient
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.VPSOpsHealthURL, nil)
		if err != nil {
			parts = append(parts, "OPS health URL некорректен")
		} else {
			resp, err := client.Do(req)
			if err != nil {
				parts = append(parts, fmt.Sprintf("OPS health недоступен: %v", err))
			} else {
				defer resp.Body.Close()
				b, _ := io.ReadAll(resp.Body)
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					var v any
					if json.Unmarshal(b, &v) == nil {
						parts = append(parts, fmt.Sprintf("OPS health JSON: %s", string(b)))
					} else {
						parts = append(parts, fmt.Sprintf("OPS health body: %s", string(bytes.TrimSpace(b))))
					}
				} else {
					parts = append(parts, fmt.Sprintf("OPS health: HTTP %d", resp.StatusCode))
				}
			}
		}
	}
	if cfg.VPSOpsAlertStatePath != "" {
		b, err := os.ReadFile(cfg.VPSOpsAlertStatePath)
		if err != nil {
			parts = append(parts, fmt.Sprintf("Не удалось прочитать alert-state (%s): %v", cfg.VPSOpsAlertStatePath, err))
		} else {
			s := string(b)
			if len(s) > 3500 {
				s = s[:3500]
			}
			parts = append(parts, fmt.Sprintf("alert-state.json (до 3500 симв.):\n%s", s))
		}
	}
	if len(parts) == 0 {
		return "Интеграция OPS не задана (VPS_OPS_* пустые)."
	}
	return strings.Join(parts, "\n---\n")
}
