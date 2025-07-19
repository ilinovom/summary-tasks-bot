package repository

// NewsRepository fetches news from storage.
type NewsRepository struct{}

func NewNewsRepository() *NewsRepository {
	return &NewsRepository{}
}

func (r *NewsRepository) GetLatest() []string {
	// Placeholder implementation
	return []string{"news 1", "news 2"}
}
