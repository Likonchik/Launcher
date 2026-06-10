package bot

import "launcher-backend/internal/telegram"

// BotMenuCommands — язык по умолчанию (Telegram UI на ru и прочие без отдельного набора).
func BotMenuCommands() []telegram.BotCommand {
	return []telegram.BotCommand{
		{Command: "start", Description: "🏠 Главная панель и клавиатура"},
		{Command: "menu", Description: "⌨ Показать клавиатуру"},
		{Command: "help", Description: "❓ Список команд и пояснения"},
		{Command: "cancel", Description: "⛔ Сбросить текущий шаг (логин, пароль, код)"},
		{Command: "profile", Description: "Мой профиль и привязка"},
		{Command: "bind", Description: "🔑 Привязать аккаунт (ник / логин и пароль)"},
		{Command: "register", Description: "📋 Регистрация нового аккаунта"},
		{Command: "password", Description: "Сменить пароль"},
		{Command: "email", Description: "Сменить e-mail"},
		{Command: "2fa", Description: "2FA для лаунчера (TOTP)"},
		{Command: "donate", Description: "Магазин и донат"},
		{Command: "launcher", Description: "Файл лаунчера (.exe) в чат"},
		{Command: "admin", Description: "🛠 Панель модератора"},
	}
}

// BotMenuCommandsEN — для клиентов Telegram с языком интерфейса English.
func BotMenuCommandsEN() []telegram.BotCommand {
	return []telegram.BotCommand{
		{Command: "start", Description: "Main menu and keyboard"},
		{Command: "menu", Description: "Show reply keyboard"},
		{Command: "help", Description: "Command list"},
		{Command: "cancel", Description: "Cancel current step"},
		{Command: "profile", Description: "My profile"},
		{Command: "bind", Description: "Link account (login + password)"},
		{Command: "register", Description: "Register"},
		{Command: "password", Description: "Change password"},
		{Command: "email", Description: "Change email"},
		{Command: "2fa", Description: "Launcher 2FA (TOTP)"},
		{Command: "donate", Description: "Shop / donate link"},
		{Command: "launcher", Description: "Launcher .exe as file"},
		{Command: "admin", Description: "Moderator panel"},
	}
}
