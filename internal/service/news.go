package service

// NewsService returns news items.
type NewsService struct{}

func NewNewsService() *NewsService {
	return &NewsService{}
}

func (s *NewsService) Latest() []string {
	// Placeholder: fetch latest news from repository
	return []string{"news 1", "news 2"}
}
