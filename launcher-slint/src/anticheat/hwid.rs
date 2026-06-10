//! Сбор аппаратного отпечатка (HWID). Кроссплатформенно, best-effort: чем больше
//! стабильных компонентов удаётся собрать, тем устойчивее отпечаток. Компоненты
//! не отправляются в сыром виде — наружу уходит только солёный SHA-256.

use sha2::{Digest, Sha256};

// Соль фиксирует отпечаток за этим лаунчером: один и тот же набор железа даст
// разный хэш в другом проекте, и сырые серийники не восстановить из хэша.
const HWID_SALT: &str = "projectminecraft-anticheat-v1";

/// Возвращает стабильный солёный SHA-256-хэш железа в hex.
pub fn collect_hwid_hash() -> String {
    let mut parts = collect_components();
    parts.sort();
    parts.dedup();

    let mut hasher = Sha256::new();
    hasher.update(HWID_SALT.as_bytes());
    for part in parts {
        hasher.update(b"|");
        hasher.update(part.as_bytes());
    }
    to_hex(hasher.finalize().as_slice())
}

fn to_hex(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        out.push_str(&format!("{:02x}", b));
    }
    out
}

#[cfg(target_os = "linux")]
fn collect_components() -> Vec<String> {
    let mut parts = Vec::new();
    for path in ["/etc/machine-id", "/sys/class/dmi/id/product_uuid"] {
        if let Ok(value) = std::fs::read_to_string(path) {
            let v = value.trim();
            if !v.is_empty() {
                parts.push(v.to_string());
            }
        }
    }
    parts.extend(mac_addresses_linux());
    parts
}

#[cfg(target_os = "linux")]
fn mac_addresses_linux() -> Vec<String> {
    let mut out = Vec::new();
    let Ok(entries) = std::fs::read_dir("/sys/class/net") else {
        return out;
    };
    for entry in entries.flatten() {
        let name = entry.file_name();
        // Виртуальные интерфейсы нестабильны — пропускаем lo и docker/veth/br.
        let name = name.to_string_lossy();
        if name == "lo" || name.starts_with("docker") || name.starts_with("veth") || name.starts_with("br-") {
            continue;
        }
        let addr_path = entry.path().join("address");
        if let Ok(addr) = std::fs::read_to_string(&addr_path) {
            let addr = addr.trim();
            if !addr.is_empty() && addr != "00:00:00:00:00:00" {
                out.push(addr.to_string());
            }
        }
    }
    out
}

#[cfg(target_os = "windows")]
fn collect_components() -> Vec<String> {
    use std::process::Command;
    let mut parts = Vec::new();

    // MachineGuid из реестра — стабильный идентификатор установки Windows.
    if let Ok(output) = Command::new("reg")
        .args([
            "query",
            r"HKLM\SOFTWARE\Microsoft\Cryptography",
            "/v",
            "MachineGuid",
        ])
        .output()
    {
        if let Some(value) = parse_reg_value(&String::from_utf8_lossy(&output.stdout)) {
            parts.push(value);
        }
    }

    // UUID материнской платы через WMIC (есть на большинстве систем).
    if let Ok(output) = Command::new("wmic")
        .args(["csproduct", "get", "UUID"])
        .output()
    {
        let text = String::from_utf8_lossy(&output.stdout);
        for line in text.lines().map(str::trim) {
            if !line.is_empty() && !line.eq_ignore_ascii_case("UUID") {
                parts.push(line.to_string());
            }
        }
    }

    parts
}

#[cfg(target_os = "windows")]
fn parse_reg_value(stdout: &str) -> Option<String> {
    for line in stdout.lines() {
        if let Some(idx) = line.find("REG_SZ") {
            let value = line[idx + "REG_SZ".len()..].trim();
            if !value.is_empty() {
                return Some(value.to_string());
            }
        }
    }
    None
}

#[cfg(not(any(target_os = "linux", target_os = "windows")))]
fn collect_components() -> Vec<String> {
    // macOS и прочее: M1 ограничивается доступным, отпечаток слабее.
    Vec::new()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hwid_hash_is_64_hex_and_stable() {
        let a = collect_hwid_hash();
        let b = collect_hwid_hash();
        assert_eq!(a, b, "HWID-хэш должен быть стабильным между вызовами");
        assert_eq!(a.len(), 64, "SHA-256 в hex = 64 символа");
        assert!(a.chars().all(|c| c.is_ascii_hexdigit()));
    }

    #[test]
    fn to_hex_encodes_bytes() {
        assert_eq!(to_hex(&[0x00, 0x0f, 0xff]), "000fff");
    }
}
