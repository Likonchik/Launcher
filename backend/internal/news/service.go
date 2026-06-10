package news

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultLimit = 20
	cacheTTL     = 5 * time.Minute
	fetchTimeout = 10 * time.Second
)

// Item — единица ленты новостей, отдаваемая лаунчеру.
type Item struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	ImageURL  string `json:"imageUrl"`
	CreatedAt string `json:"createdAt"`
}

// Service тянет новости из публичного превью Telegram-канала (t.me/s/<channel>)
// и кэширует результат, чтобы не дёргать Telegram на каждый запрос лаунчера.
type Service struct {
	channel string
	client  *http.Client

	mu        sync.Mutex
	cache     []Item
	fetchedAt time.Time
}

func NewService(channel string) *Service {
	return &Service{
		channel: strings.TrimSpace(channel),
		client:  &http.Client{Timeout: fetchTimeout},
	}
}

// List возвращает последние посты канала (новые сверху), используя кэш.
func (s *Service) List(ctx context.Context, limit int) ([]Item, error) {
	if limit <= 0 || limit > 100 {
		limit = defaultLimit
	}

	items, err := s.cached(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *Service) cached(ctx context.Context) ([]Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil && time.Since(s.fetchedAt) < cacheTTL {
		return s.cache, nil
	}

	items, err := s.fetch(ctx)
	if err != nil {
		// При сбое отдаём устаревший кэш, если он есть — лента не пропадает.
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, err
	}

	s.cache = items
	s.fetchedAt = time.Now()
	return items, nil
}

func (s *Service) fetch(ctx context.Context) ([]Item, error) {
	if s.channel == "" {
		return nil, errors.New("Telegram-канал не настроен")
	}

	url := fmt.Sprintf("https://t.me/s/%s", s.channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Telegram отдаёт превью только «обычным» браузерам.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LauncherNews/1.0)")
	req.Header.Set("Accept-Language", "ru,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Telegram вернул HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	items := parseMessages(doc)
	// На странице посты идут старые→новые, лента нужна новыми сверху.
	reverse(items)
	return items, nil
}

// parseMessages обходит DOM и собирает посты из блоков tgme_widget_message.
func parseMessages(root *html.Node) []Item {
	var items []Item
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "tgme_widget_message") && attr(n, "data-post") != "" {
			if item, ok := parseMessage(n); ok {
				items = append(items, item)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return items
}

func parseMessage(node *html.Node) (Item, bool) {
	textNode := findFirst(node, func(n *html.Node) bool {
		return hasClass(n, "tgme_widget_message_text")
	})
	if textNode == nil {
		return Item{}, false
	}

	body := strings.TrimSpace(collectText(textNode))
	if body == "" {
		return Item{}, false
	}

	createdAt := ""
	if timeNode := findFirst(node, func(n *html.Node) bool {
		return n.Data == "time" && attr(n, "datetime") != ""
	}); timeNode != nil {
		createdAt = attr(timeNode, "datetime")
	}

	imageURL := ""
	if photo := findFirst(node, func(n *html.Node) bool {
		return hasClass(n, "tgme_widget_message_photo_wrap")
	}); photo != nil {
		imageURL = backgroundImageURL(attr(photo, "style"))
	}

	title, rest := splitTitle(body)
	return Item{
		Title:     title,
		Body:      rest,
		ImageURL:  imageURL,
		CreatedAt: createdAt,
	}, true
}

// splitTitle делит текст поста на заголовок (первая строка) и остальное тело.
func splitTitle(text string) (string, string) {
	parts := strings.SplitN(text, "\n", 2)
	title := strings.TrimSpace(parts[0])
	if len([]rune(title)) > 90 {
		runes := []rune(title)
		title = strings.TrimSpace(string(runes[:90])) + "…"
	}
	rest := ""
	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}
	return title, rest
}

// --- помощники по DOM ---

func hasClass(n *html.Node, class string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, field := range strings.Fields(attr(n, "class")) {
		if field == class {
			return true
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findFirst(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirst(c, pred); found != nil {
			return found
		}
	}
	return nil
}

// collectText собирает текст узла, превращая <br> в переносы строк.
func collectText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		switch {
		case node.Type == html.TextNode:
			sb.WriteString(node.Data)
		case node.Type == html.ElementNode && node.Data == "br":
			sb.WriteString("\n")
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func backgroundImageURL(style string) string {
	idx := strings.Index(style, "url('")
	if idx < 0 {
		return ""
	}
	rest := style[idx+len("url('"):]
	end := strings.Index(rest, "')")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func reverse(items []Item) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}
